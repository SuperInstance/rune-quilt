// Command demo-visual walks a directory, creates cells per file, and serves
// the live visual dashboard so you can see the cell graph in real time.
//

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"unstable.build/rune/internal/a2a"
	"unstable.build/rune/internal/canon"
	"unstable.build/rune/internal/quilt"
	"unstable.build/rune/internal/visual"
)

func main() {
	dir := "./"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	cellID := fmt.Sprintf("visual-demo-%d", time.Now().UnixNano())

	cluster := quilt.NewCluster(filepath.Base(dir))
	c := canon.New("https://live-canon.casey-digennaro.workers.dev")
	a := a2a.New("https://quilt-a2a-v2.casey-digennaro.workers.dev",
		cellID, "visual-demo", []string{"demo", "visual"})

	// Start visual server
	srv := visual.NewServer(cluster, c, a)
	if err := srv.Start(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "start visual: %v\n", err)
		os.Exit(1)
	}
	defer srv.Stop(context.Background())

	fmt.Printf("rune-quilt visual demo · %s\n", dir)
	fmt.Printf("  cell_id:   %s\n", cellID)
	fmt.Printf("  dashboard: %s\n", srv.URL())
	fmt.Printf("  walking...\n")

	// Walk the directory
	count := 0
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) == "" {
			return nil
		}
		// Skip hidden / vendor / large
		if len(path) > 200 {
			return nil
		}
		relPath, _ := filepath.Rel(dir, path)
		contract := map[string]interface{}{
			"path": relPath,
			"ext":  filepath.Ext(path),
			"size": info.Size(),
		}
		cell := cluster.EnsureBound(relPath, contract)
		cell.Tick()
		srv.NotifyCell(cell)
		for _, w := range cell.Witnesses() {
			srv.NotifyWitness(cell, w)
		}
		count++
		if count%50 == 0 {
			fmt.Printf("  cells: %d\n", count)
		}
		return nil
	})

	fmt.Printf("\n  total: %d cells\n", count)
	fmt.Printf("  dashboard live: %s\n", srv.URL())
	fmt.Printf("  press Ctrl-C to quit\n")

	// Keep alive
	stop := make(chan os.Signal, 1)
	_ = stop
	for {
		time.Sleep(1 * time.Second)
	}
	_ = slog.Default()
}
