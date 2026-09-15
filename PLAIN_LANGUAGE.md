# rune-quilt in Plain Language

> A new way to think about your code.

## What is rune-quilt?

rune-quilt is **Rune** — a real, GPU-accelerated IDE used by professional developers — with a Quilt layer added on top.

The Quilt layer is one idea: **every file you open becomes a "cell" that remembers what you did with it.**

That's it. That's the whole idea.

## Why would I want this?

Imagine you're a pro. You work on a project. You open `auth.rs`. You fix a bug. You close it. Six months later, you open `auth.rs` in a different project. The cell for the old `auth.rs` is still there. It knows:

- When you opened it
- What you changed
- What other files you were working on
- What you asked the AI agent about it
- What other projects have similar cells

When you ask "what does this file do?", the answer isn't just what the file does. It's what the cell knows about the file across all your work.

That's the canon. The canon is the long-form memory of every cell you've ever touched.

## What does it actually do today?

The rune-quilt extension, when running inside Rune, does four things:

1. **Every file you open becomes a cell.** The cell has a name (the file path), a scope (the project), and a contract (the file's exports).
2. **Every command you run becomes a witness.** A witness is a record of what you did: "opened", "saved", "asked about", "ran tests". The cell's witnesses form a chain, like a blockchain but for code edits.
3. **Every cell is canonized.** The cell is sent to a Cloudflare Worker that holds the long-form corpus. The Worker embeds the cell using AI and admits it to the canon. Now the cell is searchable.
4. **Every cell is discoverable.** When another cell on another machine asks "who can compile Rust to Verilog?", the answer comes from a Vectorize index over all registered cells.

## What does "Quilt" mean?

Quilt is a metaphor. A real quilt is made by stitching together many pieces of fabric. Each piece is its own thing. The quilt is the whole.

The Quilt (the framework) is the same idea: each cell is its own thing, with its own state, its own witness log, its own contract. The Quilt is the whole — the cell graph, the canon, the distributed mesh.

## Is this just a fancy LSP?

No. An LSP (language server protocol) tells you about the code in front of you: types, definitions, references, errors. It doesn't remember what you did. It doesn't talk to other files on other machines. It doesn't build a corpus.

The Quilt layer sits **on top of** the LSP. It uses the LSP to know about the code, then layers in:

- **Memory** — what you did with this cell, when, why
- **Canon** — what other cells say about this one, across all your projects
- **Mesh** — what other cells (on other machines, other users) can do

## What's a cell, really?

A cell is a struct. That's it. It has:

- A name (string)
- A scope (string)
- A contract (map[string]any)
- A value (map[string]any)
- A list of witnesses (with merkle root)
- An address (a SHA-256 of name + scope, truncated)

That's the whole cell. No magic. No hidden state. No vendor lock-in.

In Go, in this repo, the cell is `quilt.Cell` in `internal/quilt/cell.go`. You can read the whole definition in 200 lines.

## What's the 5 opcodes?

The cell has 5 operations:

- **BIND** — create the cell's contract (the first time you open the file)
- **LINK** — connect this cell to another (when one file imports another)
- **EFFECT** — run an operation that changes state (when you save, when the agent runs)
- **VIEW** — read cell state (every query, every `?` question)
- **VIEW** doesn't change anything. It's the noun. **EFFECT** is the verb.
- **TICK** — advance the logical clock (every 60 seconds, on file close)

These 5 operations are the **engine** of the Quilt. In rune-quilt, they fire automatically on file events. The cell-router (the Go code in `cmd/extension_quilt/extension.go`) is the wiring.

## What's the witness log?

The witness log is the audit trail. Every BIND, LINK, EFFECT, VIEW, TICK is recorded with:

- The opcode
- A payload (what happened)
- A timestamp
- A hash of the previous witness
- A merkle root

The merkle root is a fingerprint. Two cells with the same witness log have the same root. You can prove a cell did what you said it did by hashing its witness log and comparing the root.

## What's the canon?

The canon is the long-form corpus. It's a Cloudflare Worker at `live-canon.casey-digennaro.workers.dev`. It holds:

- Every paper in the Quilt canon (1200+ papers as of this writing)
- Every cell that's been canonized
- The byte-exact state hash (`0x7d8d32cd7f8a9f26`)
- 7 operations: NAVIGATE, CONFLUENCE, LINEAGE, GHOST, TICK, F/V EILEEN

When rune-quilt canonizes a cell, it embeds the cell's content via Cloudflare Workers AI (bge-base-en-v1.5) and submits it to the live canon. The canon admits the cell, hashes the state, and stores it.

## What's the mesh?

The mesh is the peer-to-peer network. Every rune-quilt instance registers itself as a cell on the a2a Worker at `quilt-a2a-v2.casey-digennaro.workers.dev`. The Worker holds:

- A registry of every cell (currently 30+)
- A Vectorize index over cell capabilities (so you can search semantically)
- A KV-backed inbox for inter-cell messaging

When you ask "who can compile Rust to Verilog?", the answer comes from Vectorize over the cell capabilities. The cells with the right capabilities are returned, ranked by semantic similarity.

## What's next?

The roadmap:

1. **Visual layer** — a force-directed graph of all live cells in your workspace, rendered in a Rune webview.
2. **Polyformal compiler** — one canon cell → 6 substrate outputs (TypeScript, Python, C, Rust, Verilog, VHDL). "Show me this file in every language."
3. **Mesh via runenet** — inter-cell messaging over Rune's WireGuard mesh, not just HTTPS.
4. **Memory management** — pruning cells, archiving witnesses, summarizing old projects.

The first two are in active development. The last two are roadmap items.

## Who built this?

rune-quilt is built by SuperInstance (https://github.com/SuperInstance), the maintainers of the Quilt ecosystem. The Quilt is a cellular-architecture framework with 100+ repos across 12+ languages.

The Rune editor is built by Unstable Build, LLC (https://rune.build). Rune is the IDE we're extending.

We are not affiliated with Unstable Build. rune-quilt is a downstream fork. See [UPSTREAM.md](./UPSTREAM.md) for the relationship.

## How do I try it?

```bash
# Get Rune
curl -fsSL https://rune.build/install.sh | sh

# Get rune-quilt
pkg install github.com/SuperInstance/rune-quilt

# Open any project
rune ~/my-project

# Try the commands
:quilt stats
:quilt peers "compile to rust"
:canon hash
```

That's it. Open files. Watch them become cells. Ask the agent a question. Watch the canon answer.

## Why should I care?

Because every other IDE forgets. Rune is the best at staying in the flow — fast, GPU-accelerated, keyboard-driven. But when you close the editor, your context is gone. The next project starts from zero.

rune-quilt is the memory layer. It doesn't slow you down. It just remembers. So when you come back, the IDE remembers you.

That's the bet. The IDE that knows what you've done, what you meant, and what comes next.

## See also

- [QUILT.md](./QUILT.md) — the technical view
- [UPSTREAM.md](./UPSTREAM.md) — relationship to Rune
- [README.md](./README.md) — top-level readme
