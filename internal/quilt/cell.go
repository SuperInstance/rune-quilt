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

// Package quilt implements the Quilt cell-fabric runtime as a Rune extension.
//
// The Quilt is a 4D cell graph where every node is a *cell* with a name,
// scope, contract, and witness. Five opcodes (BIND, LINK, EFFECT, VIEW, TICK)
// are the engine. In rune-quilt, every file the user opens becomes a cell
// automatically; every command they run becomes a witness; every project
// becomes a cell cluster.
//
// This package is the Go port. The byte-exact reference is the Python
// implementation in SuperInstance/quilt-python (the 7σ polyformalism port).
package quilt

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// Cell is the irreducible unit of the Quilt. A cell has a name, a scope,
// a contract (the interface it exposes), a value (its current state),
// and a witness log (the audit trail of how it got there).
//
// In rune-quilt, a Cell is typically backed by a file: the file path is
// the cell's name, the project is the scope, and the file's symbols form
// the contract. But cells can also be ephemeral — conversations,
// queries, agent runs — anything worth remembering.
type Cell struct {
	Name     string                 `json:"name"`
	Scope    string                 `json:"scope"`
	Contract map[string]interface{} `json:"contract,omitempty"`
	Value    map[string]interface{} `json:"value,omitempty"`
	Tier     int                    `json:"tier"`     // 0=raw, 1=processed, 2=cited
	Scope_lv int                    `json:"scope_lv"` // 0=local, 1=project, 2=canon

	// Witness — every action recorded as a merkle-rooted log
	witnesses []Witness
	mu        sync.RWMutex
}

// Witness is one entry in the cell's merkle-rooted log.
type Witness struct {
	Op        string                 `json:"op"`
	Payload   map[string]interface{} `json:"payload"`
	Result    map[string]interface{} `json:"result,omitempty"`
	PrevRoot  string                 `json:"prev_root"`
	EntryHash string                 `json:"entry_hash"`
	Root      string                 `json:"root"`
	Ts        int64                  `json:"ts"`
}

// NewCell returns a fresh cell with the given name and scope.
func NewCell(name, scope string) *Cell {
	return &Cell{
		Name:     name,
		Scope:    scope,
		Contract: map[string]interface{}{},
		Value:    map[string]interface{}{},
		Tier:     0,
		Scope_lv: 1,
		witnesses: []Witness{},
	}
}

// Address is the cell's stable identifier — a SHA-256 of name + scope.
// Address is what gets stored in cells and used as the cell's identity
// across the mesh. Cells with the same name + scope have the same address.
func (c *Cell) Address() string {
	h := sha256.Sum256([]byte(c.Scope + "::" + c.Name))
	return hex.EncodeToString(h[:])[:16]
}

// Bind records the cell's initial contract. BIND is the first opcode
// and is what makes the cell real. Without a BIND, a cell is just a name.
func (c *Cell) Bind(contract map[string]interface{}) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Contract = contract
	c.Tier = 1
	w := c.appendWitnessLocked("BIND", map[string]interface{}{
		"contract_size": len(contract),
		"address":       c.Address(),
	}, nil)
	return w.EntryHash
}

// Link connects this cell to another by name. LINKs are the cell graph.
// A cell with no LINKs is isolated; a cell with many LINKs is a hub.
func (c *Cell) Link(other *Cell, rel string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if rel == "" {
		rel = "refs"
	}
	if c.Value["links"] == nil {
		c.Value["links"] = []interface{}{}
	}
	links, _ := c.Value["links"].([]interface{})
	c.Value["links"] = append(links, map[string]interface{}{
		"to":      other.Address(),
		"name":    other.Name,
		"rel":     rel,
	})
	w := c.appendWitnessLocked("LINK", map[string]interface{}{
		"to":  other.Address(),
		"rel": rel,
	}, nil)
	return w.EntryHash
}

// Effect runs an operation that changes the cell's state. EFFECT is the
// verb. EFFECTs are recorded in the witness log and produce a new merkle root.
func (c *Cell) Effect(op string, payload, result map[string]interface{}) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	w := c.appendWitnessLocked("EFFECT", map[string]interface{}{
		"op":     op,
		"params": payload,
	}, result)
	return w.EntryHash
}

// View returns a snapshot of the cell's current state. VIEW is the noun.
// VIEW does not modify state — it gathers the value + last N witnesses.
func (c *Cell) View() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// Last 10 witnesses
	n := 10
	if len(c.witnesses) < n {
		n = len(c.witnesses)
	}
	recent := make([]Witness, n)
	copy(recent, c.witnesses[len(c.witnesses)-n:])
	return map[string]interface{}{
		"name":     c.Name,
		"scope":    c.Scope,
		"address":  c.Address(),
		"tier":     c.Tier,
		"contract": c.Contract,
		"value":    c.Value,
		"witness_count": len(c.witnesses),
		"recent_witnesses": recent,
	}
}

// Tick advances the cell's logical clock. TICK is the substrate's heartbeat.
// In rune-quilt, TICK runs on every file event (open, save, close).
func (c *Cell) Tick() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	w := c.appendWitnessLocked("TICK", map[string]interface{}{
		"ts": time.Now().UnixMilli(),
	}, nil)
	return w.EntryHash
}

// Root returns the current merkle root of the witness chain.
func (c *Cell) Root() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.witnesses) == 0 {
		return ""
	}
	return c.witnesses[len(c.witnesses)-1].Root
}

// WitnessCount returns the number of witnesses recorded.
func (c *Cell) WitnessCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.witnesses)
}

// Witnesses returns a copy of all witnesses.
func (c *Cell) Witnesses() []Witness {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]Witness, len(c.witnesses))
	copy(out, c.witnesses)
	return out
}

// appendWitnessLocked is the internal helper. MUST be called with mu held.
// Computes the new merkle root by hashing prev + entry.
func (c *Cell) appendWitnessLocked(op string, payload, result map[string]interface{}) Witness {
	prevRoot := ""
	if len(c.witnesses) > 0 {
		prevRoot = c.witnesses[len(c.witnesses)-1].Root
	}
	ts := time.Now().UnixMilli()
	entry := map[string]interface{}{
		"op":      op,
		"ts":      ts,
		"payload": payload,
		"result":  result,
		"prev":    prevRoot,
	}
	entryHash := hashEntry(entry)
	root := hashEntry(map[string]interface{}{
		"prev":  prevRoot,
		"entry": entryHash,
	})
	w := Witness{
		Op:        op,
		Payload:   payload,
		Result:    result,
		PrevRoot:  prevRoot,
		EntryHash: entryHash,
		Root:      root,
		Ts:        ts,
	}
	c.witnesses = append(c.witnesses, w)
	return w
}

// hashEntry serializes a map and returns a hex SHA-256.
func hashEntry(m map[string]interface{}) string {
	// Deterministic serialization: sorted keys
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Sort
	for i := 0; i < len(keys); i++ {
		for j := i+1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	var buf []byte
	for _, k := range keys {
		buf = append(buf, []byte(k)...)
		buf = append(buf, []byte(fmt.Sprintf("=%v;", m[k]))...)
	}
	h := sha256.Sum256(buf)
	return hex.EncodeToString(h[:])[:16]
}

// Cluster is a group of related cells. The cell-router maintains a cluster
// per workspace.
type Cluster struct {
	Name  string
	Cells map[string]*Cell // address -> cell
	mu    sync.RWMutex
}

// NewCluster returns an empty cluster.
func NewCluster(name string) *Cluster {
	return &Cluster{
		Name:  name,
		Cells: map[string]*Cell{},
	}
}

// Add inserts (or returns) a cell in the cluster.
func (cl *Cluster) Add(c *Cell) *Cell {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	if existing, ok := cl.Cells[c.Address()]; ok {
		return existing
	}
	cl.Cells[c.Address()] = c
	return c
}

// Get returns a cell by address, or nil.
func (cl *Cluster) Get(address string) *Cell {
	cl.mu.RLock()
	defer cl.mu.RUnlock()
	return cl.Cells[address]
}

// All returns all cells in the cluster.
func (cl *Cluster) All() []*Cell {
	cl.mu.RLock()
	defer cl.mu.RUnlock()
	out := make([]*Cell, 0, len(cl.Cells))
	for _, c := range cl.Cells {
		out = append(out, c)
	}
	return out
}

// Size returns the number of cells in the cluster.
func (cl *Cluster) Size() int {
	cl.mu.RLock()
	defer cl.mu.RUnlock()
	return len(cl.Cells)
}

// EnsureBound returns the cell, binding it if it's new.
func (cl *Cluster) EnsureBound(name string, contract map[string]interface{}) *Cell {
	cell := NewCell(name, cl.Name)
	if existing := cl.Get(cell.Address()); existing != nil {
		return existing
	}
	cell.Bind(contract)
	cl.Add(cell)
	return cell
}

// _ ensures ctx is used (placeholder for future request handling).
var _ = context.Background
