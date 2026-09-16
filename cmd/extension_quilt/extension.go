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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"

	"unstable.build/rune/internal/a2a"
	"unstable.build/rune/internal/canon"
	"unstable.build/rune/internal/quilt"
	"unstable.build/rune/internal/visual"
)

// QuiltConfig is what the user sets in .rune/config.yaml under extensions.rune-quilt.
type QuiltConfig struct {
	CellID       string   `json:"cell_id"`
	Role         string   `json:"role"`
	Capabilities []string `json:"capabilities"`
	CanonURL     string   `json:"canon_url"`
	A2AURL       string   `json:"a2a_url"`
}

// DefaultConfig is the fallback when the user didn't set anything.
func DefaultConfig() QuiltConfig {
	return QuiltConfig{
		CellID: fmt.Sprintf("rune-quilt-%d", time.Now().UnixNano()),
		Role:   "cell-router",
		Capabilities: []string{
			"file-to-cell",
			"canon-query",
			"canon-shape",
			"witness-log",
			"polyformal-ports",
		},
		CanonURL: "https://live-canon.casey-digennaro.workers.dev",
		A2AURL:   "https://quilt-a2a-v2.casey-digennaro.workers.dev",
	}
}

// quiltExtension implements extensionapi.WorkspaceExtension.
type quiltExtension struct {
	cfg     QuiltConfig
	cluster *quilt.Cluster
	canon   *canon.Client
	a2a     *a2a.Client
	visual  *visual.Server
	mu      sync.Mutex
	cellMap map[string]*quilt.Cell
}

// NewExtension returns the rune-quilt extension and its metadata.
func NewExtension() (extensionapi.WorkspaceExtension, extensionapi.Metadata) {
	cfg := DefaultConfig()
	ext := &quiltExtension{
		cfg:     cfg,
		cluster: quilt.NewCluster("default"),
		canon:   canon.New(cfg.CanonURL),
		a2a:     a2a.New(cfg.A2AURL, cfg.CellID, cfg.Role, cfg.Capabilities),
		visual:  visual.NewServer(quilt.NewCluster("default"), canon.New(cfg.CanonURL), a2a.New(cfg.A2AURL, cfg.CellID, cfg.Role, cfg.Capabilities)),
		cellMap: map[string]*quilt.Cell{},
	}

	metadata := extensionapi.Metadata{
		DeveloperID:      "superinstance",
		DeveloperEmail:   "team@superinstance.dev",
		DeveloperKey:     "rune-quilt",
		ExtensionID:      "rune-quilt",
		ExtensionName:    "Quilt Cell-Router",
		ExtensionVersion: "0.1.0",
		Permissions: extensionapi.NewPermissions(
			extensionapi.PermissionStorage,
			extensionapi.PermissionEditor,
			extensionapi.PermissionFileSystem,
			extensionapi.PermissionCommands,
			extensionapi.PermissionNotifications,
			extensionapi.PermissionLLM,
			extensionapi.PermissionLSP,
		),
	}

	return ext, metadata
}

// cellRouterHandler implements textapi.CommandHandler (HandleCommand + Complete).
type cellRouterHandler struct {
	e *quiltExtension
}

// HandleCommand dispatches all our commands.
func (h *cellRouterHandler) HandleCommand(ctx context.Context, cmd textapi.Command) error {
	switch cmd.Name {
	case "quilt":
		return h.quiltCmd(ctx, cmd)
	case "cell":
		return h.cellCmd(ctx, cmd)
	case "canon":
		return h.canonCmd(ctx, cmd)
	case "peers":
		return h.peersCmd(ctx, cmd)
	case "visual":
		return h.visualCmd(ctx, cmd)
	}
	return fmt.Errorf("unknown command: %s", cmd.Name)
}

// Complete returns command completions (stub — rune will provide default completion).
func (h *cellRouterHandler) Complete(
	ctx context.Context, name string, args []string,
) (iterator.Iterator[string], error) {
	return iterator.FromSlice([]string{}), nil
}

func (h *cellRouterHandler) quiltCmd(ctx context.Context, cmd textapi.Command) error {
	sub := ""
	if len(cmd.Args) > 0 {
		sub = cmd.Args[0]
	}
	var out string
	var err error
	switch sub {
	case "", "stats":
		out = h.e.quiltStats()
	case "view":
		if len(cmd.Args) < 2 {
			return fmt.Errorf("usage: quilt view <uri>")
		}
		out = h.e.quiltView(cmd.Args[1])
	case "peers":
		if len(cmd.Args) < 2 {
			return fmt.Errorf("usage: quilt peers <query>")
		}
		out, err = h.e.quiltPeers(ctx, strings.Join(cmd.Args[1:], " "))
	case "submit":
		out, err = h.e.quiltSubmit(ctx)
	default:
		return fmt.Errorf("unknown subcommand: %s", sub)
	}
	if err != nil {
		return err
	}
	if len(out) > 200 {
		out = out[:200] + "..."
	}
	slog.Info("quilt command result", "sub", sub, "result", out)
	return nil
}

func (h *cellRouterHandler) cellCmd(ctx context.Context, cmd textapi.Command) error {
	uri := cmd.URI.String()
	if uri == "" && len(cmd.Args) > 0 {
		uri = cmd.Args[0]
	}
	if uri == "" {
		return fmt.Errorf("no URI in command")
	}
	out := h.e.quiltView(uri)
	if len(out) > 200 {
		out = out[:200] + "..."
	}
	slog.Info("cell view", "uri", uri, "result", out)
	return nil
}

func (h *cellRouterHandler) canonCmd(ctx context.Context, cmd textapi.Command) error {
	sub := "hash"
	if len(cmd.Args) > 0 {
		sub = cmd.Args[0]
	}
	var out string
	var err error
	switch sub {
	case "hash":
		var h2 *canon.Hash
		h2, err = h.e.canon.GetHash(ctx)
		if err == nil {
			b, _ := json.MarshalIndent(h2, "", "  ")
			out = string(b)
		}
	case "list":
		var papers []canon.Paper
		papers, err = h.e.canon.List(ctx, 10)
		if err == nil {
			b, _ := json.MarshalIndent(papers, "", "  ")
			out = string(b)
		}
	default:
		return fmt.Errorf("unknown subcommand: %s", sub)
	}
	if err != nil {
		return err
	}
	if len(out) > 200 {
		out = out[:200] + "..."
	}
	slog.Info("canon command result", "sub", sub, "result", out)
	return nil
}

func (h *cellRouterHandler) peersCmd(ctx context.Context, cmd textapi.Command) error {
	if len(cmd.Args) == 0 {
		return fmt.Errorf("usage: peers <query>")
	}
	out, err := h.e.quiltPeers(ctx, strings.Join(cmd.Args, " "))
	if err != nil {
		return err
	}
	if len(out) > 200 {
		out = out[:200] + "..."
	}
	slog.Info("peers result", "result", out)
	return nil
}

func (h *cellRouterHandler) visualCmd(ctx context.Context, cmd textapi.Command) error {
	if h.e.visual == nil {
		return fmt.Errorf("visual server not started")
	}
	url := h.e.visual.URL()
	slog.Info("visual dashboard ready",
		"open in browser", url,
		"snapshot JSON", url+"/snapshot",
		"SSE events", url+"/events")
	// Refresh stats once on demand
	go h.e.visual.RefreshStats(ctx)
	return nil
}

// quiltEventHandler implements textapi.EventHandler.
type quiltEventHandler struct {
	e *quiltExtension
}

// Handle dispatches to per-event-type methods.
func (h *quiltEventHandler) Handle(ctx context.Context, ev textapi.Event) bool {
	uri := ev.URI.String()
	switch ev.Type {
	case textapi.EventTypeOpen:
		content := ev.Content
		cell := h.e.ensureCell(uri, content)
		cell.Tick()
		slog.Info("cell opened",
			"uri", uri,
			"address", cell.Address(),
			"witnesses", cell.WitnessCount())
	case textapi.EventTypeChange:
		cell := h.e.getCell(uri)
		if cell != nil {
			cell.Effect("file-change", map[string]interface{}{
				"length": len(ev.Content),
				"ts":     time.Now().UnixMilli(),
			}, nil)
		}
	case textapi.EventTypeFlush:
		content := ev.Content
		cell := h.e.getCell(uri)
		if cell != nil {
			cell.Effect("file-save", map[string]interface{}{
				"length": len(content),
			}, nil)
			go h.e.canonize(ctx, cell, content)
		}
	case textapi.EventTypeClose:
		cell := h.e.getCell(uri)
		if cell != nil {
			cell.Tick()
			slog.Info("cell closed",
				"uri", uri,
				"address", cell.Address(),
				"witnesses", cell.WitnessCount())
		}
	}
	return false // keep receiving events
}

// ExtendWorkspace is the entry point Rune calls.
func (e *quiltExtension) ExtendWorkspace(
	ctx context.Context, w *extensionapi.Workspace, cfg config.Config,
) error {
	// Update cluster name from config
	name := "default"
	if cfg != nil {
		// cfg is config.Config; check if it has Config() method
		// For now, derive from CellID
		name = "workspace-" + e.cfg.CellID
	}
	e.cluster = quilt.NewCluster(name)

	// Update the visual server to share this cluster + canon + a2a client
	e.visual = visual.NewServer(e.cluster, e.canon, e.a2a)

	// Register with a2a Worker
	if res, err := e.a2a.Register(ctx); err != nil {
		slog.Warn("a2a register failed", "error", err)
	} else {
		slog.Info("registered with a2a",
			"cell_id", res.Cell.CellID,
			"total_cells", res.TotalCells)
	}

	// Connect to canon
	if h, err := e.canon.GetHash(ctx); err != nil {
		slog.Warn("canon unreachable", "error", err)
	} else {
		slog.Info("canon connected",
			"state", h.StateHash,
			"papers", h.PaperCount)
	}

	// Register all four commands via the Workspace.RegisterCommand method
	handler := &cellRouterHandler{e: e}
	commands := []textapi.CommandManual{
		{Name: "quilt", Summary: "Quilt cell-router (stats, view, peers, submit)"},
		{Name: "cell", Summary: "Show the current cell"},
		{Name: "canon", Summary: "Query the Quilt canon (hash, list)"},
		{Name: "peers", Summary: "Find cells by capability"},
		{Name: "visual", Summary: "Open the live cell-graph dashboard in browser"},
	}
	for _, manual := range commands {
		if err := w.RegisterCommand(manual, handler); err != nil {
			slog.Warn("register command failed",
				"name", manual.Name, "error", err)
		}
	}

	// Subscribe to file events
	if editor := w.Editor(ctx); editor != nil {
		if err := editor.SubscribeEvents(
			[]textapi.EventType{
				textapi.EventTypeOpen,
				textapi.EventTypeChange,
				textapi.EventTypeFlush,
				textapi.EventTypeClose,
			},
			&quiltEventHandler{e: e},
		); err != nil {
			slog.Warn("subscribe events failed", "error", err)
		}
	}

	// Start the local visual dashboard so the user can see the cell graph
	if err := e.visual.Start(ctx); err != nil {
		slog.Warn("visual server failed to start", "error", err)
	} else {
		slog.Info("visual dashboard live",
			"url", e.visual.URL(),
			"snapshot", e.visual.URL()+"/snapshot",
			"events", e.visual.URL()+"/events")
	}

	// Background: tick the a2a worker every 60s
	go e.tickLoop(ctx)

	return nil
}

// canonize submits the cell content to the live canon worker.
func (e *quiltExtension) canonize(ctx context.Context, cell *quilt.Cell, content string) {
	if cell == nil {
		return
	}
	dials := make([]int, 16)
	for i := 0; i < 16; i++ {
		dials[i] = i * 100
	}
	sub := canon.CellSubmission{
		Dials: dials,
		Refs:  []int{115, 122, 129},
		Title: fmt.Sprintf("%s — %s", cell.Scope, cell.Name),
	}
	if res, err := e.canon.Submit(ctx, sub); err != nil {
		slog.Warn("canon submit failed", "error", err)
		cell.Effect("canon-submit", map[string]interface{}{"error": err.Error()}, nil)
	} else {
		slog.Info("canon submitted", "id", res.ID)
		cell.Effect("canon-submit", map[string]interface{}{"id": res.ID}, nil)
	}
}

// ensureCell returns the cell for the URI, creating it if absent.
func (e *quiltExtension) ensureCell(uri, content string) *quilt.Cell {
	e.mu.Lock()
	defer e.mu.Unlock()
	if c, ok := e.cellMap[uri]; ok {
		return c
	}
	contract := map[string]interface{}{
		"uri":      uri,
		"length":   len(content),
		"language": detectLanguage(uri),
	}
	c := e.cluster.EnsureBound(uri, contract)
	e.cellMap[uri] = c
	return c
}

// getCell looks up a cell without creating.
func (e *quiltExtension) getCell(uri string) *quilt.Cell {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cellMap[uri]
}

// tickLoop ticks the a2a worker every 60s.
func (e *quiltExtension) tickLoop(ctx context.Context) {
	t := time.NewTicker(60 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if res, err := e.a2a.Tick(ctx); err == nil {
				slog.Info("a2a tick",
					"cells", res.TotalCells,
					"inbox", res.InboxCount)
			}
		}
	}
}

// quiltStats returns a quick summary of the cell-router state.
func (e *quiltExtension) quiltStats() string {
	cells := e.cluster.All()
	totalWitnesses := 0
	for _, c := range cells {
		totalWitnesses += c.WitnessCount()
	}
	stats := map[string]interface{}{
		"cluster":         e.cluster.Name,
		"cell_count":      len(cells),
		"total_witnesses": totalWitnesses,
		"a2a_cell_id":     e.cfg.CellID,
		"a2a_role":        e.cfg.Role,
		"canon_url":       e.cfg.CanonURL,
	}
	b, _ := json.MarshalIndent(stats, "", "  ")
	return string(b)
}

// quiltView returns the snapshot of one cell.
func (e *quiltExtension) quiltView(uri string) string {
	e.mu.Lock()
	cell := e.cellMap[uri]
	e.mu.Unlock()
	if cell == nil {
		return `{"err": "no cell for that URI"}`
	}
	v := cell.View()
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

// quiltPeers finds semantically-relevant peer cells via the a2a Worker.
func (e *quiltExtension) quiltPeers(ctx context.Context, query string) (string, error) {
	res, err := e.a2a.FindSemantic(ctx, query, 5)
	if err != nil {
		return "", err
	}
	hits := make([]map[string]interface{}, 0, len(res.Matches))
	for _, m := range res.Matches {
		hits = append(hits, map[string]interface{}{
			"cell_id": m.ID,
			"score":   m.Score,
			"role":    m.Metadata["role"],
		})
	}
	out := map[string]interface{}{
		"query":   query,
		"matches": hits,
		"count":   len(hits),
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	return string(b), nil
}

// quiltSubmit pushes every cell's content to the live canon.
func (e *quiltExtension) quiltSubmit(ctx context.Context) (string, error) {
	e.mu.Lock()
	cells := make([]*quilt.Cell, 0, len(e.cellMap))
	for _, c := range e.cellMap {
		cells = append(cells, c)
	}
	e.mu.Unlock()
	submitted := []string{}
	for _, c := range cells {
		e.canonize(ctx, c, "")
		submitted = append(submitted, c.Address())
	}
	out := map[string]interface{}{
		"submitted_count": len(submitted),
		"addresses":       submitted,
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	return string(b), nil
}

// detectLanguage guesses the programming language from a file URI.
func detectLanguage(uri string) string {
	if strings.HasSuffix(uri, ".go") {
		return "go"
	}
	if strings.HasSuffix(uri, ".py") {
		return "python"
	}
	if strings.HasSuffix(uri, ".rs") {
		return "rust"
	}
	if strings.HasSuffix(uri, ".ts") || strings.HasSuffix(uri, ".tsx") {
		return "typescript"
	}
	if strings.HasSuffix(uri, ".js") || strings.HasSuffix(uri, ".jsx") {
		return "javascript"
	}
	if strings.HasSuffix(uri, ".md") {
		return "markdown"
	}
	return "unknown"
}
