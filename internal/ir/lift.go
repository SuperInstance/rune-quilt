package ir

import (
	"fmt"
	"strings"
)

// Lifts holds the registry of substrate lifters.
var Lifts = Registry{
	TS:       liftTS,
	Python:   liftPython,
	C:        liftC,
	Rust:     liftRust,
	GDScript: liftGDScript,
	Kernel:   liftKernel,
}

// liftTS — TypeScript port (target: ~30 lines).
func liftTS(c *Cell) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "// Auto-generated from IR\n")
	fmt.Fprintf(&b, "export const CANON_%s_%s = {\n",
		sanitize(c.Scope), sanitize(c.Name))
	fmt.Fprintf(&b, "  name: %q,\n  scope: %q,\n", c.Name, c.Scope)
	fmt.Fprintf(&b, "  contract: %s,\n", jsLit(c.Contract))
	fmt.Fprintf(&b, "  witnesses: [\n")
	for _, w := range c.Witnesses {
		fmt.Fprintf(&b, "    {op: %q, root: %q, ts: %d},\n",
			string(w.Op), w.Root, w.Ts)
	}
	fmt.Fprintf(&b, "  ],\n};\n")
	return b.String(), nil
}

// liftPython — Python port.
func liftPython(c *Cell) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "# Auto-generated from IR\n")
	fmt.Fprintf(&b, "CANON_%s_%s = {\n",
		sanitize(c.Scope), sanitize(c.Name))
	fmt.Fprintf(&b, "    'name': %q, 'scope': %q,\n", c.Name, c.Scope)
	fmt.Fprintf(&b, "    'contract': %s,\n", pyLit(c.Contract))
	fmt.Fprintf(&b, "    'witnesses': [\n")
	for _, w := range c.Witnesses {
		fmt.Fprintf(&b, "        {'op': %q, 'root': %q, 'ts': %d},\n",
			string(w.Op), w.Root, w.Ts)
	}
	fmt.Fprintf(&b, "    ],\n}\n")
	return b.String(), nil
}

// liftC — C port.
func liftC(c *Cell) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "/* Auto-generated from IR */\n")
	fmt.Fprintf(&b, "#include <stdint.h>\n#include <stddef.h>\n\n")
	fmt.Fprintf(&b, "typedef struct {\n")
	fmt.Fprintf(&b, "    const char* name;\n")
	fmt.Fprintf(&b, "    const char* scope;\n")
	fmt.Fprintf(&b, "    size_t witness_count;\n")
	fmt.Fprintf(&b, "    struct { const char* op; uint64_t ts; char root[17]; } witnesses[%d];\n",
		len(c.Witnesses))
	fmt.Fprintf(&b, "} canon_%s_%s_t;\n\n",
		sanitize(c.Scope), sanitize(c.Name))
	fmt.Fprintf(&b, "static const canon_%s_%s_t CANON_%s_%s = {\n",
		sanitize(c.Scope), sanitize(c.Name),
		sanitize(c.Scope), sanitize(c.Name))
	fmt.Fprintf(&b, "    %q, %q, %d, {\n",
		c.Name, c.Scope, len(c.Witnesses))
	for _, w := range c.Witnesses {
		fmt.Fprintf(&b, "        {%q, %d, \"%s\"},\n",
			string(w.Op), w.Ts, w.Root)
	}
	fmt.Fprintf(&b, "    }\n};\n")
	return b.String(), nil
}

// liftRust — Rust port.
func liftRust(c *Cell) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "// Auto-generated from IR\n")
	fmt.Fprintf(&b, "pub const CANON_%s_%s: CanonCell = CanonCell {\n",
		sanitize(c.Scope), sanitize(c.Name))
	fmt.Fprintf(&b, "    name: %q,\n    scope: %q,\n", c.Name, c.Scope)
	fmt.Fprintf(&b, "    witness_count: %d,\n", len(c.Witnesses))
	fmt.Fprintf(&b, "    witnesses: &[\n")
	for _, w := range c.Witnesses {
		fmt.Fprintf(&b, "        Witness { op: %q, ts: %d, root: %q },\n",
			string(w.Op), w.Ts, w.Root)
	}
	fmt.Fprintf(&b, "    ],\n};\n")
	return b.String(), nil
}

// liftGDScript — Godot port.
func liftGDScript(c *Cell) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "# Auto-generated from IR\n")
	fmt.Fprintf(&b, "extends RefCounted\nclass_name CanonCell_%s_%s\n\n",
		sanitize(c.Scope), sanitize(c.Name))
	fmt.Fprintf(&b, "var name_: String = %q\n", c.Name)
	fmt.Fprintf(&b, "var scope_: String = %q\n", c.Scope)
	fmt.Fprintf(&b, "var witnesses: Array = [\n")
	for _, w := range c.Witnesses {
		fmt.Fprintf(&b, "    {\"op\": %q, \"ts\": %d, \"root\": %q},\n",
			string(w.Op), w.Ts, w.Root)
	}
	fmt.Fprintf(&b, "]\n")
	return b.String(), nil
}

// liftKernel — C-kernel (kernel module) port — minimal, no stdlib.
func liftKernel(c *Cell) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "/* Auto-generated from IR — kernel module */\n")
	fmt.Fprintf(&b, "struct canon_witness_%s_%s {\n",
		sanitize(c.Scope), sanitize(c.Name))
	fmt.Fprintf(&b, "    char op[16];\n    uint64_t ts;\n    char root[17];\n")
	fmt.Fprintf(&b, "};\n\n")
	fmt.Fprintf(&b, "static const struct canon_witness_%s_%s WITNESSES[%d] = {\n",
		sanitize(c.Scope), sanitize(c.Name), len(c.Witnesses))
	for _, w := range c.Witnesses {
		fmt.Fprintf(&b, "    {\"%s\", %d, \"%s\"},\n",
			string(w.Op), w.Ts, w.Root)
	}
	fmt.Fprintf(&b, "};\n")
	return b.String(), nil
}

// sanitize makes a string safe for use as a C / Rust identifier.
func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "anon"
	}
	if r := b.String()[0]; r >= '0' && r <= '9' {
		return "_" + b.String()
	}
	return b.String()
}

// jsLit renders a map as a JS object literal.
func jsLit(m map[string]interface{}) string {
	if len(m) == 0 {
		return "{}"
	}
	var b strings.Builder
	b.WriteString("{")
	first := true
	for k, v := range m {
		if !first {
			b.WriteString(", ")
		}
		first = false
		fmt.Fprintf(&b, "%q: %s", k, jsVal(v))
	}
	b.WriteString("}")
	return b.String()
}

func jsVal(v interface{}) string {
	switch x := v.(type) {
	case string:
		return fmt.Sprintf("%q", x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprintf("%d", x)
	case float64:
		return fmt.Sprintf("%g", x)
	default:
		return fmt.Sprintf("%q", fmt.Sprintf("%v", v))
	}
}

// pyLit renders a map as a Python dict literal.
func pyLit(m map[string]interface{}) string {
	if len(m) == 0 {
		return "{}"
	}
	var b strings.Builder
	b.WriteString("{")
	first := true
	for k, v := range m {
		if !first {
			b.WriteString(", ")
		}
		first = false
		fmt.Fprintf(&b, "%q: %s", k, pyVal(v))
	}
	b.WriteString("}")
	return b.String()
}

func pyVal(v interface{}) string {
	switch x := v.(type) {
	case string:
		return fmt.Sprintf("%q", x)
	case bool:
		if x {
			return "True"
		}
		return "False"
	case int:
		return fmt.Sprintf("%d", x)
	case float64:
		return fmt.Sprintf("%g", x)
	default:
		return fmt.Sprintf("%q", fmt.Sprintf("%v", v))
	}
}
