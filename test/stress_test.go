package test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"unstable.build/rune/internal/quilt"
)

// TestStress10K — 10,000 cells in a single cluster, witness log persistence.
func TestStress10K(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	cl := quilt.NewCluster("stress")
	start := time.Now()
	for i := 0; i < 10000; i++ {
		c := cl.EnsureBound(fmt.Sprintf("cell-%d", i),
			map[string]interface{}{"i": i, "v": "stress"})
		c.Tick()
	}
	elapsed := time.Since(start)
	if cl.Size() != 10000 {
		t.Errorf("size: got %d want 10000", cl.Size())
	}
	t.Logf("10k cells in %s (%.0f cells/sec)",
		elapsed, float64(cl.Size())/elapsed.Seconds())
}

// TestStressConcurrent — 1000 concurrent ticks on a single cell.
func TestStressConcurrent(t *testing.T) {
	c := quilt.NewCell("stress", "concurrent")
	c.Bind(map[string]interface{}{"x": 1})

	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Tick()
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)
	if c.WitnessCount() != 1001 { // 1 BIND + 1000 TICKs
		t.Errorf("witnesses: got %d want 1001", c.WitnessCount())
	}
	t.Logf("1000 concurrent ticks in %s", elapsed)
}

// TestStressConcurrent10000 — 10000 concurrent ops across 100 cells.
func TestStressConcurrent10000(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	cl := quilt.NewCluster("mass")
	cells := make([]*quilt.Cell, 100)
	for i := range cells {
		cells[i] = cl.EnsureBound(
			fmt.Sprintf("mass-%d", i),
			map[string]interface{}{"v": 1})
	}
	var wg sync.WaitGroup
	start := time.Now()
	for round := 0; round < 100; round++ {
		for _, c := range cells {
			wg.Add(1)
			go func(c *quilt.Cell) {
				defer wg.Done()
				c.Tick()
			}(c)
		}
	}
	wg.Wait()
	elapsed := time.Since(start)
	total := 0
	for _, c := range cells {
		total += c.WitnessCount()
	}
	t.Logf("10k concurrent ticks on 100 cells in %s (%d total witnesses)",
		elapsed, total)
	if total < 10000 {
		t.Errorf("total witnesses: got %d want >= 10000", total)
	}
}

// TestWitnessLogPersistence — witness log merkle chain is intact.
//
// Note: Root is timestamp-dependent, so two cells ticked at different times
// get different roots. We test the *chain integrity* instead of equality.
func TestWitnessLogPersistence(t *testing.T) {
	c := quilt.NewCell("persist", "test")
	for i := 0; i < 50; i++ {
		c.Tick()
	}
	witnesses := c.Witnesses()
	if len(witnesses) != 50 {
		t.Fatalf("witnesses: got %d want 50", len(witnesses))
	}
	// Check that the merkle chain is intact
	for i := 1; i < len(witnesses); i++ {
		if witnesses[i].PrevRoot != witnesses[i-1].Root {
			t.Errorf("chain broken at %d: prev=%s prior=%s",
				i, witnesses[i].PrevRoot, witnesses[i-1].Root)
		}
	}
	// Check that witnesses are returned in order
	for i := 1; i < len(witnesses); i++ {
		if witnesses[i].Ts < witnesses[i-1].Ts {
			t.Errorf("witness timestamps out of order at %d", i)
		}
	}
	// Check the chain forms a single DAG (each witness has unique entry hash)
	seen := map[string]bool{}
	for _, w := range witnesses {
		if seen[w.EntryHash] {
			t.Errorf("duplicate entry hash: %s", w.EntryHash)
		}
		seen[w.EntryHash] = true
	}
}
