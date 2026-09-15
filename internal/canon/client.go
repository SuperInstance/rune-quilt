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

// Package canon is the Go client for the Quilt canon — a Cloudflare
// Worker that holds the long-form corpus the cell-router queries against.
//
// Endpoints used:
//   GET  /api/canon/hash
//   GET  /api/canon?n=N
//   POST /api/cell  (submit a new cell)
//   GET  /api/canon/navigate?paper=N&depth=D
//   GET  /api/canon/lineage?id=N
//   GET  /api/vibe?lang=...
package canon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client is a thin wrapper over the canon HTTP API.
type Client struct {
	Base string        // e.g. "https://live-canon.casey-digennaro.workers.dev"
	HTTP *http.Client  // shared HTTP client
}

// New returns a Client with sensible defaults.
func New(base string) *Client {
	return &Client{
		Base: base,
		HTTP: &http.Client{Timeout: 30 * time.Second},
	}
}

// Hash is the live canon's state hash.
type Hash struct {
	StateHash    string `json:"state_hash"`
	PaperCount   int    `json:"paper_count"`
	TestCellHash string `json:"test_cell_hash"`
	CanonTarget  string `json:"canon_target"`
}

// GetHash returns the current canon state hash.
func (c *Client) GetHash(ctx context.Context) (*Hash, error) {
	var h Hash
	if err := c.get(ctx, "/api/canon/hash", &h); err != nil {
		return nil, err
	}
	return &h, nil
}

// Paper is one entry in the canon.
type Paper struct {
	ID     string `json:"id"`
	Number int    `json:"number"`
	Title  string `json:"title"`
	FNum   int    `json:"f_number,omitempty"`
	Phase  int    `json:"phase,omitempty"`
}

// List returns the first n papers in the canon.
func (c *Client) List(ctx context.Context, n int) ([]Paper, error) {
	if n <= 0 {
		n = 50
	}
	u := fmt.Sprintf("%s/api/canon?n=%d", c.Base, n)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("canon: status %d", resp.StatusCode)
	}
	var out struct {
		Papers []Paper `json:"papers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Papers, nil
}

// CellSubmission is the request body for /api/cell.
type CellSubmission struct {
	Dials []int  `json:"dials"`
	Refs  []int  `json:"refs"`
	Title string `json:"title"`
}

// CellResponse is the response from /api/cell.
type CellResponse struct {
	Admitted bool   `json:"admitted"`
	ID       string `json:"id"`
	Title    string `json:"title"`
}

// Submit sends a cell to the canon for admission.
func (c *Client) Submit(ctx context.Context, sub CellSubmission) (*CellResponse, error) {
	body, _ := json.Marshal(sub)
	u := c.Base + "/api/cell"
	req, _ := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("canon submit: status %d", resp.StatusCode)
	}
	var out CellResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// NavigateResult is the response from /api/canon/navigate.
type NavigateResult struct {
	Papers []Paper `json:"papers"`
}

// Navigate returns the neighbors of a paper up to the given depth.
func (c *Client) Navigate(ctx context.Context, paperNum, depth int) (*NavigateResult, error) {
	q := url.Values{}
	q.Set("paper", fmt.Sprintf("%d", paperNum))
	q.Set("depth", fmt.Sprintf("%d", depth))
	u := c.Base + "/api/canon/navigate?" + q.Encode()
	var out NavigateResult
	if err := c.get(ctx, u, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// LineageResult is the response from /api/canon/lineage.
type LineageResult struct {
	Papers []Paper `json:"papers"`
}

// Lineage returns the lineage chain of a cell.
func (c *Client) Lineage(ctx context.Context, id string) (*LineageResult, error) {
	q := url.Values{}
	q.Set("id", id)
	u := c.Base + "/api/canon/lineage?" + q.Encode()
	var out LineageResult
	if err := c.get(ctx, u, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// VibeRequest is the request to /api/vibe.
type VibeRequest struct {
	Lang string
	Hash string
}

// VibeResponse is the response from /api/vibe.
type VibeResponse struct {
	Lang string `json:"lang"`
	Vibe string `json:"vibe"`
}

// Vibe returns the 30-second protocol for a target language.
func (c *Client) Vibe(ctx context.Context, req VibeRequest) (*VibeResponse, error) {
	q := url.Values{}
	q.Set("lang", req.Lang)
	if req.Hash != "" {
		q.Set("hash", req.Hash)
	}
	u := c.Base + "/api/vibe?" + q.Encode()
	var out VibeResponse
	if err := c.get(ctx, u, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) get(ctx context.Context, path string, out interface{}) error {
	var u string
	switch {
	case bytes.HasPrefix([]byte(path), []byte("http")):
		// already a full URL
		u = path
	case path[0] == '/':
		// absolute path — just prepend Base
		u = c.Base + path
	default:
		// relative path — prepend Base + "/"
		u = c.Base + "/" + path
	}
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("canon: GET %s status %d", path, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}
