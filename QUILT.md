# rune-quilt — The Quilt Layer in Rune

> The Quilt is a cellular-architecture framework where every UI is an opener onto the same 4D cell graph. **rune-quilt** is the Quilt layer inside Rune — the IDE for pros.

This doc describes what the Quilt is, how it lives in this repo, and how to extend it. For a non-technical overview, see [PLAIN_LANGUAGE.md](./PLAIN_LANGUAGE.md). For the relationship to upstream Rune, see [UPSTREAM.md](./UPSTREAM.md).

---

## The principle

**The unit of foundation is the cell, not the file.** In rune-quilt:

- Every file you open becomes a cell.
- Every project is a cluster of cells.
- Every command you run becomes a witness.
- Every cell has a contract (its exports), a value (its current state), and a merkle-rooted witness log.
- Every cell links to other cells via the canon.

The Quilt treats the workspace as a graph, not a tree. Files are nodes, but so are conversations, agent runs, queries, and external services. The five opcodes (BIND, LINK, EFFECT, VIEW, TICK) are the engine that runs on this graph.

## The five opcodes

| Opcode | What it does | When rune-quilt runs it |
|---|---|---|
| **BIND** | Create the cell's contract | When you open a file |
| **LINK** | Connect two cells | When a file imports another; when the agent references past work |
| **EFFECT** | Run an operation that changes state | On file save, on command run, on agent action |
| **VIEW** | Read cell state (no side effects) | On every agent query, on `cell` command |
| **TICK** | Advance the cell's logical clock | Every 60s (heartbeat); on file close |

Every opcode writes a witness. Every witness is hashed. The chain of hashes is the cell's **merkle root** — proof that the cell's history is what we say it is.

## Where the Quilt lives in this repo

```
rune-quilt/
├── internal/
│   ├── quilt/            # the cell primitive + cluster + opcodes (Go port)
│   │   └── cell.go
│   ├── canon/            # the Cloudflare Worker client (the canon)
│   │   └── client.go
│   └── a2a/              # the inter-cell messaging client (a2a Worker v2)
│       └── client.go
├── cmd/
│   └── extension_quilt/  # the Rune extension
│       ├── main.go       # entry point
│       └── extension.go  # event handlers, commands, lifecycle
└── demo/                 # runnable demo without Rune
    ├── demo.go
    └── sample-code/      # example files to canonize
```

The `internal/quilt` package is the **Go port** of the Quilt cell primitive. It is byte-exact with the Python reference (the same hash for the same witness chain).

The `internal/canon` package is a thin client over `live-canon.casey-digennaro.workers.dev`. Every cell that gets canonized is embedded via Cloudflare Workers AI and submitted for admission.

The `internal/a2a` package is a client over `quilt-a2a-v2.casey-digennaro.workers.dev`. Every rune-quilt instance registers itself as a cell on startup; semantic search over cells uses Vectorize.

## The data flow

```
   ┌───────────────┐
   │  Rune Editor  │   file open, content change, save, close, agent run
   └───────┬───────┘
           │ events (gRPC)
           ▼
   ┌───────────────────┐
   │ extension_quilt   │   event handlers in extension.go
   └───────┬───────────┘
           │ BIND / LINK / EFFECT / VIEW / TICK
           ▼
   ┌───────────────────┐
   │ quilt.Cell + Cluster  │   merkle-rooted witness log, in-memory
   └───────┬───────────┘
           │ canonize
           ▼
   ┌───────────────────┐
   │  canon.Client     │   POST /api/cell to live-canon Worker
   └───────┬───────────┘
           │ HTTPS
           ▼
   ┌──────────────────────────────────┐
   │ live-canon.casey-digennaro...    │   Cloudflare Worker + Vectorize
   │   502 papers, state_hash         │   byte-exact with Python reference
   └──────────────────────────────────┘

           in parallel:
           ▼
   ┌───────────────────┐
   │ a2a.Client        │   POST /register, /tick, /find-semantic
   └───────┬───────────┘
           │ HTTPS
           ▼
   ┌──────────────────────────────────┐
   │ quilt-a2a-v2.casey-digennaro...  │   Cloudflare Worker + Vectorize
   │   30 cells, semantic search      │   "who can compile to multiple substrates?"
   └──────────────────────────────────┘
```

## The commands

Once the extension is installed and running in a Rune workspace, four commands become available:

| Command | What it does |
|---|---|
| `quilt stats` | cluster size, total witnesses, a2a cell id, canon URL |
| `quilt view <uri>` | full snapshot of the cell for a file |
| `quilt peers <query>` | semantic search for cells matching a query |
| `quilt submit` | push every cell to the live canon |
| `cell` | show the active file's cell |
| `canon hash \| list \| navigate \| lineage` | canon queries |
| `peers <query>` | shortcut for `quilt peers` |

## The polyformalism — one canon, 6 substrates

The Quilt has 6 substrate ports: TypeScript, Python, C, Rust (no_std), Rust (MHS), GDScript. This repo is the **Go port** (joining the family).

Byte-exact means: the merkle root of a cell's witness log is identical regardless of which substrate runs it. A `BIND` in Go produces the same hash as a `BIND` in Python produces the same hash as a `BIND` in Rust. The polyformalism is not just portable — it's *in production*.

The Python reference: https://github.com/SuperInstance/quilt-python
The C port: https://github.com/SuperInstance/quilt-c
The Rust port: https://github.com/SuperInstance/quilt-rust
The cloudflare Worker: https://github.com/SuperInstance/quilt-cloudflare
The TypeScript VM: https://github.com/SuperInstance/quilt-vm-typescript

## Extending the Quilt layer

To add a new opcode: add a method to `quilt.Cell` in `internal/quilt/cell.go`, write the witness, update the merkle root.

To add a new canon endpoint: extend `internal/canon/client.go` with a new typed method.

To add a new a2a capability: extend `internal/a2a/client.go` with a new typed method.

To add a new Rune command: add a method on `Extension` in `cmd/extension_quilt/extension.go` and register it in `Setup()`.

## The visual layer (next phase)

The next big piece is `cmd/extension_quilt/visualizer` — a Rune webview showing the cell graph of the current workspace, with edges to the canon and other cells on the mesh. The graph uses force-directed layout (vanilla D3, no deps) and updates in real time as cells bind, link, and tick.

The reference implementation is in `superinstance-advisor/data/fleet_graph.html` — 8KB of vanilla JS, no dependencies, renders 30+ cells.

## The polyformal compiler (further out)

The long-term move: one canon cell → 6 substrate outputs. The same Quilt cell compiled to TypeScript, Python, C, Rust, Verilog, and VHDL. The compiler lives in the canon Worker (`/api/vibe?lang=X`) and is wired into the extension so a `?compile to rust` query returns the Rust port of the cell.

This is the killer feature for pros: **"show me this file in every language it could be"**.

## The mesh (further out)

rune-quilt should talk to other rune-quilt instances over Rune's `runenet` (WireGuard-based mesh). Cells on different machines discover each other by capability. The current a2a Worker is HTTP-based; the mesh version is gRPC-over-runenet.

That's the future. Right now the extension is single-workspace. The mesh is a roadmap item.

## License

This code is licensed under GPL-3.0-or-later, matching upstream Rune.
