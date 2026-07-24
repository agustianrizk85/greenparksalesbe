package http

import (
	"net/http"
	"strings"
	"time"

	"greenpark/sales/internal/domain"
	"greenpark/sales/internal/service"
)

/* ---------------------------- SKP (Surat Konfirmasi Pesanan) ---------------------------- */

// skpProjectTemplates lists the Kadep-managed per-project master data (proyek
// + rekening booking fee). Any authenticated sales user may read it (needed to
// prefill a new SKP); only the Kadep (admin) may write.
func (h *Handler) skpProjectTemplates(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.svc.SkpProjectTemplates())
}

func (h *Handler) saveSkpProjectTemplate(w http.ResponseWriter, r *http.Request) {
	t, ok := decode[domain.SkpProjectTemplate](w, r)
	if !ok {
		return
	}
	t.ProjectName = strings.TrimSpace(t.ProjectName)
	if t.ProjectName == "" {
		writeError(w, http.StatusBadRequest, "nama proyek wajib diisi")
		return
	}
	saved, err := h.svc.SaveSkpProjectTemplate(t)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (h *Handler) deleteSkpProjectTemplate(w http.ResponseWriter, r *http.Request) {
	ok, err := h.svc.DeleteSkpProjectTemplate(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "data tidak ditemukan")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// skpList returns issued SKPs: the Kadep (admin) sees every one, a staff
// member sees only the ones they created — matching the screening submissions
// visibility rule.
func (h *Handler) skpList(w http.ResponseWriter, r *http.Request) {
	u, _ := r.Context().Value(userCtxKey).(domain.User)
	all := h.svc.SkpList()
	if u.Role == domain.RoleAdmin {
		writeJSON(w, http.StatusOK, all)
		return
	}
	mine := make([]domain.Skp, 0, len(all))
	for _, s := range all {
		if strings.EqualFold(s.By, u.Username) {
			mine = append(mine, s)
		}
	}
	writeJSON(w, http.StatusOK, mine)
}

// saveSkp creates or updates an issued SKP. Ownership (By/ByName) is only set
// on create so a later edit by the Kadep never reassigns the original author.
func (h *Handler) saveSkp(w http.ResponseWriter, r *http.Request) {
	s, ok := decode[domain.Skp](w, r)
	if !ok {
		return
	}
	s.Nama = strings.TrimSpace(s.Nama)
	if s.Nama == "" {
		writeError(w, http.StatusBadRequest, "nama pemesan wajib diisi")
		return
	}
	if strings.TrimSpace(s.NamaProyek) == "" {
		writeError(w, http.StatusBadRequest, "nama proyek wajib diisi")
		return
	}
	u, _ := r.Context().Value(userCtxKey).(domain.User)
	isNew := s.EntID == ""
	if isNew {
		s.By = u.Username
		s.ByName = u.Name
		s.CreatedAt = time.Now().Format(time.RFC3339)
	} else {
		// Only the author or the Kadep may edit an existing SKP.
		existing, found := findSkp(h.svc.SkpList(), s.EntID)
		if !found {
			writeError(w, http.StatusNotFound, "SKP tidak ditemukan")
			return
		}
		if u.Role != domain.RoleAdmin && !strings.EqualFold(existing.By, u.Username) {
			writeError(w, http.StatusForbidden, "hanya pembuat atau Kadep yang bisa mengubah SKP ini")
			return
		}
		s.By, s.ByName, s.CreatedAt = existing.By, existing.ByName, existing.CreatedAt
	}
	saved, err := h.svc.SaveSkp(s)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// A brand-new SKP books its unit automatically — no separate step for the
	// sales staff, and it stops the same unit being booked twice.
	if isNew && strings.TrimSpace(saved.BlokNoUnit) != "" {
		markUnitBooked(h.svc, saved)
	}
	writeJSON(w, http.StatusOK, saved)
}

func findSkp(xs []domain.Skp, id string) (domain.Skp, bool) {
	for _, x := range xs {
		if x.EntID == id {
			return x, true
		}
	}
	return domain.Skp{}, false
}

// deleteSkp removes an issued SKP (author or Kadep only).
func (h *Handler) deleteSkp(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	u, _ := r.Context().Value(userCtxKey).(domain.User)
	if u.Role != domain.RoleAdmin {
		existing, found := findSkp(h.svc.SkpList(), id)
		if !found {
			writeError(w, http.StatusNotFound, "SKP tidak ditemukan")
			return
		}
		if !strings.EqualFold(existing.By, u.Username) {
			writeError(w, http.StatusForbidden, "hanya pembuat atau Kadep yang bisa menghapus SKP ini")
			return
		}
	}
	ok, err := h.svc.DeleteSkp(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "data tidak ditemukan")
		return
	}
	// Free the unit back up — but only if it's still "booked" by THIS SKP (not
	// if the Kadep already manually moved it to "terjual").
	for _, ub := range h.svc.UnitBookings() {
		if ub.SkpID == id && ub.Status == domain.UnitBooked {
			ub.Status, ub.SkpID, ub.UpdatedAt = domain.UnitTersedia, "", time.Now().Format(time.RFC3339)
			_, _ = h.svc.SaveUnitBooking(ub)
			break
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

/* ---------------------------- Master Booking (unit status) ---------------------------- */

// markUnitBooked finds-or-creates the UnitBooking for a newly-created SKP's
// project+unit and flips it to "booked", linked to the SKP.
func markUnitBooked(svc service.SalesService, s domain.Skp) {
	proyek, blok := strings.TrimSpace(s.NamaProyek), strings.TrimSpace(s.BlokNoUnit)
	now := time.Now().Format(time.RFC3339)
	for _, ub := range svc.UnitBookings() {
		if strings.EqualFold(ub.NamaProyek, proyek) && strings.EqualFold(ub.BlokNoUnit, blok) {
			ub.Status, ub.SkpID, ub.UpdatedAt = domain.UnitBooked, s.EntID, now
			if ub.TypeUnit == "" {
				ub.TypeUnit = s.TypeUnit
			}
			_, _ = svc.SaveUnitBooking(ub)
			return
		}
	}
	_, _ = svc.SaveUnitBooking(domain.UnitBooking{
		NamaProyek: proyek, TypeUnit: s.TypeUnit, BlokNoUnit: blok,
		Status: domain.UnitBooked, SkpID: s.EntID, UpdatedAt: now,
	})
}

// unitBookings lists every tracked unit's status. Any logged-in sales user
// may read it (needed to avoid double-booking); only the Kadep (admin) may
// manage entries directly (setSkp already auto-manages the "booked" flip).
func (h *Handler) unitBookings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.svc.UnitBookings())
}

func (h *Handler) saveUnitBooking(w http.ResponseWriter, r *http.Request) {
	u, ok := decode[domain.UnitBooking](w, r)
	if !ok {
		return
	}
	u.NamaProyek = strings.TrimSpace(u.NamaProyek)
	u.BlokNoUnit = strings.TrimSpace(u.BlokNoUnit)
	if u.NamaProyek == "" || u.BlokNoUnit == "" {
		writeError(w, http.StatusBadRequest, "nama proyek dan blok/no unit wajib diisi")
		return
	}
	switch u.Status {
	case domain.UnitTersedia, domain.UnitBooked, domain.UnitAkad, domain.UnitTerjual, domain.UnitBatal:
	default:
		u.Status = domain.UnitTersedia
	}
	u.UpdatedAt = time.Now().Format(time.RFC3339)
	saved, err := h.svc.SaveUnitBooking(u)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (h *Handler) deleteUnitBooking(w http.ResponseWriter, r *http.Request) {
	ok, err := h.svc.DeleteUnitBooking(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "data tidak ditemukan")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
