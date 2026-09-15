package visual

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"unstable.build/rune/internal/a2a"
	"unstable.build/rune/internal/canon"
	"unstable.build/rune/internal/quilt"
)

// TestSnapshot — fresh server returns a snapshot of the cluster.
func TestSnapshot(t *testing.T) {
	cluster := quilt.NewCluster("test-cluster")
	for i := 0; i < 5; i++ {
		c := quilt.NewCell("c"+string(rune('a'+i)), "scope")
		c.Bind(map[string]interface{}{"i": i})
		cluster.Add(c)
	}
	c := canon.New("http://nonexistent")
	a := a2a.New("http://nonexistent", "vcell", "vrole", []string{"x"})
	srv := NewServer(cluster, c, a)
	if err := srv.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop(context.Background())

	resp, err := http.Get(srv.URL() + "/snapshot")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status: got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var snap Snapshot
	if err := json.Unmarshal(body, &snap); err != nil {
		t.Fatal(err)
	}
	if snap.Cluster != "test-cluster" {
		t.Errorf("cluster: got %s", snap.Cluster)
	}
	if len(snap.Cells) != 5 {
		t.Errorf("cells: got %d want 5", len(snap.Cells))
	}
}

// TestHealth — health endpoint returns ok.
func TestHealth(t *testing.T) {
	cluster := quilt.NewCluster("h")
	c := canon.New("http://x")
	a := a2a.New("http://x", "id", "r", nil)
	srv := NewServer(cluster, c, a)
	if err := srv.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop(context.Background())
	resp, err := http.Get(srv.URL() + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status: got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"status":"ok"`) {
		t.Errorf("body: got %s", body)
	}
}

// TestSSE — server-sent events deliver at least the initial subscription heartbeat.
func TestSSE(t *testing.T) {
	cluster := quilt.NewCluster("sse")
	c := canon.New("http://x")
	a := a2a.New("http://x", "id", "r", nil)
	srv := NewServer(cluster, c, a)
	if err := srv.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop(context.Background())

	// Subscribe and notify
	ch := make(chan Event, 8)
	srv.mu.Lock()
	srv.subs[ch] = struct{}{}
	srv.mu.Unlock()
	defer func() {
		srv.mu.Lock()
		delete(srv.subs, ch)
		srv.mu.Unlock()
	}()

	srv.Notify(Event{Type: "test", TS: time.Now()})

	select {
	case ev := <-ch:
		if ev.Type != "test" {
			t.Errorf("type: got %s", ev.Type)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for event")
	}
}

// TestNotifyWitness — pushes edge event from a witness.
func TestNotifyWitness(t *testing.T) {
	cluster := quilt.NewCluster("nw")
	c := canon.New("http://x")
	a := a2a.New("http://x", "id", "r", nil)
	srv := NewServer(cluster, c, a)
	if err := srv.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop(context.Background())

	ch := make(chan Event, 8)
	srv.mu.Lock()
	srv.subs[ch] = struct{}{}
	srv.mu.Unlock()
	defer func() {
		srv.mu.Lock()
		delete(srv.subs, ch)
		srv.mu.Unlock()
	}()

	cell := quilt.NewCell("foo", "bar")
	cell.Bind(map[string]interface{}{"k": "v"})
	witnesses := cell.Witnesses()
	if len(witnesses) == 0 {
		t.Fatal("cell should have witness")
	}
	srv.NotifyWitness(cell, witnesses[0])

	select {
	case ev := <-ch:
		if ev.Type != "edge" {
			t.Errorf("type: got %s want edge", ev.Type)
		}
		if ev.Edge == nil {
			t.Error("edge should not be nil")
		}
		if ev.Edge.Source != cell.Address() {
			t.Errorf("source: got %s want %s", ev.Edge.Source, cell.Address())
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout")
	}
}

// TestNotifyCell — pushes a new-cell event.
func TestNotifyCell(t *testing.T) {
	cluster := quilt.NewCluster("nc")
	c := canon.New("http://x")
	a := a2a.New("http://x", "id", "r", nil)
	srv := NewServer(cluster, c, a)
	if err := srv.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop(context.Background())

	ch := make(chan Event, 8)
	srv.mu.Lock()
	srv.subs[ch] = struct{}{}
	srv.mu.Unlock()
	defer func() {
		srv.mu.Lock()
		delete(srv.subs, ch)
		srv.mu.Unlock()
	}()

	cell := quilt.NewCell("foo", "bar")
	srv.NotifyCell(cell)

	select {
	case ev := <-ch:
		if ev.Type != "cell-added" {
			t.Errorf("type: got %s want cell-added", ev.Type)
		}
		if ev.Node == nil {
			t.Fatal("node should not be nil")
		}
		if ev.Node.Name != "foo" {
			t.Errorf("name: got %s", ev.Node.Name)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout")
	}
}

// TestSlowSubscriberDrops — slow subscribers don't block.
func TestSlowSubscriberDrops(t *testing.T) {
	cluster := quilt.NewCluster("slow")
	c := canon.New("http://x")
	a := a2a.New("http://x", "id", "r", nil)
	srv := NewServer(cluster, c, a)

	// Full channel (cap 32), but don't drain
	ch := make(chan Event, 4) // small buffer
	srv.mu.Lock()
	srv.subs[ch] = struct{}{}
	srv.mu.Unlock()
	defer func() {
		srv.mu.Lock()
		delete(srv.subs, ch)
		srv.mu.Unlock()
	}()

	// Fire 100 events; should not block
	for i := 0; i < 100; i++ {
		srv.Notify(Event{Type: "test"})
	}
	// If we got here without blocking, the test passes.
}
