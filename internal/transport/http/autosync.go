package http

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"greenpark/sales/internal/domain"
)

// autoSync holds the background auto-sync state: when enabled, the scheduler
// periodically pulls the Google Sheet and applies it (auto-approve), recording
// each run in the import history so it stays rollback-able.
type autoSync struct {
	mu          sync.Mutex
	enabled     bool
	interval    time.Duration
	last        time.Time
	lastErr     string
	lastSummary domain.ImportSummary
}

func newAutoSync(intervalMin int) *autoSync {
	iv := time.Duration(intervalMin) * time.Minute
	if iv <= 0 {
		iv = 10 * time.Minute
	}
	return &autoSync{enabled: intervalMin > 0, interval: iv}
}

// minInterval is the floor for auto-sync: one fetch takes ~2.5s and the Sheets
// API has per-minute quotas, so sub-30s polling is not viable.
const minInterval = 30 * time.Second

// StartAutoSync launches the scheduler goroutine. It checks every 5s whether a
// run is due (enabled && configured && interval elapsed) and runs it.
func (h *Handler) StartAutoSync(ctx context.Context) {
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				a := h.auto
				a.mu.Lock()
				due := a.enabled && h.sync != nil && time.Since(a.last) >= a.interval
				a.mu.Unlock()
				if !due {
					continue
				}
				runCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
				rec, err := h.doSyncApprove(runCtx, "auto-sync")
				cancel()
				a.mu.Lock()
				a.last = time.Now()
				if err != nil {
					a.lastErr = err.Error()
					log.Printf("sales: auto-sync error: %v", err)
				} else {
					a.lastErr = ""
					a.lastSummary = rec.Summary
					log.Printf("sales: auto-sync ok (akad=%d, leads=%d)", rec.Summary.Akad, rec.Summary.Leads)
				}
				a.mu.Unlock()
			}
		}
	}()
}

type autoStatusResp struct {
	Enabled     bool                 `json:"enabled"`
	IntervalSec int                  `json:"intervalSec"`
	Configured  bool                 `json:"configured"`
	LastSync    string               `json:"lastSync"`
	LastError   string               `json:"lastError"`
	LastSummary domain.ImportSummary `json:"lastSummary"`
}

func (h *Handler) autoStatus(w http.ResponseWriter, _ *http.Request) {
	a := h.auto
	a.mu.Lock()
	defer a.mu.Unlock()
	last := ""
	if !a.last.IsZero() {
		last = a.last.Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, autoStatusResp{
		Enabled:     a.enabled,
		IntervalSec: int(a.interval / time.Second),
		Configured:  h.sync != nil,
		LastSync:    last,
		LastError:   a.lastErr,
		LastSummary: a.lastSummary,
	})
}

type autoSetReq struct {
	Enabled     bool `json:"enabled"`
	IntervalSec int  `json:"intervalSec"`
}

func (h *Handler) autoSet(w http.ResponseWriter, r *http.Request) {
	req, ok := decode[autoSetReq](w, r)
	if !ok {
		return
	}
	if req.Enabled && h.sync == nil {
		writeError(w, http.StatusServiceUnavailable, "Sync Google Sheets belum dikonfigurasi.")
		return
	}
	iv := time.Duration(req.IntervalSec) * time.Second
	if iv < minInterval {
		iv = minInterval // floor 30s (fetch ~2.5s + Sheets API quota)
	}
	a := h.auto
	a.mu.Lock()
	a.enabled = req.Enabled
	a.interval = iv
	if a.enabled {
		a.last = time.Time{} // run on the next tick
	}
	a.mu.Unlock()
	h.autoStatus(w, r)
}
