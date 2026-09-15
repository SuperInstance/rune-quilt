// Copyright (C) 2017-2026 The Rune Authors
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

// Command demo runs the rune-quilt cell-router standalone — without Rune.
//
// It demonstrates the full pipeline:
//   1. Walk a directory, treating every file as a cell
//   2. BIND each cell with its file metadata as the contract
//   3. LINK cells that reference each other
//   4. EFFECT a "canonize" witness on save
//   5. TICK every cell at the end
//   6. Register with the a2a Worker
//   7. Query the canon for related pieces
//   8. Find peers semantically
//
// This is what Rune sees when the extension runs inside it. Without Rune
// we still get to see the full data flow end-to-end.
//
// Usage:
//   go run ./demo/ ./demo/sample-code/
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"unstable.build/rune/internal/a2a"
	"unstable.build/rune/internal/canon"
	"unstable.build/rune/internal/quilt"
)

func main() {
	var (
		dir       = flag.String("dir", "./", "directory to walk")
		canonURL  = flag.String("canon", "https://live-canon.casey-digennaro.workers.dev", "canon worker URL")
		a2aURL    = flag.String("a2a", "https://quilt-a2a-v2.casey-digennaro.workers.dev", "a2a worker URL")
		submit    = flag.Bool("submit", false, "submit cells to live canon")
		cellID    = flag.String("cell-id", fmt.Sprintf("rune-quilt-demo-%d", time.Now().UnixNano()), "this cell's ID")
		query     = flag.String("query", "", "optional semantic query to find peer cells")
	)
	flag.Parse()

	slog.Info("rune-quilt demo starting",
		"dir", *dir, "cell_id", *cellID)

	cluster := quilt.NewCluster(*dir)
	canonClient := canon.New(*canonURL)
	a2aClient := a2a.New(*a2aURL, *cellID, "demo", []string{
		"file-to-cell", "canon-query", "demo",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 1. Register with a2a
	if res, err := a2aClient.Register(ctx); err != nil {
		slog.Warn("a2a register failed (continuing)", "error", err)
	} else {
		slog.Info("a2a registered", "total_cells", res.TotalCells)
	}

	// 2. Walk the directory, creating a cell for every file
	cells := walkAndBind(ctx, cluster, *dir)
	slog.Info("cells created", "count", len(cells))

	// 3. LINK cells that share a parent directory
	linkByDirectory(cluster)
	slog.Info("links created")

	// 4. EFFECT a "demo-canonize" witness on every cell
	for _, c := range cells {
		c.Effect("demo-canonize", map[string]interface{}{
			"demo":       true,
			"witnesses":  c.WitnessCount(),
		}, nil)
	}

	// 5. TICK every cell
	for _, c := range cells {
		c.Tick()
	}

	// 6. Submit to live canon (if asked)
	if *submit {
		for _, c := range cells {
			dials := make([]int, 16)
			for i := 0; i < 16; i++ {
				dials[i] = i * 100
			}
			sub := canon.CellSubmission{
				Dials: dials,
				Refs:  []int{115, 122, 129},
				Title: fmt.Sprintf("%s — %s", c.Scope, c.Name),
			}
			if res, err := canonClient.Submit(ctx, sub); err != nil {
				slog.Warn("canon submit failed", "name", c.Name, "error", err)
			} else {
				slog.Info("canon submitted", "id", res.ID)
			}
		}
	}

	// 7. Summary
	fmt.Println()
	fmt.Println("============================================================")
	fmt.Printf("  rune-quilt demo · cluster=%s\n", cluster.Name)
	fmt.Println("============================================================")
	fmt.Printf("  cells: %d\n", len(cells))
	totalWitnesses := 0
	for _, c := range cells {
		totalWitnesses += c.WitnessCount()
	}
	fmt.Printf("  witnesses: %d\n", totalWitnesses)

	// Print first 5 cells
	fmt.Println()
	fmt.Println("  First 5 cells:")
	for i, c := range cells {
		if i >= 5 {
			break
		}
		v := c.View()
		fmt.Printf("    [%d] %s\n", i, c.Address())
		fmt.Printf("        name:    %s\n", c.Name)
		fmt.Printf("        scope:   %s\n", c.Scope)
		fmt.Printf("        tier:    %d\n", v["tier"])
		fmt.Printf("        root:    %s\n", c.Root())
		fmt.Printf("        witnesses: %d\n", v["witness_count"])
	}

	// 8. Semantic peer search (if asked)
	if *query != "" {
		fmt.Println()
		fmt.Printf("  Finding peers for: '%s'\n", *query)
		res, err := a2aClient.FindSemantic(ctx, *query, 5)
		if err != nil {
			slog.Warn("find-semantic failed", "error", err)
		} else {
			for i, m := range res.Matches {
				fmt.Printf("    [%d] %s (score=%.3f)\n", i, m.ID, m.Score)
			}
		}
	}

	fmt.Println()
	fmt.Println("  Live state:")
	if h, err := canonClient.GetHash(ctx); err == nil {
		fmt.Printf("    canon:    state=%s papers=%d\n", h.StateHash, h.PaperCount)
	} else {
		fmt.Printf("    canon:    unreachable: %v\n", err)
	}
	if info, err := a2aClient.Info(ctx); err == nil {
		fmt.Printf("    a2a:      cells=%d pending=%d\n", info.Cells, info.MessagesPending)
	} else {
		fmt.Printf("    a2a:      unreachable: %v\n", err)
	}
}

// walkAndBind walks a directory, creating a Quilt cell for every file.
// Skips common noise: .git, node_modules, __pycache__, .DS_Store.
func walkAndBind(ctx context.Context, cluster *quilt.Cluster, dir string) []*quilt.Cell {
	skip := map[string]bool{
		".git":         true,
		"node_modules": true,
		"__pycache__":  true,
		".DS_Store":    true,
		"target":       true,
		"bin":          true,
	}
	var cells []*quilt.Cell
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if skip[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if skip[info.Name()] {
			return nil
		}
		// Only bind code-ish files
		ext := filepath.Ext(path)
		if !isCodeExt(ext) {
			return nil
		}
		// Read first 200 chars for the contract
		content, _ := os.ReadFile(path)
		preview := string(content)
		if len(preview) > 200 {
			preview = preview[:200]
		}
		rel, _ := filepath.Rel(dir, path)
		contract := map[string]interface{}{
			"uri":      path,
			"rel":      rel,
			"size":     info.Size(),
			"language": detectLangFromExt(ext),
			"preview":  preview,
		}
		c := cluster.EnsureBound(path, contract)
		cells = append(cells, c)
		return nil
	})
	return cells
}

// linkByDirectory links cells that share a directory.
func linkByDirectory(cluster *quilt.Cluster) {
	byDir := map[string][]*quilt.Cell{}
	for _, c := range cluster.All() {
		dir := filepath.Dir(c.Name)
		byDir[dir] = append(byDir[dir], c)
	}
	for _, list := range byDir {
		if len(list) < 2 {
			continue
		}
		// LINK every cell to the first one in the directory
		first := list[0]
		for _, other := range list[1:] {
			first.Link(other, "sibling")
			other.Link(first, "sibling")
		}
	}
}

func isCodeExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".go", ".py", ".rs", ".ts", ".tsx", ".js", ".jsx",
		".c", ".h", ".cpp", ".hpp", ".java", ".kt", ".swift",
		".v", ".sv", ".vhd", ".vhdl", ".zig", ".mojo", ".lua",
		".rb", ".php", ".scala", ".clj", ".hs", ".ml",
		".md", ".txt", ".yaml", ".yml", ".json", ".toml":
		return true
	}
	return false
}

func detectLangFromExt(ext string) string {
	m := map[string]string{
		".go": "go", ".py": "python", ".rs": "rust",
		".ts": "typescript", ".tsx": "typescript", ".js": "javascript", ".jsx": "javascript",
		".c": "c", ".h": "c", ".cpp": "cpp", ".hpp": "cpp",
		".java": "java", ".kt": "kotlin", ".swift": "swift",
		".v": "verilog", ".sv": "systemverilog", ".vhd": "vhdl", ".vhdl": "vhdl",
		".zig": "zig", ".mojo": "mojo", ".lua": "lua",
		".rb": "ruby", ".php": "php", ".scala": "scala", ".clj": "clojure",
		".hs": "haskell", ".ml": "ocaml",
		".md": "markdown", ".txt": "text", ".yaml": "yaml", ".yml": "yaml",
		".json": "json", ".toml": "toml",
	}
	return m[strings.ToLower(ext)]
}

// silence unused import in some builds
var _ = json.Marshal
