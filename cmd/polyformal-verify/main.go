// Command polyformal-verify lifts a sample cell to all 6 substrates and verifies
// that the output at least *parses* correctly (TS via tsc, Python via ast.parse,
// C via cpp -fsyntax-only, etc.). For now we focus on TS and Python since
// those are the only two substrates with parsers available in the sandbox.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"unstable.build/rune/internal/ir"
)

func main() {
	fmt.Println("rune-quilt · polyformal-verify")
	fmt.Println("==============================")
	fmt.Println()

	// Build a sample cell
	cell := ir.Lower("verify-cell", "verify",
		map[string]interface{}{
			"language": "go",
			"version":  "1.0.0",
		},
		[]ir.CanonOp{
			{Op: ir.Bind, Payload: map[string]interface{}{"contract": map[string]interface{}{"language": "go"}}},
			{Op: ir.Tick},
			{Op: ir.Link, Payload: map[string]interface{}{"to": "5d75268454404d74", "rel": "sibling"}},
			{Op: ir.Effect, Payload: map[string]interface{}{"op": "save"}},
		})

	tmpDir, err := os.MkdirTemp("", "polyformal-verify-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	results := []verifyResult{}

	for _, sub := range ir.AllSubstrates {
		subName := string(sub)
		ext := extFor(sub)
		path := filepath.Join(tmpDir, "out"+ext)
		out, err := ir.Lifts[sub](cell)
		if err != nil {
			results = append(results, verifyResult{subName, err, false})
			continue
		}
		if err := os.WriteFile(path, []byte(out), 0644); err != nil {
			results = append(results, verifyResult{subName, err, false})
			continue
		}
		// Try to verify
		err = verify(sub, path)
		results = append(results, verifyResult{subName, nil, err == nil})
		if err != nil {
			fmt.Printf("  ✗ %s: %v\n", sub, err)
		} else {
			fmt.Printf("  ✓ %s\n", sub)
		}
	}

	fmt.Println()
	fmt.Println("=== RESULTS ===")
	for _, r := range results {
		status := "PASS"
		if !r.ok {
			status = "FAIL"
		}
		fmt.Printf("  %s: %s", r.sub, status)
		if r.err != nil {
			fmt.Printf(" — %v", r.err)
		}
		fmt.Println()
	}

	// Exit code reflects failures
	for _, r := range results {
		if !r.ok {
			os.Exit(1)
		}
	}
}

type verifyResult struct {
	sub string
	err error
	ok  bool
}


func extFor(sub ir.Substrate) string {
	switch sub {
	case ir.TS:
		return ".ts"
	case ir.Python:
		return ".py"
	case ir.C:
		return ".c"
	case ir.Rust:
		return ".rs"
	case ir.GDScript:
		return ".gd"
	case ir.Kernel:
		return ".c"
	}
	return ".txt"
}

func verify(sub ir.Substrate, path string) error {
	switch sub {
	case ir.TS:
		// Use tsc --noEmit to check syntax
		cmd := exec.Command("tsc", "--noEmit", "--strict", "--target", "es2020",
			"--module", "esnext", path)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("tsc: %s", strings.TrimSpace(string(out)))
		}
		return nil
	case ir.Python:
		// Use python3 -c "import ast; ast.parse(open('...').read())"
		cmd := exec.Command("python3", "-c",
			fmt.Sprintf("import ast; ast.parse(open(%q).read()); print('ok')", path))
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("python ast: %s", strings.TrimSpace(string(out)))
		}
		return nil
	case ir.C:
		// Try gcc -fsyntax-only (fall back to no-op if gcc missing)
		if _, err := exec.LookPath("gcc"); err != nil {
			return fmt.Errorf("no gcc in sandbox — skipped")
		}
		cmd := exec.Command("gcc", "-fsyntax-only", "-x", "c", path)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("gcc: %s", strings.TrimSpace(string(out)))
		}
		return nil
	case ir.Rust:
		// Try rustc --emit=metadata (skip if no rustc)
		if _, err := exec.LookPath("rustc"); err != nil {
			return fmt.Errorf("no rustc in sandbox — skipped")
		}
		// rustc needs a lib target; just check parse
		cmd := exec.Command("rustc", "--edition", "2021", "--crate-type", "lib",
			"--emit=metadata", path)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("rustc: %s", strings.TrimSpace(string(out)))
		}
		return nil
	case ir.GDScript:
		// GDScript has no parser available — skip
		return fmt.Errorf("no gdscript parser in sandbox — skipped")
	case ir.Kernel:
		// Same as C
		if _, err := exec.LookPath("gcc"); err != nil {
			return fmt.Errorf("no gcc in sandbox — skipped")
		}
		cmd := exec.Command("gcc", "-fsyntax-only", "-x", "c", "-nostdlib", path)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("gcc: %s", strings.TrimSpace(string(out)))
		}
		return nil
	}
	return fmt.Errorf("unknown substrate")
}
