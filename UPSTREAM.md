# UPSTREAM.md — Relationship to upstream Rune

> rune-quilt is a fork of [unstablebuild/rune](https://github.com/unstablebuild/rune) with a Quilt layer added.

## The fork

This repo starts as a fork of upstream Rune at the time of the fork. We add:

- `internal/quilt/` — the Go cell primitive + cluster
- `internal/canon/` — client for the live canon Worker
- `internal/a2a/` — client for the a2a Worker v2
- `cmd/extension_quilt/` — the Rune extension
- `demo/` — a runnable demo without Rune
- `QUILT.md`, `PLAIN_LANGUAGE.md`, `UPSTREAM.md` (this file)

Everything else — the editor, the LSP manager, the agent, the mesh — comes from upstream Rune unchanged.

## What we track

We track upstream Rune's `main` branch. To pull the latest changes:

```bash
git remote add upstream https://github.com/unstablebuild/rune.git
git fetch upstream
git merge upstream/main
```

The Quilt layer is in three packages plus one cmd directory. If upstream changes touch these, we resolve manually. Otherwise the merge is automatic.

## What we contribute back

Eventually. Not yet. We have not contributed any of the Quilt layer to upstream Rune because:

1. The Quilt layer is opinionated about Cloudflare Workers (live-canon, quilt-a2a-v2) which upstream Rune doesn't depend on.
2. The Quilt layer assumes the `unstablebuild` extension permission model, which would require UnstableBuild to bless it as a verified publisher.
3. The Quilt canon + a2a Workers are SuperInstance infrastructure; we don't want upstream Rune to depend on it.

If upstream Rune adds a generic "extension data" feature — a way for extensions to register their own persistent storage and messaging — we'd contribute the Quilt layer back. Until then, this is a downstream fork.

## What we DON'T change

We don't modify any file under:

- `cmd/rune/` (the editor)
- `cmd/rune-agent/` (the AI agent)
- `internal/term/`, `internal/text/`, `internal/ide/`, `internal/component/` (the editor core)
- `internal/extension/` (the extension system — we use it, we don't change it)
- `internal/runenet/` (the peer-to-peer mesh)

The Quilt layer plugs in via `internal/quilt/`, `internal/canon/`, `internal/a2a/`, and `cmd/extension_quilt/`. That's it.

## Versioning

This repo uses semantic versioning on the Quilt layer only. Upstream Rune uses its own versioning.

The Quilt layer's first version is `0.1.0` — early alpha. The interface is unstable; expect breaking changes until `1.0.0`.

The canonical version lives in:
- `cmd/extension_quilt/extension.go` — `ExtensionVersion` in `NewExtension()`
- The metadata block of `cmd/extension_quilt/main.go`

## License compatibility

Upstream Rune is GPL-3.0-or-later. Our additions are also GPL-3.0-or-later (matching the LICENSE_HEADER template). This means:

- Anything you write against the rune-quilt extension API is **not** required to be GPL.
- Anything you write **inside** the extension (e.g. handlers, commands) **is** required to be GPL.
- Distributing rune-quilt requires the source.

If you want a non-GPL Quilt layer, you can write your own extension — the public API (gRPC to Rune, HTTPS to Workers) is unencumbered. But you can't reuse our Go code without GPL.

## Contributing to rune-quilt

Open a PR. Make sure:

1. The new code uses the `LICENSE_HEADER` template at the top of every file.
2. `go test ./...` passes.
3. `make lint` passes.
4. The commit is signed off (`git commit -s`) per the DCO.
5. New commands register in `Setup()` in `extension.go`.

The maintainers of SuperInstance review and merge.

## Sync notes

When upstream changes break the Quilt layer:

- `internal/extension/` API changes → update `cmd/extension_quilt/extension.go` to match
- `internal/extensionapi/` API changes → update permission list and metadata in `cmd/extension_quilt/extension.go`
- `internal/llm/` or `cmd/rune-agent/` changes → no impact (we don't touch them)
- `internal/runenet/` changes → opportunity for mesh integration; low priority

When the Quilt canon or a2a Workers change:

- `internal/canon/client.go` endpoint additions or changes
- `internal/a2a/client.go` endpoint additions or changes

These should happen in lockstep with the Workers.

## See also

- [QUILT.md](./QUILT.md) — the Quilt layer
- [PLAIN_LANGUAGE.md](./PLAIN_LANGUAGE.md) — non-technical overview
- [README.md](./README.md) — top-level readme
- [AGENTS.md](./AGENTS.md) — project context for AI agents
