// Command demo-polyformal demonstrates the polyformal compiler.
//
// It lowers a sample canon cell into IR, then lifts to all 6 substrate ports,
// printing each one.
package main

import (
	"fmt"
	"time"

	"unstable.build/rune/internal/ir"
)

func main() {
	fmt.Println("rune-quilt · polyformal compiler demo")
	fmt.Println("====================================")
	fmt.Println()

	// Build a sample canon cell
	cell := ir.Lower("hello-cell", "demo",
		map[string]interface{}{
			"language": "go",
			"version":  "1.0.0",
			"purpose":  "demonstrate polyformalism",
		},
		[]ir.CanonOp{
			{Op: ir.Bind, Payload: map[string]interface{}{
				"contract": map[string]interface{}{
					"language": "go",
					"version":  "1.0.0",
				},
			}},
			{Op: ir.Link, Payload: map[string]interface{}{
				"to":  "5d75268454404d74",
				"rel": "sibling",
			}},
			{Op: ir.Tick},
			{Op: ir.Effect, Payload: map[string]interface{}{
				"op":     "file-save",
				"params": map[string]interface{}{"length": 100},
			}},
			{Op: ir.Tick},
			{Op: ir.View},
		})

	fmt.Printf("Source IR cell:\n")
	fmt.Printf("  name: %s\n", cell.Name)
	fmt.Printf("  scope: %s\n", cell.Scope)
	fmt.Printf("  witnesses: %d\n", len(cell.Witnesses))
	fmt.Printf("  final root: %s\n", cell.Witnesses[len(cell.Witnesses)-1].Root)
	fmt.Println()

	// Lift to every substrate
	fmt.Println("Lifted outputs:")
	fmt.Println()
	for _, sub := range ir.AllSubstrates {
		start := time.Now()
		out, err := ir.Lifts[sub](cell)
		elapsed := time.Since(start)
		if err != nil {
			fmt.Printf("  %s: ERROR %v\n", sub, err)
			continue
		}
		fmt.Printf("─── %s (%s, %d bytes) ───\n", sub, elapsed, len(out))
		// Print first 12 lines
		lines := 0
		for _, line := range splitLines(out) {
			fmt.Println(line)
			lines++
			if lines >= 12 {
				if countLines(out) > 12 {
					fmt.Printf("... +%d more lines\n", countLines(out)-12)
				}
				break
			}
		}
		fmt.Println()
	}
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i, r := range s {
		if r == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func countLines(s string) int {
	n := 1
	for _, r := range s {
		if r == '\n' {
			n++
		}
	}
	return n
}
