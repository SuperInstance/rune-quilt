// Package ir is the intermediate representation for the polyformal compiler.
//
// The IR is the *minimal* representation of a Quilt cell that can be lowered
// from a canon cell and lifted to any of the 6 substrate ports (TS, Python,
// C, Rust, GDScript, C-kernel). The IR is intentionally tiny: just the
// data + 5 opcodes + a few semantic predicates.
package ir

// Op is one of the 5 canonical opcodes. These names match the 6 polyformal
// ports byte-exactly (all-caps per Quilt canon).
type Op string

const (
	Bind   Op = "BIND"
	Link   Op = "LINK"
	Effect Op = "EFFECT"
	View   Op = "VIEW"
	Tick   Op = "TICK"
)

// Cell is the IR representation of a single cell.
//
// It captures everything that varies across substrates:
//   - Name (the cell's identity)
//   - Scope (the cluster it belongs to)
//   - Contract (initial input map)
//   - Value (current state map)
//   - Witnesses (the audit log)
//
// Substrate-specific concerns (Rust borrow checker, Python GIL, C memory
// layout) are NOT in the IR — they're chosen at lift time.
type Cell struct {
	Name     string                 `json:"name"`
	Scope    string                 `json:"scope"`
	Contract map[string]interface{} `json:"contract,omitempty"`
	Value    map[string]interface{} `json:"value,omitempty"`
	Tier     int                    `json:"tier"`

	Witnesses []Witness `json:"witnesses"`
}

// Witness is one entry in the cell's merkle-rooted log.
type Witness struct {
	Op        Op                     `json:"op"`
	Payload   map[string]interface{} `json:"payload"`
	Result    map[string]interface{} `json:"result,omitempty"`
	PrevRoot  string                 `json:"prev_root"`
	EntryHash string                 `json:"entry_hash"`
	Root      string                 `json:"root"`
	Ts        int64                  `json:"ts"`
}

// Substrate identifies one of the 6 polyformal ports.
type Substrate string

const (
	TS       Substrate = "ts"
	Python   Substrate = "python"
	C        Substrate = "c"
	Rust     Substrate = "rust"
	GDScript Substrate = "gdscript"
	Kernel   Substrate = "c-kernel"
)

// AllSubstrates is the canonical ordering used by benchmarks.
var AllSubstrates = []Substrate{TS, Python, C, Rust, GDScript, Kernel}

// Lift is the interface every substrate implements.
//
// Given a Cell in the IR, return the equivalent code in that substrate.
// This is intentionally simple: one function, no hidden state.
type Lift func(*Cell) (string, error)

// Registry maps substrate names to lifters.
type Registry map[Substrate]Lift
