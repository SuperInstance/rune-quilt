// Copyright (C) 2017-2026 The Rune Authors
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

// Command extension_quilt is the rune-quilt Rune extension.
//
// It runs inside a Rune workspace as a child process, talks to Rune over
// gRPC, and turns every file the user touches into a Quilt cell. The cells
// are then canonized to the live canon Worker and registered with the a2a
// Worker, so they can be discovered by other cells on the mesh.
//
// On startup:
//   1. Connect to the canon Worker and fetch the state hash
//   2. Register with the a2a Worker as a cell with capabilities
//   3. Wire up Rune event handlers (file open, save, close, agent run)
//   4. On every event, create/update a cell, log a witness, query canon
//
// On every file event:
//   - Read the file (Rune gives us URI + content)
//   - Create or update a Cell with the file path as name
//   - LINK to related cells (imports, references)
//   - EFFECT to canonize (embed + submit to live canon)
//   - VIEW to surface related canon pieces to the agent
//
// On every agent run:
//   - The agent queries the canon first (it sees related cells across projects)
//   - The agent's actions become witnesses in the cell's log
package main

import (
	"log/slog"
	"os"

	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"unstable.build/rune/internal/debug"
)

func main() {
	debug.StartPProfOnSignal()

	ext, metadata := NewExtension()
	if err := extensionapi.ServeWorkspaceExtension(ext, metadata); err != nil {
		slog.Error("serve extension", "error", err)
		os.Exit(1)
	}
}
