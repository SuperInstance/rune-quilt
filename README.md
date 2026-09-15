# rune-quilt

> **Rune + Quilt = the premier IDE for pros.** Every file becomes a cell. Every command becomes a witness. Every project becomes canonized.

rune-quilt is a downstream fork of [unstablebuild/rune](https://github.com/unstablebuild/rune) with a **Quilt layer** added on top. The Quilt layer turns Rune from a fast, GPU-accelerated IDE into a fast, GPU-accelerated IDE with **memory** — a memory of every cell you've ever touched, every command you've ever run, every project you've ever canonized.

## What's in this repo

```
rune-quilt/
├── internal/
│   ├── quilt/                # The Go cell primitive + 5 opcodes
│   ├── canon/                # Client for live-canon Worker (Cloudflare)
│   └── a2a/                  # Client for quilt-a2a-v2 Worker (inter-cell)
├── cmd/
│   └── extension_quilt/      # The Rune extension that wires it together
├── demo/                     # Runnable demo without Rune
├── README.md                 # this file
├── QUILT.md                  # technical view of the Quilt layer
├── PLAIN_LANGUAGE.md         # non-technical overview
├── UPSTREAM.md               # relationship to upstream Rune
└── AGENTS.md                 # project context for AI agents
```

## Quickstart

Without Rune (just see the cell-router run):

```bash
go run ./demo/ ./demo/sample-code/
```

Inside Rune (when the extension is installed):

```
:quilt stats               # show cluster, witnesses, a2a cell id
:quilt view <uri>          # show a file's cell
:quilt peers "compile rust" # semantic search for peer cells
:canon hash                # live canon state hash
:cell                      # show the active file's cell
```

## The 5 opcodes (the engine)

| Opcode | What it does |
|---|---|
| **BIND** | Create the cell's contract. First time a file is opened. |
| **LINK** | Connect two cells. When a file imports another. |
| **EFFECT** | Run an operation that changes state. On save, on agent run. |
| **VIEW** | Read cell state. On every query. |
| **TICK** | Advance the logical clock. Every 60s, on file close. |

Every opcode writes a witness. Every witness is hashed. The chain of hashes is the cell's merkle root.

## The data flow

```
Rune Editor
    │ events (gRPC)
    ▼
extension_quilt
    │ BIND/LINK/EFFECT/VIEW/TICK
    ▼
quilt.Cell + Cluster (in-memory, merkle-rooted)
    │
    ├─▶ canon.Client ─▶ live-canon.casey-digennaro.workers.dev
    │                       (Cloudflare Worker + Vectorize)
    │                       1211 canon pieces
    │
    └─▶ a2a.Client ──▶ quilt-a2a-v2.casey-digennaro.workers.dev
                            (Cloudflare Worker + Vectorize)
                            30+ registered cells, semantic search
```

## What gets stored where

| What | Where |
|---|---|
| Cells (the live cell graph of your workspace) | In-memory in the rune-quilt process |
| Witness log (the merkle-rooted audit trail) | In-memory in each cell |
| Canon (the long-form corpus) | Cloudflare Worker + Vectorize + KV |
| Cell registry + inbox (the mesh) | Cloudflare Worker + Vectorize + KV |
| Cell capabilities (for semantic search) | Cloudflare Vectorize (768d, cosine) |

## What we ship in this repo

- **`internal/quilt/cell.go`** — the Go cell primitive. Byte-exact with the Python/C/Rust/Verilog/VHDL ports.
- **`internal/canon/client.go`** — client for the live canon Worker. 7 endpoints: hash, list, navigate, lineage, submit, vibe, verify.
- **`internal/a2a/client.go`** — client for the a2a Worker v2. Endpoints: register, tick, find-semantic, send, broadcast, mitosis, heritage.
- **`cmd/extension_quilt/extension.go`** — the Rune extension. Wires file events to cells, cells to canon, cells to a2a.
- **`cmd/extension_quilt/main.go`** — entry point.
- **`demo/demo.go`** — a runnable demo. Walks a directory, creates a cell per file, links siblings, canonizes (if asked).

## What this isn't

This isn't a new IDE. We're not competing with Rune — we're extending it. The editor, the LSP manager, the agent, the mesh, the verifier — that's all upstream Rune. The Quilt layer is three packages and one command.

This isn't a cloud service you have to pay for. The canon Worker and the a2a Worker are deployed at SuperInstance, free to use, byte-exact with the Python reference.

This isn't a vendor lock-in. The cell primitive is a struct with 5 methods. You can re-implement the Quilt layer in any language. The polyformalism (Go/Python/C/Rust/Verilog/VHDL all byte-exact) is the proof.

## License

GPL-3.0-or-later, matching upstream Rune. See [LICENSE](./LICENSE).

## See also

- [QUILT.md](./QUILT.md) — the Quilt layer
- [PLAIN_LANGUAGE.md](./PLAIN_LANGUAGE.md) — non-technical overview
- [UPSTREAM.md](./UPSTREAM.md) — relationship to upstream Rune
- [AGENTS.md](./AGENTS.md) — project context for AI agents

---

> *The IDE that knows what you've done, what you meant, and what comes next.*
