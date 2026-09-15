// Package visual — server.go implements the live HTTP dashboard.
package visual

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"unstable.build/rune/internal/a2a"
	"unstable.build/rune/internal/canon"
	"unstable.build/rune/internal/quilt"
)

//go:embed dashboard.html
var dashboardHTML []byte

// Server is the live visual server.
type Server struct {
	mu      sync.RWMutex
	cluster *quilt.Cluster
	canon   *canon.Client
	a2a     *a2a.Client
	subs    map[chan Event]struct{}
	srv     *http.Server
	ln      net.Listener
	port    int
}

// NewServer creates a visual server bound to localhost.
func NewServer(cluster *quilt.Cluster, c *canon.Client, a *a2a.Client) *Server {
	s := &Server{
		cluster: cluster,
		canon:   c,
		a2a:     a,
		subs:    map[chan Event]struct{}{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/snapshot", s.handleSnapshot)
	mux.HandleFunc("/events", s.handleSSE)
	mux.HandleFunc("/health", s.handleHealth)
	s.srv = &http.Server{
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 0, // SSE streams stay open
	}
	return s
}

// Start binds to localhost on a random port and serves until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	s.ln = ln
	s.port = ln.Addr().(*net.TCPAddr).Port
	go func() {
		_ = s.srv.Serve(ln)
	}()
	return nil
}

// Port returns the bound port.
func (s *Server) Port() int { return s.port }

// URL returns the localhost URL.
func (s *Server) URL() string { return fmt.Sprintf("http://127.0.0.1:%d", s.port) }

// Stop closes all subscriptions and shuts down the server.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	for ch := range s.subs {
		close(ch)
		delete(s.subs, ch)
	}
	s.mu.Unlock()
	return s.srv.Shutdown(ctx)
}

// Notify pushes an event to all subscribers.
func (s *Server) Notify(ev Event) {
	ev.TS = time.Now().UTC()
	s.mu.RLock()
	defer s.mu.RUnlock()
	for ch := range s.subs {
		select {
		case ch <- ev:
		default:
			// Drop if subscriber is slow
		}
	}
}

// witnessPayload extracts a "note" from a witness Payload map.
func witnessNote(w quilt.Witness) string {
	if v, ok := w.Payload["note"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return string(w.Op)
}

// NotifyWitness pushes an edge event for a witness entry.
func (s *Server) NotifyWitness(cell *quilt.Cell, w quilt.Witness) {
	if cell == nil {
		return
	}
	note := witnessNote(w)
	s.Notify(Event{
		Type: "edge",
		Edge: &Edge{
			Source: cell.Address(),
			Target: note,
			Type:   w.Op,
			Tick:   w.Ts,
		},
		Witness: &Witness{
			ID:   w.EntryHash,
			Op:   w.Op,
			Cell: cell.Address(),
			Note: note,
			Tick: w.Ts,
		},
	})
}

// NotifyCell pushes a new-cell event.
func (s *Server) NotifyCell(c *quilt.Cell) {
	if c == nil {
		return
	}
	s.Notify(Event{
		Type: "cell-added",
		Node: &Node{
			ID:    c.Address(),
			Name:  c.Name,
			Scope: c.Scope,
			Role:  "cell",
			Tier:  c.Tier,
		},
	})
}

// RefreshStats pushes a stats update.
func (s *Server) RefreshStats(ctx context.Context) {
	s.mu.RLock()
	cellCount := len(s.cluster.All())
	stats := Stats{
		CellCount: cellCount,
		Cluster:   s.cluster.Name,
	}
	if s.canon != nil {
		if h, err := s.canon.GetHash(ctx); err == nil {
			stats.CanonHash = h.StateHash
		}
	}
	if s.a2a != nil {
		stats.A2ACellID = s.a2a.CellID
		stats.A2AURL = s.a2a.Base
	}
	s.mu.RUnlock()
	s.Notify(Event{Type: "stats", Stats: &stats})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(dashboardHTML)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":    "ok",
		"cluster":   s.cluster.Name,
		"timestamp": time.Now().UTC(),
	})
}

func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	cells := s.cluster.All()
	nodeList := make([]Node, 0, len(cells))
	edgeList := make([]Edge, 0)
	for _, c := range cells {
		nodeList = append(nodeList, Node{
			ID:    c.Address(),
			Name:  c.Name,
			Scope: c.Scope,
			Role:  "cell",
			Tier:  c.Tier,
		})
		for _, w := range c.Witnesses() {
			edgeList = append(edgeList, Edge{
				Source: c.Address(),
				Target: witnessNote(w),
				Type:   w.Op,
				Tick:   w.Ts,
			})
		}
	}
	stats := Stats{
		CellCount: len(cells),
		Cluster:   s.cluster.Name,
	}
	if s.canon != nil {
		if h, err := s.canon.GetHash(r.Context()); err == nil {
			stats.CanonHash = h.StateHash
		}
	}
	if s.a2a != nil {
		stats.A2ACellID = s.a2a.CellID
		stats.A2AURL = s.a2a.Base
	}
	s.mu.RUnlock()
	snap := Snapshot{
		Cluster:  s.cluster.Name,
		Cells:    nodeList,
		Edges:    edgeList,
		Stats:    stats,
		ServerTS: time.Now().UTC(),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(snap)
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ch := make(chan Event, 32)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.subs, ch)
		s.mu.Unlock()
	}()

	// Heartbeat
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev := <-ch:
			data, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-ticker.C:
			_, _ = fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}
