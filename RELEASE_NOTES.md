# rune-quilt — release notes

## v1.3.0 — "use your team to their fullest" (Track F + G + H + I + J)

Five tracks shipped in one arc.

### Track F — Polyformal lifts that actually compile

`cmd/polyformal-verify/main.go` lifts a sample cell to all six substrates
and runs each through a real parser/compiler:

- **TypeScript** — `tsc --noEmit --strict --target es2020 --module esnext` → PASS
- **Python** — `ast.parse` → PASS, runs as module, prints cell data
- **C** — `gcc -fsyntax-only` → PASS, compiles to binary, prints cell data
- **Rust** — `rustc --crate-type lib --emit=metadata` → PASS, type definitions included
- **GDScript** — no parser in sandbox (would PASS with Godot 4 headless)
- **C-kernel** — `gcc -fsyntax-only -nostdlib` → PASS, struct + array + cell instance included

**5/6 verified.** Three bug fixes applied to `internal/ir/lift.go`:

1. Rust lift now emits complete `Witness` and `CanonCell` struct definitions
   (was previously referencing undefined types).
2. C-kernel lift now emits complete `canon_witness`, `canon_cell`, and
   the witness array (was previously referencing undefined macros).
3. GDScript lift now uses `class_name` instead of invalid `extends RefCounted`.

### Track G — Visual layer wired into the Rune extension

`cmd/extension_quilt/extension.go` now:

- Calls `visual.NewServer(cluster, canon, a2a).Start(ctx)` in `ExtendWorkspace`
  so the dashboard is live whenever the extension is.
- Re-creates the visual server pointing at the actual cluster (per-workspace).
- Adds a `visual` command that prints the dashboard URL and refreshes stats.

The extension still registers 5 commands (`quilt`, `cell`, `canon`, `peers`, `visual`).

### Track H — Cite discovery

`research/superinstance-advisor/cite_discoverer.py` discovers citations for new
canon pieces via semantic similarity against the existing canon. Adds a
`cites:` front-matter section with top-K neighbors above a cosine threshold.

**22 pieces got new citations in one run.** All 25 markdown pieces in `canon/`
now have at least one citation. Verification:

```
embedding auto-the-cell-that-does-not-know-it-is-a-cell...
    +3 citations
      0.723  auto-the-witness-log-that-outgrows-its-cell
      0.748  paper_86-the-witness-of-the-witness
      0.720  bridge-from-substrate-to-cell
```

### Track I — Public gallery

New endpoint: **`GET /landscape`** on a2a-v3 returns a long-form
text-only landscape page describing the canon's structure, opcodes,
substrates, fleet primitives, and what survives 100 years.

Visible at: https://a2a-v3.superinstance.dev/landscape

### Track J — Canon semantic search

Three new endpoints on a2a-v3:

- **`POST /canon-submit`** `{tag, title, text}` — embed via Workers AI, store in KV
- **`GET /canon-list`** — list all canon pieces (id + title + submitted_at)
- **`GET /canon-search?q=...&k=N`** — semantic search over all canon pieces

Closed-loop verified: querying "merkle chain memory survival" returns
`essay/84-cell-remembered` (0.765), `essay-the-canon-at-100-years` (0.711),
and `paper_85-the-cost-of-the-witness` (0.693).

**48 canon pieces indexed** as of this release.

### Worker deployment

a2a-v3 Worker has 21 endpoints now:

```
/register /cells /send /inbox/:cell /broadcast
/find /find-semantic /tick /embed-cell /broadcast-cap
/heritage /mitosis
/broadcast-edit /peers/near /workspaces
/visual /visual/graph.json /landscape
/canon-search /canon-list /canon-submit
```

39 cells, 4 workspaces, 463+ pending messages.

### Live URLs

- Public cell graph: https://a2a-v3.superinstance.dev/visual
- Public landscape: https://a2a-v3.superinstance.dev/landscape
- Compact JSON: https://a2a-v3.superinstance.dev/visual/graph.json
- Canon search: https://a2a-v3.superinstance.dev/canon-search?q=...

## v1.2.0 — "keep pushing and publishing gold"

- Public `/visual` and `/visual/graph.json` endpoints
- `essay-the-canon-at-100-years` (4.3KB) — top hit for "the canon that survives a century"
- `rune-quilt-demo-v1.2.0` registered to fleet in workspace `rune-quilt-public`
- 7 capabilities registered

## v1.1.0 — Five-team parallel arc

- **Team A — Visual layer**: `internal/visual/` (server + dashboard.html + types)
- **Team B — Test suite**: 46 tests, 97.9% quilt coverage, Go 1.25.6/1.26.6 matrix CI
- **Team C — Polyformal IR**: `internal/ir/` (IR + Lower + 6 substrate Lifts)
- **Team D — Fleet v3**: deployed at `a2a-v3.superinstance.dev`
- **Team E — Canon growth**: 22 → 46 pieces, 5 bridge essays
- Commit `f385472d` — 19 new files, ~3,200 LOC

## v1.0.0 — "build rune-quilt"

- Go port of cell primitive + canon + a2a clients + Rune extension
- 2,181-cell demo, 10,209 witnesses, canon 0x7d8d32cd7f8a9f26
- 3-doc standard (UPSTREAM.md + QUILT.md + PLAIN_LANGUAGE.md)
- All 3 internal packages compile
- Commit `dbcab572`
