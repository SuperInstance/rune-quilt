// Package visual provides a live HTTP dashboard for the cell graph.
//
// The server emits Server-Sent Events at /events for every witness entry,
// and serves a self-contained vanilla-JS dashboard at / that force-layouts
// the cell graph in real-time.
package visual

import "time"

// Node is a cell in the visual graph.
type Node struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Scope    string   `json:"scope"`
	Role     string   `json:"role"`
	Tier     int      `json:"tier"`
	Language string   `json:"language"`
	Caps     []string `json:"caps"`
}

// Edge is a witness-driven link between two cells.
type Edge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"` // bind, link, effect, view, tick
	Tick   int64  `json:"tick"`
}

// Snapshot is the initial state pushed on connect.
type Snapshot struct {
	Cluster  string    `json:"cluster"`
	Cells    []Node    `json:"cells"`
	Edges    []Edge    `json:"edges"`
	Stats    Stats     `json:"stats"`
	ServerTS time.Time `json:"server_ts"`
}

// Stats is the dashboard header.
type Stats struct {
	CellCount    int    `json:"cell_count"`
	EdgeCount    int    `json:"edge_count"`
	WitnessCount int    `json:"witness_count"`
	Cluster      string `json:"cluster"`
	CanonHash    string `json:"canon_hash"`
	A2ACellID    string `json:"a2a_cell_id"`
	A2AURL       string `json:"a2a_url"`
}

// Event is a Server-Sent Event payload.
type Event struct {
	Type    string    `json:"type"` // "cell-added", "witness", "edge"
	Node    *Node     `json:"node,omitempty"`
	Edge    *Edge     `json:"edge,omitempty"`
	Witness *Witness  `json:"witness,omitempty"`
	Stats   *Stats    `json:"stats,omitempty"`
	TS      time.Time `json:"ts"`
}

// Witness is a single witness entry.
type Witness struct {
	ID    string `json:"id"`
	Op    string `json:"op"`
	Cell  string `json:"cell"`
	Note  string `json:"note"`
	Tick  int64  `json:"tick"`
}
