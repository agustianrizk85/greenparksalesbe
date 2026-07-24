package http

import "net/http"

// NewRouter wires all routes to the handler and applies global middleware.
//
// Access tiers:
//   - public: health check + login
//   - authenticated (any logged-in user): all dashboard reads + me/logout
//   - admin only: every master-data write
func NewRouter(h *Handler, allowOrigin string) http.Handler {
	mux := http.NewServeMux()

	// ---- public ----
	mux.HandleFunc("GET /api/health", h.health)
	mux.HandleFunc("POST /api/auth/login", h.login)

	// ---- authenticated session ----
	mux.HandleFunc("GET /api/auth/me", h.requireAuth(h.me))
	mux.HandleFunc("POST /api/auth/logout", h.requireAuth(h.logout))

	// ---- reads (authenticated) ----
	mux.HandleFunc("GET /api/dashboard", h.requireAuth(h.dashboard))
	mux.HandleFunc("GET /api/version", h.requireAuth(h.version))
	mux.HandleFunc("GET /api/ws", h.ws) // auth via ?token= (browser can't set headers)
	mux.HandleFunc("GET /api/summary", h.requireAuth(h.summary))
	mux.HandleFunc("GET /api/exec", h.requireAuth(h.exec))
	mux.HandleFunc("GET /api/funnel", h.requireAuth(h.funnel))
	mux.HandleFunc("GET /api/projects", h.requireAuth(h.projects))
	mux.HandleFunc("GET /api/projects/{code}", h.requireAuth(h.projectByCode))
	mux.HandleFunc("GET /api/channels", h.requireAuth(h.channels))
	mux.HandleFunc("GET /api/sales", h.requireAuth(h.sales))
	mux.HandleFunc("GET /api/reasons", h.requireAuth(h.reasons))
	mux.HandleFunc("GET /api/agents", h.requireAuth(h.agents))
	mux.HandleFunc("GET /api/alerts", h.requireAuth(h.alerts))
	mux.HandleFunc("GET /api/ai/alerts", h.requireAuth(h.aiAlerts))
	mux.HandleFunc("GET /api/kpis", h.requireAuth(h.kpis))

	// ---- konsumen screening ----
	// Any logged-in sales user reads the questionnaire + runs an assessment;
	// staff see only their own submissions. Managing the questions and deleting
	// submissions is admin (Kadep) only.
	mux.HandleFunc("GET /api/screening/questions", h.requireAuth(h.screeningQuestions))
	mux.HandleFunc("PUT /api/screening/questions", h.requireAdmin(h.setScreeningQuestions))
	mux.HandleFunc("GET /api/screening/submissions", h.requireAuth(h.screeningSubmissions))
	mux.HandleFunc("POST /api/screening/assess", h.requireAuth(h.assessScreening))
	mux.HandleFunc("DELETE /api/screening/submissions/{id}", h.requireAdmin(h.deleteScreeningSubmission))

	// ---- SKP (Surat Konfirmasi Pesanan) ----
	// Any logged-in sales user reads the project templates (to prefill a new
	// SKP) and creates/edits their own SKPs; only the Kadep (admin) manages the
	// project templates. Editing/deleting an SKP is restricted to its author or
	// the Kadep (enforced in the handler, since it needs to inspect ownership).
	mux.HandleFunc("GET /api/skp/projects", h.requireAuth(h.skpProjectTemplates))
	mux.HandleFunc("POST /api/skp/projects", h.requireAdmin(h.saveSkpProjectTemplate))
	mux.HandleFunc("DELETE /api/skp/projects/{id}", h.requireAdmin(h.deleteSkpProjectTemplate))
	mux.HandleFunc("GET /api/skp", h.requireAuth(h.skpList))
	mux.HandleFunc("POST /api/skp", h.requireAuth(h.saveSkp))
	mux.HandleFunc("DELETE /api/skp/{id}", h.requireAuth(h.deleteSkp))

	// ---- Master Booking (unit status: tersedia/booked/terjual) ----
	// Any logged-in sales user reads it (avoid double-booking a unit); only the
	// Kadep (admin) edits directly — new SKPs auto-flip a unit to "booked".
	mux.HandleFunc("GET /api/skp/units", h.requireAuth(h.unitBookings))
	mux.HandleFunc("POST /api/skp/units", h.requireAdmin(h.saveUnitBooking))
	mux.HandleFunc("DELETE /api/skp/units/{id}", h.requireAdmin(h.deleteUnitBooking))

	// ---- import / upload pipeline (admin) ----
	mux.HandleFunc("POST /api/import/preview", h.requireAdmin(h.importPreview))
	mux.HandleFunc("POST /api/import/approve", h.requireAdmin(h.importApprove))
	mux.HandleFunc("GET /api/import/history", h.requireAdmin(h.importHistory))
	mux.HandleFunc("POST /api/import/rollback/{id}", h.requireAdmin(h.importRollback))
	mux.HandleFunc("POST /api/import/reset", h.requireAdmin(h.importReset))
	mux.HandleFunc("POST /api/import/sync-preview", h.requireAdmin(h.importSyncPreview))
	mux.HandleFunc("POST /api/import/sync-approve", h.requireAdmin(h.importSyncApprove))
	mux.HandleFunc("GET /api/import/auto", h.requireAdmin(h.autoStatus))
	mux.HandleFunc("PUT /api/import/auto", h.requireAdmin(h.autoSet))

	// ---- singleton writes (admin) ----
	mux.HandleFunc("PUT /api/meta", h.requireAdmin(h.setMeta))
	mux.HandleFunc("PUT /api/exec", h.requireAdmin(h.setExec))
	mux.HandleFunc("PUT /api/stock", h.requireAdmin(h.setStock))
	mux.HandleFunc("PUT /api/events", h.requireAdmin(h.setEvents))
	mux.HandleFunc("PUT /api/funnel", h.requireAdmin(h.setFunnel))
	mux.HandleFunc("PUT /api/monthly", h.requireAdmin(h.setMonthly))
	mux.HandleFunc("PUT /api/reason-meta", h.requireAdmin(h.setReasonMeta))

	// ---- collection writes (admin) ----
	mux.HandleFunc("POST /api/projects", h.requireAdmin(h.saveProject))
	mux.HandleFunc("DELETE /api/projects/{id}", h.requireAdmin(h.deleteHandler(h.svc.DeleteProject)))
	mux.HandleFunc("POST /api/sales", h.requireAdmin(h.saveSalesRep))
	mux.HandleFunc("DELETE /api/sales/{id}", h.requireAdmin(h.deleteHandler(h.svc.DeleteSalesRep)))
	mux.HandleFunc("POST /api/channels", h.requireAdmin(h.saveChannel))
	mux.HandleFunc("DELETE /api/channels/{id}", h.requireAdmin(h.deleteHandler(h.svc.DeleteChannel)))
	mux.HandleFunc("POST /api/reasons", h.requireAdmin(h.saveReason))
	mux.HandleFunc("DELETE /api/reasons/{id}", h.requireAdmin(h.deleteHandler(h.svc.DeleteReason)))
	mux.HandleFunc("POST /api/agents", h.requireAdmin(h.saveAgent))
	mux.HandleFunc("DELETE /api/agents/{id}", h.requireAdmin(h.deleteHandler(h.svc.DeleteAgent)))
	mux.HandleFunc("POST /api/alerts", h.requireAdmin(h.saveAlert))
	mux.HandleFunc("DELETE /api/alerts/{id}", h.requireAdmin(h.deleteHandler(h.svc.DeleteAlert)))
	mux.HandleFunc("POST /api/kpis", h.requireAdmin(h.saveKPI))
	mux.HandleFunc("DELETE /api/kpis/{id}", h.requireAdmin(h.deleteHandler(h.svc.DeleteKPI)))

	return chain(mux, logger, cors(allowOrigin))
}
