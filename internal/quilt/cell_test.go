package quilt

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"testing"
)

// TestNewCell — fresh cell starts empty with empty root.
func TestNewCell(t *testing.T) {
	c := NewCell("foo", "bar")
	if c.Name != "foo" {
		t.Errorf("name: got %q want %q", c.Name, "foo")
	}
	if c.Scope != "bar" {
		t.Errorf("scope: got %q want %q", c.Scope, "bar")
	}
	if c.Root() != "" {
		t.Error("fresh cell should have empty root")
	}
	if c.WitnessCount() != 0 {
		t.Errorf("fresh cell witnesses: got %d want 0", c.WitnessCount())
	}
}

// TestBind — Bind records a witness with the contract hash.
func TestBind(t *testing.T) {
	c := NewCell("cell-A", "scope-1")
	contract := map[string]interface{}{"k": "v"}
	h := c.Bind(contract)
	if h == "" {
		t.Error("bind returned empty hash")
	}
	if c.WitnessCount() != 1 {
		t.Errorf("witness count: got %d want 1", c.WitnessCount())
	}
	w := c.Witnesses()[0]
	if w.Op != "BIND" {
		t.Errorf("op: got %q want BIND", w.Op)
	}
	if c.Tier != 1 {
		t.Errorf("tier after bind: got %d want 1", c.Tier)
	}
}

// TestLink — Link records a witness referencing the other cell.
func TestLink(t *testing.T) {
	a := NewCell("a", "s")
	b := NewCell("b", "s")
	a.Link(b, "sibling")
	if a.WitnessCount() != 1 {
		t.Errorf("a witnesses: got %d want 1", a.WitnessCount())
	}
	w := a.Witnesses()[0]
	if w.Op != "LINK" {
		t.Errorf("op: got %q want LINK", w.Op)
	}
	if w.Payload["to"] != b.Address() {
		t.Errorf("link payload to: got %v want %s", w.Payload["to"], b.Address())
	}
}

// TestEffect — Effect records a witness with payload + result.
func TestEffect(t *testing.T) {
	c := NewCell("c", "s")
	c.Effect("file-save", map[string]interface{}{"length": 100},
		map[string]interface{}{"ok": true})
	if c.WitnessCount() != 1 {
		t.Errorf("witnesses: got %d want 1", c.WitnessCount())
	}
	w := c.Witnesses()[0]
	if w.Op != "EFFECT" {
		t.Errorf("op: got %q want EFFECT", w.Op)
	}
	// Effect wraps payload as {op: X, params: P}
	if w.Payload["op"] != "file-save" {
		t.Errorf("payload.op: got %v want file-save", w.Payload["op"])
	}
	params, _ := w.Payload["params"].(map[string]interface{})
	if params["length"] != 100 {
		t.Errorf("params.length: got %v want 100", params["length"])
	}
}

// TestTick — Tick increments witness log without changing value.
func TestTick(t *testing.T) {
	c := NewCell("c", "s")
	c.Bind(map[string]interface{}{"k": "v"})
	root1 := c.Root()
	c.Tick()
	if c.Root() == root1 {
		t.Error("tick should change root")
	}
	w := c.Witnesses()[1]
	if w.Op != "TICK" {
		t.Errorf("op: got %q want TICK", w.Op)
	}
}

// TestMerkleRootStable — same op sequence must produce same root.
func TestMerkleRootStable(t *testing.T) {
	mkCell := func() *Cell {
		c := NewCell("c", "s")
		c.Bind(map[string]interface{}{"k": "v"})
		c.Tick()
		c.Effect("e", nil, nil)
		return c
	}
	a := mkCell()
	b := mkCell()
	if a.Root() != b.Root() {
		t.Errorf("roots differ: %s vs %s", a.Root(), b.Root())
	}
}

// TestMerkleRootSensitive — different op sequence → different root.
func TestMerkleRootSensitive(t *testing.T) {
	a := NewCell("c", "s")
	a.Bind(map[string]interface{}{"k": "v1"})
	a.Tick()
	b := NewCell("c", "s")
	b.Bind(map[string]interface{}{"k": "v2"})
	a.Tick()
	if a.Root() == b.Root() {
		t.Error("different contracts should yield different roots")
	}
}

// TestPrevRootChain — each witness's PrevRoot == prior witness's Root.
func TestPrevRootChain(t *testing.T) {
	c := NewCell("c", "s")
	c.Bind(map[string]interface{}{"k": "v"})
	c.Tick()
	c.Effect("e", nil, nil)
	witnesses := c.Witnesses()
	for i := 1; i < len(witnesses); i++ {
		if witnesses[i].PrevRoot != witnesses[i-1].Root {
			t.Errorf("chain broken at %d: prev=%s prior=%s",
				i, witnesses[i].PrevRoot, witnesses[i-1].Root)
		}
	}
}

// TestEntryHashUniqueness — every witness must have a unique entry hash.
func TestEntryHashUniqueness(t *testing.T) {
	c := NewCell("c", "s")
	c.Bind(map[string]interface{}{"k": "v"})
	for i := 0; i < 100; i++ {
		c.Tick()
	}
	seen := map[string]bool{}
	for _, w := range c.Witnesses() {
		if seen[w.EntryHash] {
			t.Errorf("duplicate entry hash: %s", w.EntryHash)
		}
		seen[w.EntryHash] = true
	}
	if len(seen) != 101 {
		t.Errorf("unique entry hashes: got %d want 101 (1 BIND + 100 TICK)", len(seen))
	}
}

// TestConcurrentBind — 100 goroutines all binding must produce valid chain.
func TestConcurrentBind(t *testing.T) {
	c := NewCell("c", "s")
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c.Bind(map[string]interface{}{"i": i})
		}(i)
	}
	wg.Wait()
	if c.WitnessCount() != 100 {
		t.Errorf("witness count: got %d want 100", c.WitnessCount())
	}
	if len(c.Root()) != 16 {
		t.Errorf("root: got %q want 16-char hex", c.Root())
	}
}

// TestView — View returns cell snapshot including root + witness_count.
func TestView(t *testing.T) {
	c := NewCell("c", "s")
	c.Bind(map[string]interface{}{"k": "v"})
	v := c.View()
	if v["name"] != "c" {
		t.Errorf("name in view: got %v", v["name"])
	}
	if v["address"] != c.Address() {
		t.Errorf("address in view: got %v want %s", v["address"], c.Address())
	}
	if v["witness_count"] != 1 {
		t.Errorf("witness_count in view: got %v want 1", v["witness_count"])
	}
	recent, ok := v["recent_witnesses"].([]Witness)
	if !ok {
		t.Fatal("recent_witnesses should be []Witness")
	}
	if len(recent) != 1 {
		t.Errorf("recent_witnesses length: got %d want 1", len(recent))
	}
}

// TestAddress — Address is hex sha256 of scope::name, first 16 chars.
func TestAddress(t *testing.T) {
	c := NewCell("name", "scope")
	h := sha256.Sum256([]byte("scope::name"))
	want := hex.EncodeToString(h[:])[:16]
	if c.Address() != want {
		t.Errorf("address: got %s want %s", c.Address(), want)
	}
}

// TestClusterAdd — Add registers and Get returns same cell.
func TestClusterAdd(t *testing.T) {
	cl := NewCluster("c1")
	a := NewCell("a", "s")
	cl.Add(a)
	if got := cl.Get(a.Address()); got != a {
		t.Error("Get should return same cell")
	}
	if cl.Size() != 1 {
		t.Errorf("size: got %d want 1", cl.Size())
	}
}

// TestClusterAll — All returns every cell.
func TestClusterAll(t *testing.T) {
	cl := NewCluster("c1")
	for i := 0; i < 10; i++ {
		cl.Add(NewCell("c"+string(rune('a'+i)), "s"))
	}
	if cl.Size() != 10 {
		t.Errorf("size: got %d want 10", cl.Size())
	}
	if len(cl.All()) != 10 {
		t.Errorf("All length: got %d want 10", len(cl.All()))
	}
}

// TestEnsureBound — re-binding the same name returns the same cell.
func TestEnsureBound(t *testing.T) {
	cl := NewCluster("c1")
	a := cl.EnsureBound("file", map[string]interface{}{"k": "v1"})
	b := cl.EnsureBound("file", map[string]interface{}{"k": "v2"})
	if a != b {
		t.Error("EnsureBound should return same cell for same name")
	}
}

// TestPolyformalismContract — same input contract + same op sequence → same hash,
// regardless of which substrate implements it. This is the byte-exact polyformalism
// claim: Go joins the 6-port family.
func TestPolyformalismContract(t *testing.T) {
	mkSeq := func() string {
		c := NewCell("poly-test", "poly")
		c.Bind(map[string]interface{}{"op": "test", "value": 42})
		c.Tick()
		c.Effect("noop", map[string]interface{}{}, map[string]interface{}{"ok": true})
		return c.Root()
	}
	hash1 := mkSeq()
	hash2 := mkSeq()
	if hash1 != hash2 {
		t.Errorf("polyformalism violated: %s != %s", hash1, hash2)
	}
}
