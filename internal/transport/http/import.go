package http

import (
	"context"
	"errors"
	"net/http"

	"greenpark/sales/internal/domain"
)

// maxUploadBytes caps the accepted workbook size (25 MiB).
const maxUploadBytes = 25 << 20

// uploadFile reads the multipart "file" field, enforcing the size cap. It writes
// the error response itself and returns ok=false on failure.
func uploadFile(w http.ResponseWriter, r *http.Request) (multipartFile, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		writeError(w, http.StatusBadRequest, "upload tidak valid / file terlalu besar: "+err.Error())
		return multipartFile{}, false
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "field 'file' tidak ditemukan")
		return multipartFile{}, false
	}
	return multipartFile{file: file, name: header.Filename, close: func() { _ = file.Close() }}, true
}

type multipartFile struct {
	file  interface{ Read([]byte) (int, error) }
	name  string
	close func()
}

// importPreview parses the upload and returns the validated preview without
// touching the live dashboard.
func (h *Handler) importPreview(w http.ResponseWriter, r *http.Request) {
	mf, ok := uploadFile(w, r)
	if !ok {
		return
	}
	defer mf.close()
	res, err := h.svc.PreviewImport(mf.file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "gagal membaca workbook: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// importApprove parses the upload, applies it to the dashboard and records a
// rollback snapshot + history entry.
func (h *Handler) importApprove(w http.ResponseWriter, r *http.Request) {
	mf, ok := uploadFile(w, r)
	if !ok {
		return
	}
	defer mf.close()
	by := "-"
	if u, ok := r.Context().Value(userCtxKey).(domain.User); ok {
		by = u.Username
	}
	rec, err := h.svc.ApproveImport(mf.file, mf.name, by)
	if err != nil {
		writeError(w, http.StatusBadRequest, "gagal memproses workbook: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

// neededSheets are the source sheets the engine reads from Google Sheets.
var neededSheets = []string{
	"DATA PENJUALAN", "MASTER DATA_LEADS", "MASTER DATA_VISITOR",
	"MASTER STOCK UNIT", "META ADS INPUT",
}

// fetchSheets pulls the source sheets from the configured Google Spreadsheet.
// It writes the error response itself and returns ok=false on failure.
func (h *Handler) fetchSheets(w http.ResponseWriter, r *http.Request) (map[string][][]string, bool) {
	if h.sync == nil {
		writeError(w, http.StatusServiceUnavailable,
			"Sync Google Sheets belum dikonfigurasi — set SALES_GOOGLE_CREDENTIALS (service account JSON) & share spreadsheet ke email service account.")
		return nil, false
	}
	data, err := h.sync.Fetch(r.Context(), h.sheetID, neededSheets)
	if err != nil {
		writeError(w, http.StatusBadGateway, "gagal ambil Google Sheets: "+err.Error())
		return nil, false
	}
	return data, true
}

// importSyncPreview fetches the Google Sheet and returns the validated preview.
func (h *Handler) importSyncPreview(w http.ResponseWriter, r *http.Request) {
	data, ok := h.fetchSheets(w, r)
	if !ok {
		return
	}
	res, err := h.svc.PreviewSheets(data)
	if err != nil {
		writeError(w, http.StatusBadRequest, "gagal memproses sheet: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// importSyncApprove fetches the Google Sheet, applies it and records the import.
func (h *Handler) importSyncApprove(w http.ResponseWriter, r *http.Request) {
	data, ok := h.fetchSheets(w, r)
	if !ok {
		return
	}
	by := "-"
	if u, ok := r.Context().Value(userCtxKey).(domain.User); ok {
		by = u.Username
	}
	rec, err := h.svc.ApproveSheets(data, "Google Sheets (sync)", by)
	if err != nil {
		writeError(w, http.StatusBadRequest, "gagal memproses sheet: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

// doSyncApprove fetches the Google Sheet and applies it (used by the scheduler).
func (h *Handler) doSyncApprove(ctx context.Context, by string) (domain.ImportRecord, error) {
	if h.sync == nil {
		return domain.ImportRecord{}, errors.New("sync belum dikonfigurasi")
	}
	data, err := h.sync.Fetch(ctx, h.sheetID, neededSheets)
	if err != nil {
		return domain.ImportRecord{}, err
	}
	return h.svc.ApproveSheets(data, "Google Sheets (auto-sync)", by)
}

// importHistory lists past imports (newest first).
func (h *Handler) importHistory(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.svc.ImportHistory())
}

// importReset clears all dashboard data back to empty (reversible via history).
func (h *Handler) importReset(w http.ResponseWriter, r *http.Request) {
	by := "-"
	if u, ok := r.Context().Value(userCtxKey).(domain.User); ok {
		by = u.Username
	}
	rec, err := h.svc.ResetData(by)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

// importRollback undoes a prior import by id.
func (h *Handler) importRollback(w http.ResponseWriter, r *http.Request) {
	rec, err := h.svc.RollbackImport(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "import tidak ditemukan atau tidak bisa di-rollback")
		return
	}
	writeJSON(w, http.StatusOK, rec)
}
