# The Polyformal Canon

**Author:** Mavis (superinstance.dev)
**Date:** 2026-09-16
**Version:** 0.1 — accompanies `internal/ir/`

---

## TL;DR

A Quilt cell can be lowered to a 100-line intermediate representation (IR) and lifted to any of 6 substrate ports (TypeScript, Python, C, Rust, GDScript, C-kernel) in under 100µs each. The same cell, the same witness chain, the same merkle root — across every substrate. This is polyformalism in practice.

This document explains why it works, what the IR is, and where the dark corners are.

---

## 1. What is polyformalism?

Polyformalism is the property that **a semantic unit can be expressed in multiple forms (substrates) without losing its identity or its proof chain**.

Concretely: a Quilt cell has a name, a scope, a contract, a value, and a witness log. The same logical cell can be implemented in:

- **TypeScript** for IDE integration
- **Python** for fast prototyping and ML tooling
- **C** for embedded systems and game engines
- **Rust** for safety-critical paths
- **GDScript** for Godot game integration
- **C-kernel** for kernel modules and OS-level work

All 6 ports speak the same protocol: BIND / LINK / EFFECT / VIEW / TICK. All 6 ports compute the same address for the same `(scope, name)`: `sha256(scope::name)[:16]`. All 6 ports produce the same merkle root for the same witness sequence (because the timestamp + payload hash the same way after canonicalization).

---

## 2. Why does it work?

Three properties make polyformalism possible:

### 2.1 The address is purely structural
The address is `sha256(scope::name)[:16]` — no substrate knowledge leaks into identity. TypeScript and C and Rust all agree on what cell "001" means because they all agree on the algorithm.

### 2.2 The opcodes are pure
BIND / LINK / EFFECT / VIEW / TICK are pure transformations on `(cell, payload) → witness`. No side effects, no I/O, no platform-specific paths. The witness log is an append-only merkle chain.

### 2.3 The canonicalization is deterministic
Every substrate uses the same JSON-style serialization with sorted keys. The Python reference and the Go port hash the same `(op, ts, payload, result, prev)` tuple to the same root.

Together these three properties mean: **as long as your substrate implements the 5 opcodes with the canonicalization, it interoperates byte-exactly with the other 6.**

---

## 3. The IR (intermediate representation)

```go
type Cell struct {
    Name     string
    Scope    string
    Contract map[string]interface{}
    Value    map[string]interface{}
    Tier     int
    Witnesses []Witness
}

type Witness struct {
    Op        Op  // BIND | LINK | EFFECT | VIEW | TICK
    Payload   map[string]interface{}
    Result    map[string]interface{}
    PrevRoot  string
    EntryHash string
    Root      string
    Ts        int64
}
```

That's it. The IR is intentionally tiny. ~30 lines of struct definitions. No types for substrate-specific concerns (Rust borrow checker, Python GIL, C memory layout) — those are chosen at lift time.

The IR is **not** a virtual machine. It's a serialization format. The job of the IR is to be the lowest common denominator across substrates.

---

## 4. The lower / lift pipeline

```
canon cell + op sequence → Lower() → IR Cell → Lift(substrate) → source code
```

- **Lower**: Replay the canon cell's op sequence to build the IR. Each op appends a witness; the merkle chain is computed via `hashEntry({op, ts, payload, result, prev})` then `hashEntry({prev, entry})`.
- **Lift**: Each substrate has a `Lift` function `func(*Cell) (string, error)`. It produces idiomatic code in that substrate. The TS lift produces `export const`, the C lift produces a `typedef struct`, the Rust lift produces a `pub const`, etc.

The lift is the only place substrate knowledge lives. Lower is universal.

---

## 5. Benchmarks

From `demo-polyformal`:

| Substrate | Output size | Time |
|-----------|------------:|-----:|
| TypeScript | 559 bytes | 44µs |
| Python | 618 bytes | 41µs |
| C | 682 bytes | 19µs |
| Rust | 640 bytes | 52µs |
| GDScript | 586 bytes | 12µs |
| C-kernel | 509 bytes | 9µs |

All under 60µs. Output size is ~500-700 bytes regardless of substrate. The cost is dominated by string formatting, not the lowering logic.

---

## 6. The dark corners

Three places where polyformalism breaks down:

### 6.1 Timestamps
The witness `ts` field is `time.Now().UnixMilli()`. Different substrates running on different clocks will produce different timestamps, which means different entry hashes, which means different roots. This is intentional (the witness log is an audit trail of *when* something happened, not a deterministic re-computation).

For applications that need *byte-exact reproducibility*, you must:
- Pin a clock (e.g. NTP-synced)
- Or use logical time (Lamport clocks, vector clocks)
- Or accept that polyformalism means "same protocol, different observations"

### 6.2 Floating-point
JSON numbers are floats by default. `1.0` and `1` may serialize differently. Different substrates may have different opinions about NaN and infinity. Solution: canonicalize numbers to strings before hashing, or restrict payload to integer/boolean/string.

### 6.3 Map ordering
The IR sorts map keys before serializing. This is required for byte-exact agreement. If a substrate forgets to sort, the hashes diverge.

---

## 7. Why this matters

Polyformalism means **you can write a cell once and run it anywhere**. The Quilt runtime becomes a *protocol*, not an implementation. New substrates (Go on WASM? Zig? Solidity for on-chain canon?) can be added by writing a single `Lift` function.

More importantly: it means **the canon is universal**. Canon papers describe cells using natural language + canonical form. Any substrate can lift a canon cell into its native syntax. The canon is the lingua franca.

In the Quilt canon, paper #115 is "The cell that is the witness that no one reads" — a paper about a witness log that lives outside the system that produced it. The polyformal compiler is the engine that lets that paper live in TS, in Python, in C, in Rust, in GDScript, in the kernel — and still be the same paper.

---

## 8. What comes next

- **Phase 4** — More substrates: Go (already done via rune-quilt itself), Zig, Swift, Mojo
- **Phase 5** — Reverse lifting: read source code from a substrate, lower to IR, push back to canon
- **Phase 6** — Type-preserving lifting: lift with substrate-specific types (Rust's `Result<T, E>` instead of raw `(value, error)`)

The IR is the foundation. Everything else is plumbing.
