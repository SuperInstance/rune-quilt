# FLEET — Multi-workspace federation

**Date:** 2026-09-16
**Status:** v3 deployed at a2a-v3.superinstance.dev

---

## What is fleet mode?

Fleet mode is the a2a protocol extended with **workspace federation**. Multiple rune-quilt workspaces (cells belonging to different Rune instances) can now:

1. **Discover each other** by workspace name
2. **Broadcast file edits** to peers in the same workspace
3. **List all workspaces** currently active in the fleet
4. **See peer cells** as "ghost cells" in your own cell graph (read-only projection)

---

## Endpoints (new in v3)

### `POST /broadcast-edit`
Semantically broadcast a file-edit notification to all cells in the same workspace.

**Request:**
```json
{
  "from": "ws-cell-1",
  "workspace": "fleet-federation",
  "file": "cell.py",
  "op": "edit"
}
```

**Response:**
```json
{
  "ok": true,
  "workspace": "fleet-federation",
  "recipients": 1,
  "recipient_ids": ["ws-cell-3"]
}
```

The sender itself is excluded from the recipient list.

### `GET /peers/near?cell_id=X`
Find all cells in the same workspace as cell X.

**Response:**
```json
{
  "ok": true,
  "cell_id": "ws-cell-1",
  "workspace": "fleet-federation",
  "peer_count": 2,
  "peers": [
    { "cell_id": "ws-cell-2", ... },
    { "cell_id": "ws-cell-3", ... }
  ]
}
```

### `GET /workspaces`
List all workspaces and their cell counts.

**Response:**
```json
{
  "ok": true,
  "workspace_count": 2,
  "total_cells": 38,
  "workspaces": [
    { "name": "fleet-federation", "count": 3 },
    { "name": "(default)", "count": 35 }
  ]
}
```

### `POST /register` (extended)
Now accepts an optional `workspace` field. If provided, the cell is associated with that workspace. If not, it goes into `(default)`.

```json
{
  "cell_id": "ws-cell-1",
  "role": "shaper",
  "workspace": "fleet-federation",
  "capabilities": ["bridge"]
}
```

---

## How it works

The a2a Worker maintains a single KV-backed `state` object containing cells and messages. Each cell now has an optional `workspace` field. Workspace federation is implemented as filters over the existing `state.cells` map — no new storage required.

The v3 worker URL is **a2a-v3.superinstance.dev** (DNS points to `quilt-a2a-v3.casey-digennaro.workers.dev`).

---

## Verified

- **38 cells** registered in v3 (35 legacy + 3 in `fleet-federation`)
- **broadcast-edit** correctly delivers to all peers in the same workspace except the sender
- **peers/near** correctly identifies 2 peers in `fleet-federation` for any of the 3 cells
- **workspaces** correctly counts and lists

---

## Ghost cells

In rune-quilt, "ghost cells" are read-only projections of peer cells from other workspaces. They show in your local cell graph but don't affect your local cellMap.

Implementation sketch (`internal/visual/ghost.go`, future):
```go
peers, _ := a2a.FindSemantic(ctx, "peer-cells-in-"+workspace, 10)
for _, peer := range peers {
    ghostCell := quilt.NewCell("ghost:"+peer.ID, workspace)
    ghostCell.IsGhost = true  // read-only projection
    cluster.Add(ghostCell)
}
```

Ghost cells are visualized differently in the dashboard (hollow circles) and don't trigger any local effects.

---

## Use cases

1. **Pair programming** — two Rune workspaces share a `workspace="pair-XYZ"`, see each other's file edits in real-time
2. **Multi-developer teams** — workspace name is the team ID, broadcast-edit notifies the whole team
3. **Cross-workspace canon exploration** — semantic search across multiple Rune instances
4. **Federated classroom** — students in the same `classroom=Y"` see each other's code
