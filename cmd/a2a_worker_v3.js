// a2a_worker_v3.js — Quilt a2a-protocol v3 with multi-workspace federation
//
// Adds over v2:
//   POST /broadcast-edit    — semantic broadcast of file changes to peers in same workspace
//   GET  /peers/near        — find cells in same-named workspace
//   GET  /workspaces        — list all workspaces in fleet
//   POST /register          — now accepts `workspace` field for federation
//
// Storage:
//   KV:       CELL_WITNESS_KV → cell registry + inboxes + workspace index
//   Vectorize: fleet-embeddings-v2 → 768d vectors per cell

export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);
    const path = url.pathname;
    const method = request.method;
    const cors = {
      "Access-Control-Allow-Origin": "*",
      "Access-Control-Allow-Methods": "GET, POST, OPTIONS",
      "Access-Control-Allow-Headers": "Content-Type",
      "Content-Type": "application/json",
    };
    if (method === "OPTIONS") return new Response(null, { headers: cors });

    try {
      const state = await getState(env);

      if (path === "/" || path === "") {
        return json({
          service: "quilt-a2a-v3",
          version: "3.0.0",
          features: ["v2-protocol", "vectorize-semantic-search", "cell-heritage", "workspace-federation"],
          cells: Object.keys(state.cells).length,
          workspaces: countWorkspaces(state),
          messages_pending: Object.values(state.messages).reduce((a, b) => a + b.length, 0),
          endpoints: [
            "/register", "/cells", "/send", "/inbox/:cell", "/broadcast",
            "/find", "/find-semantic", "/tick", "/embed-cell", "/broadcast-cap",
            "/heritage", "/mitosis",
            // v3 additions:
            "/broadcast-edit", "/peers/near", "/workspaces"
          ],
        }, cors);
      }

      if (path === "/register" && method === "POST") {
        const body = await request.json();
        const cell = {
          cell_id: body.cell_id || `anon-${Date.now()}`,
          role: body.role || "unknown",
          address: body.address || body.cell_id,
          capabilities: body.capabilities || [],
          parent_id: body.parent_id || null,        // for mitosis/heritage
          lineage: body.lineage || [],              // lineage chain
          workspace: body.workspace || null,        // v3: workspace name for federation
          registered_at: Date.now(),
          last_seen: Date.now(),
          load: body.load || 0,                     // 0..1 for load-based mitosis
        };
        state.cells[cell.cell_id] = cell;
        state.messages[cell.cell_id] = state.messages[cell.cell_id] || [];

        // Auto-embed: if capabilities provided, embed them
        if (body.capabilities && body.capabilities.length > 0 && env.VECTORIZE) {
          try {
            const text = buildCellText(cell);
            const vec = await embed(env, text);
            await env.VECTORIZE.upsert([{
              id: cell.cell_id,
              values: vec,
              metadata: {
                cell_id: cell.cell_id,
                role: cell.role,
                capabilities: cell.capabilities,
                parent_id: cell.parent_id,
                text: text.slice(0, 1000),
              },
            }]);
          } catch (e) {
            console.log(`embed err: ${e.message}`);
          }
        }

        await saveState(env, state);
        return json({
          ok: true, cell,
          total_cells: Object.keys(state.cells).length,
        }, cors);
      }

      if (path === "/cells" && method === "GET") {
        return json({ cells: Object.values(state.cells) }, cors);
      }

      if (path === "/send" && method === "POST") {
        const body = await request.json();
        const { from, to, type, payload } = body;
        if (!to || !type) return json({ ok: false, err: "missing to/type" }, cors);
        const msg = {
          id: crypto.randomUUID(),
          from: from || "anon",
          to, type, payload: payload || {},
          ts: Date.now(),
        };
        state.messages[to] = state.messages[to] || [];
        state.messages[to].push(msg);
        await saveState(env, state);
        return json({ ok: true, msg, inbox_size: state.messages[to].length }, cors);
      }

      if (path.startsWith("/inbox/") && method === "GET") {
        const cell_id = path.split("/")[2];
        const messages = state.messages[cell_id] || [];
        if (url.searchParams.get("drain") !== "false") {
          state.messages[cell_id] = [];
          await saveState(env, state);
        }
        return json({ cell_id, messages, count: messages.length }, cors);
      }

      if (path === "/broadcast" && method === "POST") {
        const body = await request.json();
        const { from, type, payload, capability } = body;
        const targets = capability
          ? Object.values(state.cells).filter(c => c.capabilities?.includes(capability))
          : Object.values(state.cells);
        const msgs = [];
        for (const t of targets) {
          if (t.cell_id === from) continue;
          const msg = {
            id: crypto.randomUUID(),
            from: from || "anon", to: t.cell_id,
            type, payload: payload || {},
            ts: Date.now(), broadcast: true,
          };
          state.messages[t.cell_id] = state.messages[t.cell_id] || [];
          state.messages[t.cell_id].push(msg);
          msgs.push(msg);
        }
        await saveState(env, state);
        return json({ ok: true, delivered: msgs.length, msgs }, cors);
      }

      if (path === "/find" && method === "GET") {
        const cap = url.searchParams.get("cap");
        const role = url.searchParams.get("role");
        let cells = Object.values(state.cells);
        if (cap) cells = cells.filter(c => c.capabilities?.includes(cap));
        if (role) cells = cells.filter(c => c.role === role);
        return json({ cells, count: cells.length }, cors);
      }

      // ---- v2 NEW: semantic capability search via Vectorize ----
      if (path === "/find-semantic" && method === "GET") {
        const q = url.searchParams.get("q") || "";
        const k = parseInt(url.searchParams.get("k") || "5");
        if (!env.VECTORIZE) return json({ err: "no vectorize binding" }, cors);
        try {
          const vec = await embed(env, q);
          const matches = await env.VECTORIZE.query(vec, { topK: k, returnMetadata: "all" });
          return json({
            query: q, k, matches: matches.matches || [],
            count: (matches.matches || []).length,
          }, cors);
        } catch (e) {
          return json({ err: e.message }, cors);
        }
      }

      if (path === "/embed-cell" && method === "POST") {
        const body = await request.json();
        const { cell_id, text } = body;
        if (!cell_id || !text) return json({ ok: false, err: "missing cell_id/text" }, cors);
        if (!env.VECTORIZE) return json({ err: "no vectorize binding" }, cors);
        const cell = state.cells[cell_id];
        if (!cell) return json({ ok: false, err: "unknown cell" }, cors, 404);
        try {
          const vec = await embed(env, text);
          await env.VECTORIZE.upsert([{
            id: cell_id, values: vec,
            metadata: { cell_id, role: cell.role, text: text.slice(0, 1000) },
          }]);
          return json({ ok: true, cell_id, dim: vec.length }, cors);
        } catch (e) {
          return json({ ok: false, err: e.message }, cors);
        }
      }

      if (path === "/broadcast-cap" && method === "POST") {
        // Semantic broadcast: find cells matching a query, then broadcast
        const body = await request.json();
        const { from, query, type, payload, k } = body;
        if (!query) return json({ ok: false, err: "missing query" }, cors);
        if (!env.VECTORIZE) return json({ err: "no vectorize binding" }, cors);
        try {
          const vec = await embed(env, query);
          const matches = await env.VECTORIZE.query(vec, {
            topK: k || 5, returnMetadata: "all",
          });
          const targetIds = (matches.matches || []).map(m => m.id);
          const msgs = [];
          for (const tid of targetIds) {
            if (tid === from) continue;
            const msg = {
              id: crypto.randomUUID(),
              from: from || "anon", to: tid,
              type: type || "EVENT", payload: payload || {},
              ts: Date.now(), broadcast: true, semantic_query: query,
            };
            state.messages[tid] = state.messages[tid] || [];
            state.messages[tid].push(msg);
            msgs.push(msg);
          }
          await saveState(env, state);
          return json({
            ok: true, query, delivered: msgs.length,
            target_ids: targetIds,
          }, cors);
        } catch (e) {
          return json({ ok: false, err: e.message }, cors);
        }
      }

      // ---- Cell heritage: inherit capabilities from parent ----
      if (path === "/heritage" && method === "GET") {
        const cell_id = url.searchParams.get("cell_id");
        const cell = state.cells[cell_id];
        if (!cell) return json({ ok: false, err: "unknown cell" }, cors, 404);
        const parent = cell.parent_id ? state.cells[cell.parent_id] : null;
        const lineage = [];
        let cur = cell;
        while (cur) {
          lineage.push({ cell_id: cur.cell_id, role: cur.role, capabilities: cur.capabilities });
          cur = cur.parent_id ? state.cells[cur.parent_id] : null;
        }
        return json({ cell_id, parent, lineage }, cors);
      }

      // ---- Cell mitosis: spawn a child cell ----
      if (path === "/mitosis" && method === "POST") {
        const body = await request.json();
        const parent_id = body.parent_id;
        const parent = state.cells[parent_id];
        if (!parent) return json({ ok: false, err: "unknown parent" }, cors, 404);
        const child_id = `${parent_id}-${Math.random().toString(36).slice(2, 8)}`;
        const child = {
          cell_id: child_id,
          role: parent.role,
          address: child_id,
          capabilities: [...parent.capabilities],  // inherit all
          parent_id: parent_id,
          lineage: [...(parent.lineage || []), parent_id],
          registered_at: Date.now(),
          last_seen: Date.now(),
          load: 0,
        };
        state.cells[child_id] = child;
        state.messages[child_id] = [];
        // Notify parent
        state.messages[parent_id] = state.messages[parent_id] || [];
        state.messages[parent_id].push({
          id: crypto.randomUUID(),
          from: child_id, to: parent_id,
          type: "MITOSIS",
          payload: { child_id, parent_id, capabilities: child.capabilities },
          ts: Date.now(),
        });
        await saveState(env, state);
        return json({ ok: true, child, parent_id }, cors);
      }

      if (path === "/tick" && method === "POST") {
        const body = await request.json().catch(() => ({}));
        const cell_id = body.cell_id || `anon-${Date.now()}`;
        if (state.cells[cell_id]) {
          state.cells[cell_id].last_seen = Date.now();
          state.cells[cell_id].load = body.load ?? state.cells[cell_id].load;
        } else {
          state.cells[cell_id] = {
            cell_id, role: body.role || "anon",
            address: cell_id, capabilities: body.capabilities || [],
            parent_id: body.parent_id || null,
            lineage: body.lineage || [],
            registered_at: Date.now(), last_seen: Date.now(),
            load: body.load || 0,
          };
        }
        state.messages[cell_id] = state.messages[cell_id] || [];
        const messages = state.messages[cell_id] || [];
        state.messages[cell_id] = [];
        await saveState(env, state);
        return json({
          ok: true, cell_id, total_cells: Object.keys(state.cells).length,
          inbox_count: messages.length, messages,
        }, cors);
      }

      // ── v3: workspace federation ─────────────────────────────────────

      if (path === "/broadcast-edit" && method === "POST") {
        // Broadcast a file-edit notification to all cells in the same workspace
        // as the sender, except the sender itself.
        const body = await request.json();
        const from = body.from || "anon";
        const workspace = body.workspace;
        const file = body.file || "";
        const op = body.op || "edit";
        if (!workspace) {
          return json({ ok: false, err: "missing workspace" }, cors, 400);
        }
        const recipients = [];
        for (const [cid, c] of Object.entries(state.cells)) {
          if (cid === from) continue;
          if (c.workspace !== workspace) continue;
          state.messages[cid] = state.messages[cid] || [];
          const msg = {
            id: `msg-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
            from,
            to: cid,
            type: "file-edit",
            payload: { file, op, workspace, ts: Date.now() },
            received_at: Date.now(),
          };
          state.messages[cid].push(msg);
          recipients.push(cid);
        }
        await saveState(env, state);
        return json({
          ok: true, workspace, recipients: recipients.length,
          recipient_ids: recipients,
        }, cors);
      }

      if (path === "/peers/near" && method === "GET") {
        // Find cells in the same workspace as the given cell.
        const cellID = url.searchParams.get("cell_id");
        if (!cellID) {
          return json({ ok: false, err: "missing cell_id" }, cors, 400);
        }
        const cell = state.cells[cellID];
        if (!cell) {
          return json({ ok: false, err: "cell not found" }, cors, 404);
        }
        const ws = cell.workspace;
        const peers = [];
        for (const [cid, c] of Object.entries(state.cells)) {
          if (cid === cellID) continue;
          if (c.workspace !== ws) continue;
          peers.push(c);
        }
        return json({
          ok: true, cell_id: cellID, workspace: ws,
          peer_count: peers.length, peers,
        }, cors);
      }

      if (path === "/workspaces" && method === "GET") {
        // List all workspaces + counts.
        const counts = {};
        for (const c of Object.values(state.cells)) {
          const ws = c.workspace || "(default)";
          counts[ws] = (counts[ws] || 0) + 1;
        }
        const list = Object.entries(counts).map(([name, count]) => ({
          name, count,
        })).sort((a, b) => b.count - a.count);
        return json({
          ok: true,
          workspace_count: list.length,
          total_cells: Object.keys(state.cells).length,
          workspaces: list,
        }, cors);
      }

      return json({ ok: false, err: "no route", path }, cors, 404);
    } catch (e) {
      return json({ ok: false, err: e.message }, cors, 500);
    }
  },

  async scheduled(event, env, ctx) {
    // Periodic GC: remove cells unseen for >1 hour
    const state = await getState(env);
    const now = Date.now();
    const cutoff = now - 3600_000;
    for (const [id, c] of Object.entries(state.cells)) {
      if (c.last_seen < cutoff) delete state.cells[id];
    }
    await saveState(env, state);
    ctx.waitUntil(Promise.resolve());
  },
};

function countWorkspaces(state) {
  const set = new Set();
  for (const c of Object.values(state.cells)) {
    set.add(c.workspace || "(default)");
  }
  return set.size;
}

function buildCellText(cell) {
  return [
    `Cell ${cell.cell_id} (${cell.role})`,
    cell.parent_id ? `parent: ${cell.parent_id}` : "",
    `capabilities: ${(cell.capabilities || []).join(", ")}`,
    cell.lineage?.length ? `lineage: ${cell.lineage.join(" → ")}` : "",
    `load: ${cell.load || 0}`,
  ].filter(Boolean).join(". ");
}

async function embed(env, text) {
  if (!env.AI) throw new Error("no AI binding");
  const result = await env.AI.run("@cf/baai/bge-base-en-v1.5", {
    text: text.slice(0, 2000),
  });
  return result.data[0];
}

async function getState(env) {
  const raw = await env.CELL_WITNESS_KV.get("a2a:state");
  if (!raw) return { cells: {}, messages: {} };
  try { return JSON.parse(raw); } catch { return { cells: {}, messages: {} }; }
}

async function saveState(env, state) {
  await env.CELL_WITNESS_KV.put("a2a:state", JSON.stringify(state));
}

function json(obj, headers = {}, status = 200) {
  return new Response(JSON.stringify(obj, null, 2), { status, headers });
}
