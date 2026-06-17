package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"greenpark/sales/internal/auth"
	"greenpark/sales/internal/domain"
	"greenpark/sales/internal/gsheets"
	"greenpark/sales/internal/repository"
	"greenpark/sales/internal/service"
)

// Handler holds the dependencies for the HTTP handlers.
type Handler struct {
	svc     service.SalesService
	auth    *auth.Service
	sync    *gsheets.Client
	sheetID string
	auto    *autoSync
	hub     *wsHub
}

// NewHandler creates a Handler bound to the service and auth service. sync may
// be nil when Google Sheets sync is not configured. intervalMin seeds the
// auto-sync schedule (0 = disabled until turned on from the UI).
func NewHandler(svc service.SalesService, authSvc *auth.Service, sync *gsheets.Client, sheetID string, intervalMin int) *Handler {
	return &Handler{svc: svc, auth: authSvc, sync: sync, sheetID: sheetID, auto: newAutoSync(intervalMin), hub: newWSHub()}
}

/* ---------------------------- auth plumbing ---------------------------- */

type ctxKey int

const userCtxKey ctxKey = 0

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(h[len("Bearer "):])
	}
	return ""
}

// requireAuth wraps a handler, rejecting requests without a valid session.
func (h *Handler) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := h.auth.Validate(bearer(r))
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userCtxKey, u)))
	}
}

// requireAdmin wraps a handler, requiring a valid session with the admin role.
func (h *Handler) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return h.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		if u, ok := r.Context().Value(userCtxKey).(domain.User); !ok || u.Role != domain.RoleAdmin {
			writeError(w, http.StatusForbidden, "butuh akses admin")
			return
		}
		next(w, r)
	})
}

// decode reads the JSON request body into a value of type T.
func decode[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var v T
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		writeError(w, http.StatusBadRequest, "body JSON tidak valid: "+err.Error())
		return v, false
	}
	return v, true
}

/* ---------------------------- auth handlers ---------------------------- */

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[loginReq](w, r)
	if !ok {
		return
	}
	token, user, err := h.auth.Login(req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": user})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	h.auth.Logout(bearer(r))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	u, _ := r.Context().Value(userCtxKey).(domain.User)
	writeJSON(w, http.StatusOK, u)
}

/* ---------------------------- read handlers ---------------------------- */

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "sales"})
}

func (h *Handler) dashboard(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.svc.Dashboard())
}

// version returns the current data revision; the front-end polls it cheaply and
// reloads the dashboard only when it changes (realtime auto-refresh).
func (h *Handler) version(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]int64{"rev": h.svc.Revision()})
}

func (h *Handler) summary(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.svc.Summary())
}

func (h *Handler) exec(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.svc.Exec())
}

func (h *Handler) funnel(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.svc.Funnel())
}

func (h *Handler) projects(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.svc.Projects())
}

func (h *Handler) projectByCode(w http.ResponseWriter, r *http.Request) {
	project, err := h.svc.ProjectByCode(r.PathValue("code"))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load project")
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func (h *Handler) channels(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.svc.Channels())
}

func (h *Handler) sales(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.svc.Sales())
}

func (h *Handler) reasons(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.svc.Reasons())
}

func (h *Handler) agents(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.svc.Agents())
}

func (h *Handler) alerts(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.svc.Alerts())
}

func (h *Handler) kpis(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.svc.KPIs())
}

/* ---------------------------- singleton write handlers ---------------------------- */

type metaReq struct {
	Period  string `json:"period"`
	Updated string `json:"updated"`
}

func (h *Handler) setMeta(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[metaReq](w, r)
	if !ok {
		return
	}
	if err := h.svc.SetMeta(req.Period, req.Updated); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, req)
}

func (h *Handler) setExec(w http.ResponseWriter, r *http.Request) {
	v, ok := decode[domain.Exec](w, r)
	if !ok {
		return
	}
	if err := h.svc.SetExec(v); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h *Handler) setStock(w http.ResponseWriter, r *http.Request) {
	v, ok := decode[domain.MasterStock](w, r)
	if !ok {
		return
	}
	if err := h.svc.SetStock(v); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h *Handler) setEvents(w http.ResponseWriter, r *http.Request) {
	v, ok := decode[domain.Events](w, r)
	if !ok {
		return
	}
	if err := h.svc.SetEvents(v); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h *Handler) setFunnel(w http.ResponseWriter, r *http.Request) {
	v, ok := decode[[]domain.FunnelStage](w, r)
	if !ok {
		return
	}
	if err := h.svc.SetFunnel(v); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h *Handler) setMonthly(w http.ResponseWriter, r *http.Request) {
	v, ok := decode[[]domain.MonthPoint](w, r)
	if !ok {
		return
	}
	if err := h.svc.SetMonthly(v); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h *Handler) setReasonMeta(w http.ResponseWriter, r *http.Request) {
	v, ok := decode[map[string]domain.ReasonMetaItem](w, r)
	if !ok {
		return
	}
	if err := h.svc.SetReasonMeta(v); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, v)
}

/* ---------------------------- collection write handlers ---------------------------- */

func (h *Handler) saveProject(w http.ResponseWriter, r *http.Request) {
	v, ok := decode[domain.Project](w, r)
	if !ok {
		return
	}
	saved, err := h.svc.SaveProject(v)
	respondSave(w, saved, err)
}

func (h *Handler) saveSalesRep(w http.ResponseWriter, r *http.Request) {
	v, ok := decode[domain.SalesRep](w, r)
	if !ok {
		return
	}
	saved, err := h.svc.SaveSalesRep(v)
	respondSave(w, saved, err)
}

func (h *Handler) saveChannel(w http.ResponseWriter, r *http.Request) {
	v, ok := decode[domain.Channel](w, r)
	if !ok {
		return
	}
	saved, err := h.svc.SaveChannel(v)
	respondSave(w, saved, err)
}

func (h *Handler) saveReason(w http.ResponseWriter, r *http.Request) {
	v, ok := decode[domain.Reason](w, r)
	if !ok {
		return
	}
	saved, err := h.svc.SaveReason(v)
	respondSave(w, saved, err)
}

func (h *Handler) saveAgent(w http.ResponseWriter, r *http.Request) {
	v, ok := decode[domain.Agent](w, r)
	if !ok {
		return
	}
	saved, err := h.svc.SaveAgent(v)
	respondSave(w, saved, err)
}

func (h *Handler) saveAlert(w http.ResponseWriter, r *http.Request) {
	v, ok := decode[domain.Alert](w, r)
	if !ok {
		return
	}
	saved, err := h.svc.SaveAlert(v)
	respondSave(w, saved, err)
}

func (h *Handler) saveKPI(w http.ResponseWriter, r *http.Request) {
	v, ok := decode[domain.KPI](w, r)
	if !ok {
		return
	}
	saved, err := h.svc.SaveKPI(v)
	respondSave(w, saved, err)
}

func respondSave[T any](w http.ResponseWriter, saved T, err error) {
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

// deleteFunc adapts a repository delete to an HTTP handler keyed on {id}.
func (h *Handler) deleteHandler(del func(id string) (bool, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ok, err := del(r.PathValue("id"))
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
}
