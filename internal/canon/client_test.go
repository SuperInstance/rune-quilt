package canon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestGetHash — happy path: mock server returns Hash JSON.
func TestGetHash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/canon/hash" {
			t.Errorf("path: got %s want /api/canon/hash", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(Hash{
			StateHash:  "0xdeadbeef",
			PaperCount: 14,
		})
	}))
	defer srv.Close()
	c := New(srv.URL)
	h, err := c.GetHash(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if h.StateHash != "0xdeadbeef" {
		t.Errorf("state: got %s", h.StateHash)
	}
	if h.PaperCount != 14 {
		t.Errorf("papers: got %d", h.PaperCount)
	}
}

// TestGetHashError — server returns 500.
func TestGetHashError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := New(srv.URL)
	_, err := c.GetHash(context.Background())
	if err == nil {
		t.Error("expected error on 500")
	}
}

// TestList — server returns N papers wrapped in {papers: [...]}.
func TestList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/canon") {
			t.Errorf("path: got %s want /api/canon*", r.URL.Path)
		}
		if r.URL.Query().Get("n") != "10" {
			t.Errorf("query n: got %s want 10", r.URL.Query().Get("n"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"papers":[{"id":"115","title":"first"},{"id":"116","title":"second"}]}`))
	}))
	defer srv.Close()
	c := New(srv.URL)
	papers, err := c.List(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(papers) != 2 {
		t.Errorf("count: got %d want 2", len(papers))
	}
	if papers[0].Title != "first" {
		t.Errorf("first title: got %s", papers[0].Title)
	}
}

// TestSubmit — server receives correct body.
func TestSubmit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/cell" {
			t.Errorf("path: got %s want /api/cell", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method: got %s want POST", r.Method)
		}
		var got CellSubmission
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		if got.Title != "test-cell" {
			t.Errorf("title: got %q", got.Title)
		}
		_ = json.NewEncoder(w).Encode(CellResponse{ID: "115"})
	}))
	defer srv.Close()
	c := New(srv.URL)
	dials := make([]int, 16)
	for i := range dials {
		dials[i] = i
	}
	res, err := c.Submit(context.Background(), CellSubmission{
		Dials: dials,
		Refs:  []int{1, 2},
		Title: "test-cell",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ID != "115" {
		t.Errorf("id: got %s", res.ID)
	}
}

// TestNavigate — server returns NavigateResult.
func TestNavigate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/canon/navigate") {
			t.Errorf("path: got %s", r.URL.Path)
		}
		if r.URL.Query().Get("paper") != "115" {
			t.Errorf("paper query: got %s", r.URL.Query().Get("paper"))
		}
		_ = json.NewEncoder(w).Encode(NavigateResult{
			Papers: []Paper{
				{ID: "115", Title: "first"},
				{ID: "116", Title: "second"},
			},
		})
	}))
	defer srv.Close()
	c := New(srv.URL)
	res, err := c.Navigate(context.Background(), 115, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Papers) != 2 {
		t.Errorf("papers: got %d want 2", len(res.Papers))
	}
}

// TestLineage — server returns lineage chain.
func TestLineage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/canon/lineage") {
			t.Errorf("path: got %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(LineageResult{
			Papers: []Paper{
				{ID: "100"},
				{ID: "105"},
			},
		})
	}))
	defer srv.Close()
	c := New(srv.URL)
	res, err := c.Lineage(context.Background(), "115")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Papers) != 2 {
		t.Errorf("papers: got %d", len(res.Papers))
	}
}

// TestVibe — server accepts and returns VibeResponse.
func TestVibe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/vibe") {
			t.Errorf("path: got %s want /api/vibe*", r.URL.Path)
		}
		if r.URL.Query().Get("lang") != "rust" {
			t.Errorf("lang query: got %s", r.URL.Query().Get("lang"))
		}
		_ = json.NewEncoder(w).Encode(VibeResponse{
			Lang: "rust",
			Vibe: "high-energy",
		})
	}))
	defer srv.Close()
	c := New(srv.URL)
	res, err := c.Vibe(context.Background(), VibeRequest{Lang: "rust"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Vibe != "high-energy" {
		t.Errorf("vibe: got %s", res.Vibe)
	}
}

// TestContextCancel — context cancellation propagates.
func TestContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()
	c := New(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.GetHash(ctx)
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

// TestNew — New preserves trailing slash (it doesn't strip).
// (New() doesn't trim; tests should use exact match)
func TestNew(t *testing.T) {
	c := New("http://example.com")
	if c.Base != "http://example.com" {
		t.Errorf("base: got %s want http://example.com", c.Base)
	}
}

// TestErrorFormat — error wraps server message.
func TestErrorFormat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "out of cheese", http.StatusBadRequest)
	}))
	defer srv.Close()
	c := New(srv.URL)
	_, err := c.List(context.Background(), 10)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("err: got %v should mention status 400", err)
	}
}
