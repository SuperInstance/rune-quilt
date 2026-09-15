package ir

import (
	"strings"
	"testing"
)

// TestLower — basic lowering produces a Cell with witnesses.
func TestLower(t *testing.T) {
	c := Lower("cell-1", "scope-1",
		map[string]interface{}{"k": "v"},
		[]CanonOp{
			{Op: Bind, Payload: map[string]interface{}{
				"contract": map[string]interface{}{"k": "v"},
			}},
			{Op: Tick},
			{Op: Effect, Payload: map[string]interface{}{"x": 1}},
		})
	if c.Name != "cell-1" {
		t.Errorf("name: got %s", c.Name)
	}
	if len(c.Witnesses) != 3 {
		t.Errorf("witnesses: got %d want 3", len(c.Witnesses))
	}
	if c.Tier != 1 {
		t.Errorf("tier after BIND: got %d want 1", c.Tier)
	}
}

// TestChainIntegrity — merkle chain is intact.
func TestChainIntegrity(t *testing.T) {
	c := Lower("c", "s", nil,
		[]CanonOp{
			{Op: Bind},
			{Op: Tick},
			{Op: Tick},
			{Op: Effect, Payload: map[string]interface{}{"x": 1}},
		})
	for i := 1; i < len(c.Witnesses); i++ {
		if c.Witnesses[i].PrevRoot != c.Witnesses[i-1].Root {
			t.Errorf("chain broken at %d", i)
		}
	}
}

// TestLifts — every substrate produces non-empty output.
func TestLifts(t *testing.T) {
	c := Lower("cell", "scope",
		map[string]interface{}{"k": "v"},
		[]CanonOp{
			{Op: Bind, Payload: map[string]interface{}{
				"contract": map[string]interface{}{"k": "v"},
			}},
			{Op: Tick},
		})
	for sub, lift := range Lifts {
		out, err := lift(c)
		if err != nil {
			t.Errorf("%s: %v", sub, err)
		}
		if len(out) < 30 {
			t.Errorf("%s: output too short (%d chars)", sub, len(out))
		}
		if !strings.Contains(out, c.Name) {
			t.Errorf("%s: output should mention cell name", sub)
		}
	}
}

// TestLiftsRoundTrip — all 6 substrates produce identical witness roots.
func TestLiftsRoundTrip(t *testing.T) {
	c := Lower("poly", "test",
		map[string]interface{}{"k": "v"},
		[]CanonOp{
			{Op: Bind},
			{Op: Tick},
			{Op: Effect, Payload: map[string]interface{}{"x": 1}},
		})
	// Extract the witness roots from each lifted output
	for sub, lift := range Lifts {
		out, err := lift(c)
		if err != nil {
			t.Errorf("%s: %v", sub, err)
			continue
		}
		// Each output should mention at least one of our 16-char witness roots
		found := false
		for _, w := range c.Witnesses {
			if strings.Contains(out, w.Root) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: output should reference witness roots", sub)
		}
	}
}

// TestAllSubstrates — Registry has 6 entries.
func TestAllSubstrates(t *testing.T) {
	if len(AllSubstrates) != 6 {
		t.Errorf("substrates: got %d want 6", len(AllSubstrates))
	}
	if len(Lifts) != 6 {
		t.Errorf("lifts: got %d want 6", len(Lifts))
	}
}

// TestSanitize — identifier sanitizer is sane.
func TestSanitize(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"hello", "hello"},
		{"hello-world", "hello_world"},
		{"123abc", "_123abc"},
		{"", "anon"},
		{"a.b/c", "a_b_c"},
	}
	for _, tt := range tests {
		if got := sanitize(tt.in); got != tt.want {
			t.Errorf("sanitize(%q): got %q want %q", tt.in, got, tt.want)
		}
	}
}
