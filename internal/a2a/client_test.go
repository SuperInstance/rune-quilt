package a2a

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(srvURL string) *Client {
	return New(srvURL, "test-cell", "test-role", []string{"cap-1", "cap-2"})
}

// TestInfo — happy path.
func TestInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			t.Errorf("path: got %s want /", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(ServiceInfo{
			Service:  "quilt-a2a-v2",
			Version:  "2.0.0",
			Features: []string{"semantic-search"},
		})
	}))
	defer srv.Close()
	c := newTestClient(srv.URL)
	info, err := c.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Service != "quilt-a2a-v2" {
		t.Errorf("service: got %s", info.Service)
	}
}

// TestRegister — server receives cell metadata.
func TestRegister(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/register" {
			t.Errorf("path: got %s want /register", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method: got %s want POST", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(RegisterResult{
			Cell: &Cell{
				CellID: "test-cell",
				Role:   "test-role",
			},
			TotalCells: 1,
		})
	}))
	defer srv.Close()
	c := newTestClient(srv.URL)
	res, err := c.Register(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Cell.CellID != "test-cell" {
		t.Errorf("id: got %s", res.Cell.CellID)
	}
	if gotBody["cell_id"] != "test-cell" {
		t.Errorf("body.cell_id: got %v", gotBody["cell_id"])
	}
}

// TestTick — server returns inbox + total.
func TestTick(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tick" {
			t.Errorf("path: got %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(TickResult{
			TotalCells: 5,
			InboxCount: 3,
			Messages: []Message{
				{ID: "m1", From: "other-cell", Type: "ping"},
			},
		})
	}))
	defer srv.Close()
	c := newTestClient(srv.URL)
	res, err := c.Tick(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalCells != 5 {
		t.Errorf("total: got %d", res.TotalCells)
	}
	if res.InboxCount != 3 {
		t.Errorf("inbox: got %d", res.InboxCount)
	}
	if len(res.Messages) != 1 {
		t.Errorf("inbox length: got %d", len(res.Messages))
	}
}

// TestFindSemantic — server returns ranked matches.
func TestFindSemantic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/find-semantic") {
			t.Errorf("path: got %s", r.URL.Path)
		}
		if r.URL.Query().Get("q") != "compile" {
			t.Errorf("query q: got %s", r.URL.Query().Get("q"))
		}
		_ = json.NewEncoder(w).Encode(FindSemanticResult{
			Matches: []SemanticHit{
				{ID: "polyformal-1", Score: 0.85,
					Metadata: map[string]interface{}{"role": "polyformal"}},
				{ID: "shaper-1", Score: 0.72,
					Metadata: map[string]interface{}{"role": "shaper"}},
			},
		})
	}))
	defer srv.Close()
	c := newTestClient(srv.URL)
	res, err := c.FindSemantic(context.Background(), "compile", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 2 {
		t.Errorf("matches: got %d", len(res.Matches))
	}
	if res.Matches[0].ID != "polyformal-1" {
		t.Errorf("first id: got %s", res.Matches[0].ID)
	}
}

// TestSend — server accepts unicast.
func TestSend(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/send" {
			t.Errorf("path: got %s", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(SendResult{
			OK: true,
			Msg: Message{
				ID:   "msg-1",
				From: "test-cell",
				To:   "target-cell",
			},
		})
	}))
	defer srv.Close()
	c := newTestClient(srv.URL)
	res, err := c.Send(context.Background(), "target-cell", "ping", map[string]interface{}{"hello": "world"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Error("not ok")
	}
	if gotBody["to"] != "target-cell" {
		t.Errorf("body.to: got %v", gotBody["to"])
	}
}

// TestBroadcast — server accepts broadcast.
func TestBroadcast(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/broadcast" {
			t.Errorf("path: got %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(BroadcastResult{
			Delivered: 3,
			Msgs: []Message{
				{ID: "m1"}, {ID: "m2"}, {ID: "m3"},
			},
		})
	}))
	defer srv.Close()
	c := newTestClient(srv.URL)
	res, err := c.Broadcast(context.Background(), "announce", nil, "compile")
	if err != nil {
		t.Fatal(err)
	}
	if res.Delivered != 3 {
		t.Errorf("count: got %d", res.Delivered)
	}
}

// TestMitosis — server creates child cell.
func TestMitosis(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mitosis" {
			t.Errorf("path: got %s", r.URL.Path)
		}
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["parent_id"] != "parent-cell" {
			t.Errorf("parent_id: got %v", body["parent_id"])
		}
		_ = json.NewEncoder(w).Encode(RegisterResult{
			Cell: &Cell{
				CellID:   "child-cell",
				Role:     "polyformal",
				ParentID: "parent-cell",
				Lineage:  []string{"parent-cell"},
			},
			TotalCells: 2,
		})
	}))
	defer srv.Close()
	c := newTestClient(srv.URL)
	res, err := c.Mitosis(context.Background(), "parent-cell")
	if err != nil {
		t.Fatal(err)
	}
	if res.Cell.ParentID != "parent-cell" {
		t.Errorf("parent: got %s", res.Cell.ParentID)
	}
	if len(res.Cell.Lineage) != 1 {
		t.Errorf("lineage: got %d", len(res.Cell.Lineage))
	}
}

// TestHeritage — server returns lineage chain.
func TestHeritage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/heritage") {
			t.Errorf("path: got %s", r.URL.Path)
		}
		if r.URL.Query().Get("cell_id") != "child-cell" {
			t.Errorf("query cell_id: got %s", r.URL.Query().Get("cell_id"))
		}
		_ = json.NewEncoder(w).Encode(HeritageResult{
			CellID: "child-cell",
			Parent: &Cell{CellID: "parent-cell"},
			Lineage: []Cell{
				{CellID: "root"},
				{CellID: "parent-cell"},
			},
		})
	}))
	defer srv.Close()
	c := newTestClient(srv.URL)
	res, err := c.Heritage(context.Background(), "child-cell")
	if err != nil {
		t.Fatal(err)
	}
	if res.Parent.CellID != "parent-cell" {
		t.Errorf("parent: got %s", res.Parent.CellID)
	}
	if len(res.Lineage) != 2 {
		t.Errorf("lineage: got %d", len(res.Lineage))
	}
}

// TestServerError — server returns 500.
func TestServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := newTestClient(srv.URL)
	_, err := c.Info(context.Background())
	if err == nil {
		t.Error("expected error on 500")
	}
}

// TestNew — constructor sets fields.
func TestNew(t *testing.T) {
	c := New("http://x", "id", "role", []string{"a", "b"})
	if c.Base != "http://x" {
		t.Errorf("base: got %s", c.Base)
	}
	if c.CellID != "id" {
		t.Errorf("id: got %s", c.CellID)
	}
	if c.Role != "role" {
		t.Errorf("role: got %s", c.Role)
	}
	if len(c.Caps) != 2 {
		t.Errorf("caps: got %d", len(c.Caps))
	}
}

// TestContextCancel — context cancel propagates.
func TestContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()
	c := newTestClient(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.Info(ctx)
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

// TestUrlEncode — urlEncode helper.
func TestUrlEncode(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"hello", "hello"},
		{"hello world", "hello+world"},
		{"a&b=c", "a%26b%3Dc"},
	}
	for _, tt := range tests {
		if got := urlEncode(tt.in); got != tt.want {
			t.Errorf("urlEncode(%q): got %q want %q", tt.in, got, tt.want)
		}
	}
}

// TestHexDigit — hex helper.
func TestHexDigit(t *testing.T) {
	tests := []struct {
		in  byte
		want byte
	}{
		{0, '0'}, {9, '9'}, {10, 'A'}, {15, 'F'},
	}
	for _, tt := range tests {
		if got := hexDigit(tt.in); got != tt.want {
			t.Errorf("hexDigit(%d): got %q want %q", tt.in, got, tt.want)
		}
	}
}
