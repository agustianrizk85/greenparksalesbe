package http

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// wsHub keeps the set of connected dashboard browsers and pushes a data-revision
// message whenever the backend data changes — giving instant, no-refresh updates
// over a WebSocket (the browser reloads the dashboard on each push).
type wsHub struct {
	mu    sync.Mutex
	conns map[*websocket.Conn]bool
}

func newWSHub() *wsHub { return &wsHub{conns: map[*websocket.Conn]bool{}} }

func (h *wsHub) add(c *websocket.Conn) {
	h.mu.Lock()
	h.conns[c] = true
	h.mu.Unlock()
}

func (h *wsHub) remove(c *websocket.Conn) {
	h.mu.Lock()
	delete(h.conns, c)
	h.mu.Unlock()
	_ = c.Close()
}

// send writes rev to every connection. The hub is the only writer, so writes are
// serialised by its mutex (gorilla conns allow one concurrent writer).
func (h *wsHub) send(rev int64) {
	msg := map[string]int64{"rev": rev}
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.conns {
		_ = c.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := c.WriteJSON(msg); err != nil {
			delete(h.conns, c)
			_ = c.Close()
		}
	}
}

func (h *wsHub) sendTo(c *websocket.Conn, rev int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	_ = c.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_ = c.WriteJSON(map[string]int64{"rev": rev})
}

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool { return true }, // same-trust dev/LAN setup
}

// StartRealtime launches the revision watcher that pushes to WebSocket clients
// within ~1s of any data change.
func (h *Handler) StartRealtime() {
	go func() {
		last := int64(-1)
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for range t.C {
			rev := h.svc.Revision()
			if rev != last {
				last = rev
				h.hub.send(rev)
			}
		}
	}()
}

// ws upgrades the request to a WebSocket. Browsers cannot send the Authorization
// header on a WS handshake, so the bearer token is passed as a query parameter.
func (h *Handler) ws(w http.ResponseWriter, r *http.Request) {
	tok := r.URL.Query().Get("token")
	if _, err := h.auth.Validate(tok); err != nil {
		if _, ok := h.ssoUser(tok); !ok {
			writeError(w, http.StatusUnauthorized, "token tidak valid")
			return
		}
	}
	c, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	h.hub.add(c)
	h.hub.sendTo(c, h.svc.Revision()) // sync immediately on connect
	// Read loop: we don't expect client messages — it only detects disconnect.
	go func() {
		defer h.hub.remove(c)
		c.SetReadLimit(512)
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	}()
}
