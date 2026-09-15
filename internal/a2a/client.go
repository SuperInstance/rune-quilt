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

// Package a2a is the Go client for the Quilt a2a-protocol Worker.
//
// a2a (agent-to-agent) is the inter-cell messaging layer. Every rune-quilt
// extension registers itself as a cell on startup, ticks on every operation,
// and queries the Vectorize index to find semantically-relevant peers.
//
// The Worker is deployed at https://quilt-a2a-v2.casey-digennaro.workers.dev/
package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client talks to the a2a Worker over HTTPS.
type Client struct {
	Base   string
	HTTP   *http.Client
	CellID string
	Role   string
	Caps   []string
}

// New returns an a2a Client.
func New(base, cellID, role string, caps []string) *Client {
	return &Client{
		Base:   base,
		HTTP:   &http.Client{Timeout: 30 * time.Second},
		CellID: cellID,
		Role:   role,
		Caps:   caps,
	}
}

// ServiceInfo is the GET / response.
type ServiceInfo struct {
	Service         string `json:"service"`
	Version         string `json:"version"`
	Features        []string `json:"features"`
	Cells           int    `json:"cells"`
	MessagesPending int    `json:"messages_pending"`
	Endpoints       []string `json:"endpoints"`
}

// Info returns the service info.
func (c *Client) Info(ctx context.Context) (*ServiceInfo, error) {
	var out ServiceInfo
	if err := c.get(ctx, "/", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Cell is a registered cell in the fleet.
type Cell struct {
	CellID       string   `json:"cell_id"`
	Role         string   `json:"role"`
	Capabilities []string `json:"capabilities"`
	ParentID     string   `json:"parent_id,omitempty"`
	Lineage      []string `json:"lineage,omitempty"`
	Load         float64  `json:"load"`
	LastSeen     int64    `json:"last_seen"`
}

// RegisterResult is the response from POST /register.
type RegisterResult struct {
	OK         bool   `json:"ok"`
	Cell       *Cell  `json:"cell"`
	TotalCells int    `json:"total_cells"`
}

// Register sends a registration to the a2a worker. The cell is auto-embedded
// into Vectorize so semantic search can find it.
func (c *Client) Register(ctx context.Context) (*RegisterResult, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"cell_id":      c.CellID,
		"role":         c.Role,
		"address":      c.CellID,
		"capabilities": c.Caps,
	})
	var out RegisterResult
	if err := c.post(ctx, "/register", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// TickResult is the response from POST /tick.
type TickResult struct {
	OK          bool        `json:"ok"`
	CellID      string      `json:"cell_id"`
	TotalCells  int         `json:"total_cells"`
	InboxCount  int         `json:"inbox_count"`
	Messages    []Message   `json:"messages"`
}

// Message is an a2a message.
type Message struct {
	ID        string                 `json:"id"`
	From      string                 `json:"from"`
	To        string                 `json:"to"`
	Type      string                 `json:"type"`
	Payload   map[string]interface{} `json:"payload"`
	Ts        int64                  `json:"ts"`
	Broadcast bool                   `json:"broadcast,omitempty"`
}

// Tick sends a heartbeat + drains the inbox.
func (c *Client) Tick(ctx context.Context) (*TickResult, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"cell_id":      c.CellID,
		"role":         c.Role,
		"capabilities": c.Caps,
	})
	var out TickResult
	if err := c.post(ctx, "/tick", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// FindSemantic is the response from GET /find-semantic.
type FindSemanticResult struct {
	Query   string         `json:"query"`
	K       int            `json:"k"`
	Matches []SemanticHit  `json:"matches"`
	Count   int            `json:"count"`
}

// SemanticHit is one match from Vectorize.
type SemanticHit struct {
	ID       string                 `json:"id"`
	Score    float64                `json:"score"`
	Metadata map[string]interface{} `json:"metadata"`
}

// FindSemantic does semantic search over the cell fleet.
func (c *Client) FindSemantic(ctx context.Context, query string, k int) (*FindSemanticResult, error) {
	if k <= 0 {
		k = 5
	}
	path := fmt.Sprintf("/find-semantic?q=%s&k=%d", urlEncode(query), k)
	var out FindSemanticResult
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SendResult is the response from POST /send.
type SendResult struct {
	OK        bool    `json:"ok"`
	Msg       Message `json:"msg"`
	InboxSize int     `json:"inbox_size"`
}

// Send sends a unicast message to another cell.
func (c *Client) Send(ctx context.Context, to, msgType string, payload map[string]interface{}) (*SendResult, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"from":    c.CellID,
		"to":      to,
		"type":    msgType,
		"payload": payload,
	})
	var out SendResult
	if err := c.post(ctx, "/send", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// BroadcastResult is the response from POST /broadcast.
type BroadcastResult struct {
	OK       bool      `json:"ok"`
	Delivered int      `json:"delivered"`
	Msgs     []Message `json:"msgs"`
}

// Broadcast sends to all cells with the given capability.
func (c *Client) Broadcast(ctx context.Context, msgType string, payload map[string]interface{}, capability string) (*BroadcastResult, error) {
	body := map[string]interface{}{
		"from":    c.CellID,
		"type":    msgType,
		"payload": payload,
	}
	if capability != "" {
		body["capability"] = capability
	}
	b, _ := json.Marshal(body)
	var out BroadcastResult
	if err := c.post(ctx, "/broadcast", b, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Mitosis spawns a child cell inheriting from a parent.
func (c *Client) Mitosis(ctx context.Context, parentID string) (*RegisterResult, error) {
	body, _ := json.Marshal(map[string]interface{}{"parent_id": parentID})
	var out RegisterResult
	if err := c.post(ctx, "/mitosis", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// HeritageResult is the response from GET /heritage.
type HeritageResult struct {
	CellID  string `json:"cell_id"`
	Parent  *Cell  `json:"parent"`
	Lineage []Cell `json:"lineage"`
}

// Heritage returns the lineage chain.
func (c *Client) Heritage(ctx context.Context, cellID string) (*HeritageResult, error) {
	path := "/heritage?cell_id=" + urlEncode(cellID)
	var out HeritageResult
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) get(ctx context.Context, path string, out interface{}) error {
	u := c.Base + path
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("a2a GET %s: status %d: %s", path, resp.StatusCode, string(body))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) post(ctx context.Context, path string, body []byte, out interface{}) error {
	u := c.Base + path
	req, _ := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("a2a POST %s: status %d: %s", path, resp.StatusCode, string(b))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func urlEncode(s string) string {
	// Simple URL-encoding for path values
	out := make([]byte, 0, len(s)*3)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ' ':
			out = append(out, '+')
		case (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9'):
			out = append(out, c)
		default:
			out = append(out, '%', hexDigit(c>>4), hexDigit(c&0xf))
		}
	}
	return string(out)
}

func hexDigit(b byte) byte {
	if b < 10 {
		return '0' + b
	}
	return 'A' + b - 10
}

// ── v3: workspace federation ─────────────────────────────────────

// Workspace is a single workspace entry returned by /workspaces.
type Workspace struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// WorkspacesResult is the response from GET /workspaces.
type WorkspacesResult struct {
	OK             bool        `json:"ok"`
	WorkspaceCount int         `json:"workspace_count"`
	TotalCells     int         `json:"total_cells"`
	Workspaces     []Workspace `json:"workspaces"`
}

// Workspaces lists all workspaces + their cell counts.
func (c *Client) Workspaces(ctx context.Context) (*WorkspacesResult, error) {
	var out WorkspacesResult
	if err := c.get(ctx, "/workspaces", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PeersNearResult is the response from GET /peers/near.
type PeersNearResult struct {
	OK        bool   `json:"ok"`
	CellID    string `json:"cell_id"`
	Workspace string `json:"workspace"`
	PeerCount int    `json:"peer_count"`
	Peers     []Cell `json:"peers"`
}

// PeersNear finds cells in the same workspace as the given cell.
func (c *Client) PeersNear(ctx context.Context, cellID string) (*PeersNearResult, error) {
	path := "/peers/near?cell_id=" + urlEncode(cellID)
	var out PeersNearResult
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// BroadcastEditResult is the response from POST /broadcast-edit.
type BroadcastEditResult struct {
	OK           bool     `json:"ok"`
	Workspace    string   `json:"workspace"`
	Recipients   int      `json:"recipients"`
	RecipientIDs []string `json:"recipient_ids"`
}

// BroadcastEdit broadcasts a file-edit notification to all peers in the same workspace.
func (c *Client) BroadcastEdit(ctx context.Context, workspace, file, op string) (*BroadcastEditResult, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"from":      c.CellID,
		"workspace": workspace,
		"file":      file,
		"op":        op,
	})
	var out BroadcastEditResult
	if err := c.post(ctx, "/broadcast-edit", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
