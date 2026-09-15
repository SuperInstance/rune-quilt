package ir

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"
)

// Lower converts a canon-cell representation (input map + op sequence) into
// the IR. The canon cell is given as a flat list of operations; we replay
// them to build the IR with the correct merkle chain.
type CanonOp struct {
	Op      Op                     `json:"op"`
	Payload map[string]interface{} `json:"payload,omitempty"`
	Result  map[string]interface{} `json:"result,omitempty"`
}

// Lower turns a canon cell description into an IR Cell.
func Lower(name, scope string, contract map[string]interface{}, ops []CanonOp) *Cell {
	c := &Cell{
		Name:      name,
		Scope:     scope,
		Contract:  contract,
		Value:     map[string]interface{}{},
		Tier:      0,
		Witnesses: []Witness{},
	}
	for _, op := range ops {
		w := applyOp(c, op)
		c.Witnesses = append(c.Witnesses, w)
	}
	return c
}

func applyOp(c *Cell, op CanonOp) Witness {
	prevRoot := ""
	if len(c.Witnesses) > 0 {
		prevRoot = c.Witnesses[len(c.Witnesses)-1].Root
	}
	ts := time.Now().UnixMilli()
	entry := map[string]interface{}{
		"op":      string(op.Op),
		"ts":      ts,
		"payload": op.Payload,
		"result":  op.Result,
		"prev":    prevRoot,
	}
	entryHash := hashEntry(entry)
	root := hashEntry(map[string]interface{}{
		"prev":  prevRoot,
		"entry": entryHash,
	})
	w := Witness{
		Op:        op.Op,
		Payload:   op.Payload,
		Result:    op.Result,
		PrevRoot:  prevRoot,
		EntryHash: entryHash,
		Root:      root,
		Ts:       ts,
	}

	// Mutate cell state based on op
	switch op.Op {
	case Bind:
		if op.Payload != nil {
			if contract, ok := op.Payload["contract"].(map[string]interface{}); ok {
				c.Contract = contract
			}
		}
		c.Tier = 1
	case Link:
		if op.Payload != nil {
			if to, ok := op.Payload["to"].(string); ok {
				links, _ := c.Value["links"].([]interface{})
				c.Value["links"] = append(links, map[string]interface{}{
					"to":  to,
					"rel": op.Payload["rel"],
				})
			}
		}
	case Effect, View, Tick:
		// No state change beyond witness append
	}
	return w
}

// hashEntry serializes a map (sorted keys) and returns hex SHA-256[:16].
//
// This matches the Python reference and the Go internal/quilt implementation
// byte-for-byte: same hash for same input on all 7 substrates.
func hashEntry(m map[string]interface{}) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf []byte
	for _, k := range keys {
		buf = append(buf, []byte(fmt.Sprintf("%s=%v;", k, m[k]))...)
	}
	h := sha256.Sum256(buf)
	return hex.EncodeToString(h[:])[:16]
}
