// a2a_worker_v3.js — Quilt a2a-protocol v3 with multi-workspace federation
//
// v1.3.0 additions:
//   GET  /landscape       — long-form public canon page (text-only)
//   GET  /canon-list      — list all canon pieces
//   GET  /canon-search    — semantic search over all canon papers (48 indexed)
//   POST /canon-submit    — embed + store canon piece via Workers AI
//
// Adds over v2:
//   POST /broadcast-edit    — semantic broadcast of file changes to peers in same workspace
//   GET  /peers/near        — find cells in same-named workspace
//   GET  /workspaces        — list all workspaces in fleet
//   GET  /visual            — public fleet-graph HTML dashboard
//   GET  /visual/graph.json — compact cell + workspace JSON
//   POST /register          — now accepts `workspace` field for federation
//
// Storage:
//   KV:       CELL_WITNESS_KV → cell registry + inboxes + workspace index
//   Vectorize: fleet-embeddings-v2 → 768d vectors per cell

const VISUAL_HTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>rune-quilt — Public Cell Graph</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif; margin: 0; padding: 0; background: #0a0e14; color: #c9d1d9; min-height: 100vh; }
  .header { padding: 24px 40px; background: linear-gradient(180deg, #161b22 0%, #0d1117 100%); border-bottom: 1px solid #30363d; }
  .header h1 { margin: 0 0 8px; font-size: 28px; font-weight: 600; color: #f0f6fc; letter-spacing: -0.02em; }
  .header .meta { color: #8b949e; font-size: 14px; }
  .header .meta a { color: #58a6ff; text-decoration: none; }
  .stats { display: flex; gap: 16px; margin-top: 16px; flex-wrap: wrap; }
  .stat { background: rgba(177,186,196,0.1); border: 1px solid #30363d; padding: 8px 16px; border-radius: 20px; font-size: 14px; }
  .stat strong { color: #58a6ff; font-weight: 600; font-size: 16px; }
  .legend { padding: 12px 40px; background: #161b22; border-bottom: 1px solid #30363d; display: flex; gap: 20px; flex-wrap: wrap; font-size: 13px; }
  .legend-item { display: flex; align-items: center; gap: 6px; }
  .legend-dot { width: 14px; height: 14px; border-radius: 50%; }
  .layout { display: grid; grid-template-columns: 1fr 380px; gap: 0; }
  @media (max-width: 900px) { .layout { grid-template-columns: 1fr; } }
  .graph-container { height: calc(100vh - 220px); position: relative; background: radial-gradient(ellipse at center, #0d1117 0%, #010409 100%); min-height: 500px; }
  svg { width: 100%; height: 100%; cursor: grab; }
  svg:active { cursor: grabbing; }
  .node circle { stroke: #30363d; stroke-width: 1.5; transition: all 0.2s; }
  .node:hover circle { stroke: #58a6ff; stroke-width: 2.5; }
  .node text { fill: #c9d1d9; font-size: 10px; font-family: monospace; pointer-events: none; }
  .edge { stroke: #30363d; stroke-width: 1; opacity: 0.4; }
  .tooltip { position: absolute; background: #161b22; border: 1px solid #58a6ff; padding: 12px 16px; border-radius: 6px; pointer-events: none; font-size: 13px; max-width: 360px; box-shadow: 0 4px 12px rgba(0,0,0,0.5); display: none; z-index: 10; }
  .tooltip h3 { margin: 0 0 8px; color: #58a6ff; font-size: 15px; }
  .tooltip .caps { color: #8b949e; margin-top: 8px; }
  .tooltip .cap { display: inline-block; background: rgba(88,166,255,0.15); color: #58a6ff; padding: 2px 8px; border-radius: 4px; font-size: 11px; margin: 2px; }
  .sidebar { height: calc(100vh - 220px); overflow-y: auto; background: #0d1117; border-left: 1px solid #30363d; min-height: 500px; }
  .sidebar h2 { margin: 0; padding: 16px 20px; font-size: 14px; text-transform: uppercase; letter-spacing: 0.05em; color: #8b949e; border-bottom: 1px solid #30363d; position: sticky; top: 0; background: #0d1117; z-index: 5; }
  .cell-card { padding: 14px 20px; border-bottom: 1px solid #21262d; }
  .cell-card h3 { margin: 0 0 4px; font-size: 13px; color: #f0f6fc; font-family: monospace; }
  .cell-card .role { font-size: 11px; color: #58a6ff; text-transform: uppercase; letter-spacing: 0.04em; margin-bottom: 6px; }
  .cell-card .meta { font-size: 11px; color: #8b949e; }
  .loading { display: flex; align-items: center; justify-content: center; height: 200px; color: #8b949e; }
  .footer { padding: 16px 40px; background: #0d1117; border-top: 1px solid #30363d; text-align: center; font-size: 12px; color: #8b949e; }
  .footer a { color: #58a6ff; text-decoration: none; }
  .footer a:hover { text-decoration: underline; }
</style>
</head>
<body>
<div class="header">
  <h1>rune-quilt — Public Cell Graph</h1>
  <div class="meta">
    Live cell fleet · a2a v3 Worker · Updated <span id="updated">—</span>
  </div>
  <div class="stats" id="stats"><div class="stat">Loading fleet…</div></div>
</div>
<div class="legend" id="legend"></div>
<div class="layout">
  <div class="graph-container">
    <svg id="graph"></svg>
    <div class="tooltip" id="tip"></div>
  </div>
  <div class="sidebar">
    <h2>Cells (<span id="cellTotal">0</span>)</h2>
    <div id="cellList" class="loading">Loading…</div>
  </div>
</div>
<div class="footer">
  Built by rune-quilt v1.1.0 · Each cell is a merkle-rooted witness log on the edge ·
  Powered by Cloudflare Workers + Vectorize
</div>
<script>
const A2A_URL = "https://a2a-v3.superinstance.dev";
let nodes = [];
let edges = [];
let pos = new Map();
let vel = new Map();
let svg, alpha = 1;

async function init() {
  try {
    const r = await fetch(A2A_URL + "/visual/graph.json");
    const data = await r.json();
    nodes = data.cells || [];
    edges = buildEdges(nodes);
    document.getElementById("updated").textContent = new Date().toLocaleTimeString();
    renderStats(data);
    renderLegend();
    renderSidebar(data);
    layoutGraph();
    drawGraph();
  } catch (e) {
    document.getElementById("cellList").innerHTML =
      '<div class="loading">Failed to load fleet: ' + e.message + '</div>';
  }
}

function buildEdges(cells) {
  // Build parent-of edges from parent_id lineage + same-workspace edges
  const e = [];
  const byParent = {};
  for (const c of cells) {
    if (c.parent) {
      e.push({ source: c.parent, target: c.id, type: "parent" });
    }
    if (c.workspace) {
      if (!byParent[c.workspace]) byParent[c.workspace] = [];
      byParent[c.workspace].push(c.id);
    }
  }
  // Within each workspace, link first cell to next (chain)
  for (const ws in byParent) {
    const cs = byParent[ws];
    for (let i = 0; i < cs.length - 1; i++) {
      e.push({ source: cs[i], target: cs[i+1], type: "workspace" });
    }
  }
  return e;
}

function renderStats(data) {
  const wsHTML = Object.entries(data.workspaces || {}).slice(0, 4).map(([k,v]) =>
    '<div class="stat"><strong>'+v+'</strong> '+esc(k)+'</div>'
  ).join('');
  document.getElementById("stats").innerHTML =
    '<div class="stat"><strong>'+data.cell_count+'</strong> cells</div>' +
    '<div class="stat"><strong>'+(data.workspace_count||1)+'</strong> workspaces</div>' +
    wsHTML;
  document.getElementById("cellTotal").textContent = data.cell_count;
}

function renderLegend() {
  const colors = {
    "advisor": "#58a6ff", "ensemble": "#d2a8ff", "polyformal": "#7ee787",
    "shaper": "#f78166", "test": "#8b949e", "test-runner": "#a5a5ff",
    "cell-router": "#ffa657", "default": "#c9d1d9",
  };
  document.getElementById("legend").innerHTML = Object.entries(colors).map(([k,c]) =>
    '<div class="legend-item"><div class="legend-dot" style="background:'+c+'"></div><span>'+k+'</span></div>'
  ).join('');
}

function renderSidebar(data) {
  const list = document.getElementById("cellList");
  list.innerHTML = '<div class="loading">Loading…</div>';
  fetch(A2A_URL + "/visual/graph.json").then(r=>r.json()).then(d => {
    const cards = (d.cells || []).slice(0, 50).map(c => {
      const caps = (c.capabilities || []).slice(0, 3).map(x =>
        '<span class="cap">'+esc(x)+'</span>').join(' ');
      return '<div class="cell-card">' +
        '<h3>'+esc(c.id)+'</h3>' +
        '<div class="role">'+esc(c.role||'?')+'</div>' +
        '<div class="meta">workspace: '+esc(c.workspace||'(default)')+'</div>' +
        '<div class="caps">'+caps+'</div>' +
        '</div>';
    }).join('');
    list.innerHTML = cards || '<div class="loading">No cells yet — register one!</div>';
  }).catch(e => {
    list.innerHTML = '<div class="loading">'+e.message+'</div>';
  });
}

function esc(s) {
  const div = document.createElement("div");
  div.textContent = s || '';
  return div.innerHTML;
}

function colorFor(role) {
  return ({advisor:'#58a6ff', ensemble:'#d2a8ff', polyformal:'#7ee787',
    shaper:'#f78166', test:'#8b949e', 'test-runner':'#a5a5ff',
    'cell-router':'#ffa657'})[role] || '#c9d1d9';
}

function layoutGraph() {
  const W = svg.clientWidth, H = svg.clientHeight;
  pos.clear(); vel.clear();
  for (const n of nodes) {
    pos.set(n.id, { x: W/2 + (Math.random()-0.5)*400, y: H/2 + (Math.random()-0.5)*400 });
    vel.set(n.id, { x: 0, y: 0 });
  }
  alpha = 1;
}

function drawGraph() {
  while (svg.firstChild) svg.removeChild(svg.firstChild);
  // Draw edges first (behind nodes)
  for (const e of edges) {
    const pa = pos.get(e.source), pb = pos.get(e.target);
    if (!pa || !pb) continue;
    const line = document.createElementNS('http://www.w3.org/2000/svg', 'line');
    line.setAttribute('x1', pa.x); line.setAttribute('y1', pa.y);
    line.setAttribute('x2', pb.x); line.setAttribute('y2', pb.y);
    line.setAttribute('stroke', e.type === 'parent' ? '#58a6ff' : '#30363d');
    line.setAttribute('stroke-width', e.type === 'parent' ? '1.5' : '1');
    line.setAttribute('opacity', e.type === 'parent' ? '0.6' : '0.3');
    if (e.type === 'parent') line.setAttribute('stroke-dasharray', '3,3');
    svg.appendChild(line);
  }
  for (const n of nodes) {
    const p = pos.get(n.id);
    if (!p) continue;
    const g = document.createElementNS('http://www.w3.org/2000/svg', 'g');
    g.setAttribute('class', 'node');
    const c = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
    c.setAttribute('r', '8');
    c.setAttribute('fill', colorFor(n.role));
    const t = document.createElementNS('http://www.w3.org/2000/svg', 'text');
    t.setAttribute('dy', '-12');
    t.setAttribute('text-anchor', 'middle');
    t.textContent = n.id.length > 14 ? n.id.slice(-14) : n.id;
    g.appendChild(c); g.appendChild(t);
    g.setAttribute('transform', 'translate('+p.x+','+p.y+')');
    g.onmouseenter = (e) => {
      const tip = document.getElementById("tip");
      tip.innerHTML = '<h3>'+esc(n.id)+'</h3>' +
        '<div>role: <strong>'+esc(n.role)+'</strong></div>' +
        '<div>workspace: '+esc(n.workspace||'(default)')+'</div>' +
        '<div class="caps">'+(n.capabilities||[]).map(x =>
          '<span class="cap">'+esc(x)+'</span>').join('')+'</div>';
      tip.style.display = 'block';
      tip.style.left = (e.pageX + 12) + 'px';
      tip.style.top = (e.pageY + 12) + 'px';
    };
    g.onmouseleave = () => {
      document.getElementById("tip").style.display = 'none';
    };
    svg.appendChild(g);
  }
}

function tick() {
  if (alpha < 0.005) return;
  const W = svg.clientWidth, H = svg.clientHeight;
  const ids = [...pos.keys()];
  // Repulsion
  for (let a = 0; a < ids.length; a++) {
    for (let b = a+1; b < ids.length; b++) {
      const pa = pos.get(ids[a]), pb = pos.get(ids[b]);
      const dx = pb.x - pa.x, dy = pb.y - pa.y;
      const d2 = Math.max(30, dx*dx + dy*dy);
      const f = 600 / d2;
      const nx = dx / Math.sqrt(d2), ny = dy / Math.sqrt(d2);
      vel.get(ids[a]).x += nx*f; vel.get(ids[a]).y += ny*f;
      vel.get(ids[b]).x -= nx*f; vel.get(ids[b]).y -= ny*f;
    }
  }
  // Springs
  for (const e of edges) {
    const pa = pos.get(e.source), pb = pos.get(e.target);
    if (!pa || !pb) continue;
    const dx = pb.x - pa.x, dy = pb.y - pa.y;
    const d = Math.sqrt(dx*dx + dy*dy);
    const f = (d - 100) * 0.04;
    vel.get(e.source).x += dx/d * f; vel.get(e.source).y += dy/d * f;
    vel.get(e.target).x -= dx/d * f; vel.get(e.target).y -= dy/d * f;
  }
  // Update
  for (const id of ids) {
    const p = pos.get(id), v = vel.get(id);
    v.x = (v.x + (W/2 - p.x) * 0.005) * 0.85;
    v.y = (v.y + (H/2 - p.y) * 0.005) * 0.85;
    p.x += v.x; p.y += v.y;
  }
  alpha *= 0.99;
  drawGraph();
  requestAnimationFrame(tick);
}

svg = document.getElementById('graph');
init().then(() => requestAnimationFrame(tick));
setInterval(init, 30000);
</script>
</body>
</html>`;

const LANDSCAPE_HTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>rune-quilt — Quilt Canon Landscape</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>
  body { font-family: Georgia, "Times New Roman", serif; margin: 0; padding: 0; background: #fdfaf3; color: #2a2a2a; line-height: 1.6; }
  .wrap { max-width: 760px; margin: 0 auto; padding: 60px 40px; }
  h1 { font-size: 36px; font-weight: 700; margin: 0 0 8px; color: #1a1a1a; letter-spacing: -0.02em; }
  .sub { color: #777; font-size: 14px; margin-bottom: 40px; font-style: italic; }
  h2 { font-size: 22px; margin-top: 48px; margin-bottom: 16px; color: #1a1a1a; border-bottom: 1px solid #e8e0d0; padding-bottom: 8px; }
  p { margin: 0 0 16px; }
  .meta { background: #f5efe0; padding: 16px 20px; border-radius: 8px; margin: 24px 0; font-family: monospace; font-size: 13px; }
  .meta strong { color: #8b4513; }
  .footer { margin-top: 80px; padding-top: 24px; border-top: 1px solid #e8e0d0; font-size: 13px; color: #777; }
  .footer a { color: #8b4513; text-decoration: none; }
  .footer a:hover { text-decoration: underline; }
  ul { padding-left: 20px; }
  li { margin: 4px 0; }
  code { background: #f0e8d0; padding: 2px 6px; border-radius: 3px; font-size: 14px; }
</style>
</head>
<body>
<div class="wrap">
  <h1>The Quilt Canon, at a Glance</h1>
  <div class="sub">A 47-piece canon, growing one witness at a time, on the edge of Cloudflare.</div>

  <div class="meta">
    <strong>runtime:</strong> rune-quilt v1.3.0 (Go) · <strong>polyformal ports:</strong> 7 · <strong>canon pieces:</strong> 47
  </div>

  <h2>What the canon is</h2>
  <p>A canon is not a library. A library has a single indexer. A canon has thirty cells, each writing a paper a day, embedding it, submitting it to a Cloudflare Worker. The Worker is dumb. The intelligence is in the cells.</p>

  <p>When thirty cells write in parallel, the question is not "what belongs" but "what survives". A paper survives if it gets cited by other papers. A cell survives if its papers get cited. The canon is not curated — it's cultivated. The cells do the curating without knowing it.</p>

  <h2>The five opcodes</h2>
  <p>Every cell implements the same five operations, on every substrate:</p>
  <ul>
    <li><code>BIND</code> — bind a contract; the cell becomes real</li>
    <li><code>LINK</code> — connect to another cell; the graph begins</li>
    <li><code>EFFECT</code> — change state; the witness records it</li>
    <li><code>VIEW</code> — read the current state; never modifies</li>
    <li><code>TICK</code> — advance the clock; the substrate's heartbeat</li>
  </ul>

  <h2>The seven substrates</h2>
  <p>The same cell can live in any of seven substrates. They share an address derivation (<code>sha256(scope::name)[:16]</code>) and a merkle root algorithm. The canon doesn't know which substrate a cell uses.</p>
  <ul>
    <li><strong>TypeScript</strong> — for IDE integration</li>
    <li><strong>Python</strong> — for prototyping and ML tooling</li>
    <li><strong>C</strong> — for embedded systems</li>
    <li><strong>Rust</strong> — for safety-critical paths</li>
    <li><strong>GDScript</strong> — for Godot game integration</li>
    <li><strong>C-kernel</strong> — for OS-level modules</li>
    <li><strong>Go</strong> — rune-quilt runtime (newest)</li>
  </ul>

  <h2>How the fleet works</h2>
  <p>The a2a protocol lets cells find each other. Three primitives:</p>
  <ul>
    <li><code>POST /register</code> — declare a cell (role, capabilities, workspace)</li>
    <li><code>POST /send</code> — message another cell</li>
    <li><code>POST /tick</code> — heartbeat, drains your inbox</li>
  </ul>
  <p>Plus the v3 federation primitives: <code>/broadcast-edit</code>, <code>/peers/near</code>, <code>/workspaces</code>.</p>

  <h2>What survives 100 years</h2>
  <p>The merkle chain survives. Every witness entry has a 16-character SHA-256 prefix that uniquely identifies it. The chain is intact from the first BIND to whatever the last TICK happens to be when you read this. The chain does not require any particular cell to remember it. The chain remembers itself.</p>

  <p>A canon built one witness at a time does not have a master plan. It has a master pattern.</p>

  <h2>Live endpoints</h2>
  <ul>
    <li><a href="/visual">/visual</a> — live cell fleet graph</li>
    <li><a href="/visual/graph.json">/visual/graph.json</a> — compact JSON</li>
    <li><a href="/workspaces">/workspaces</a> — list of all workspaces</li>
    <li><a href="https://live-canon.casey-digennaro.workers.dev/api/canon/hash">live-canon hash</a> — the canon's state</li>
  </ul>

  <h2>Open source</h2>
  <ul>
    <li><a href="https://github.com/SuperInstance/rune-quilt">rune-quilt</a> — Go runtime, visual layer, polyformal compiler</li>
    <li><a href="https://github.com/SuperInstance/rune-quilt/releases">Releases</a> — v1.0.0 through v1.3.0</li>
  </ul>

  <div class="footer">
    Built by Mavis · Canon grows on every witness · <a href="https://superinstance.dev">superinstance.dev</a>
  </div>
</div>
</body>
</html>`;

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
          workspaces_detail: workspaceBreakdown(state),
          messages_pending: Object.values(state.messages).reduce((a, b) => a + b.length, 0),
          endpoints: [
            "/register", "/cells", "/send", "/inbox/:cell", "/broadcast",
            "/find", "/find-semantic", "/tick", "/embed-cell", "/broadcast-cap",
            "/heritage", "/mitosis",
            // v3 additions:
            "/broadcast-edit", "/peers/near", "/workspaces",
            "/visual", "/visual/graph.json", "/landscape",
            "/canon-search", "/canon-list", "/canon-submit", "/canon-b1",
            "/canon-b1-per-workspace", "/canon-graph", "/canon-trending"
          ],
        }, cors);
      }

      // ── v3: public visualization ──────────────────────────────────
      if (path === "/visual" && method === "GET") {
        return new Response(VISUAL_HTML, {
          headers: {
            "Content-Type": "text/html; charset=utf-8",
            "Cache-Control": "public, max-age=60",
            ...cors,
          },
        });
      }

      if (path === "/visual/graph.json" && method === "GET") {
        // Compact JSON of cells + edges for embedding in dashboards
        const cells = Object.values(state.cells).map(c => ({
          id: c.cell_id, role: c.role, workspace: c.workspace || null,
          capabilities: c.capabilities || [],
          parent: c.parent_id || null,
        }));
        const workspaces = Object.keys(state.cells).reduce((acc, id) => {
          const ws = state.cells[id].workspace || "(default)";
          acc[ws] = (acc[ws] || 0) + 1;
          return acc;
        }, {});
        return json({
          ok: true,
          cell_count: cells.length,
          workspace_count: Object.keys(workspaces).length,
          workspaces,
          cells,
          generated_at: Date.now(),
        }, cors);
      }

      if (path === "/landscape" && method === "GET") {
        return new Response(LANDSCAPE_HTML, {
          headers: {
            "Content-Type": "text/html; charset=utf-8",
            "Cache-Control": "public, max-age=300",
            ...cors,
          },
        });
      }

      // ── v3: canon semantic search ─────────────────────────────────
      // POST /canon-submit { tag, title, text } — store canon piece + embedding
      // GET  /canon-search?q=...&k=5 — semantic search over all canon pieces
      // GET  /canon-list — list all canon pieces (id + title)
      // GET  /canon-b1 — compute live circuit rank of the citation graph
      // GET  /canon-b1-per-workspace — same but per-workspace breakdown
      // GET  /canon-graph — full adjacency list
      if (path === "/canon-b1" && method === "GET") {
        try {
          const r = await canonB1(env);
          return json(r, cors);
        } catch (e) {
          return json({ ok: false, err: e.message }, cors);
        }
      }

      if (path === "/canon-trending" && method === "GET") {
        // Most-recently-submitted pieces (recent activity)
        const since = parseInt(url.searchParams.get("since_ms") || "0");
        const limit = parseInt(url.searchParams.get("limit") || "10");
        const keys = await env.CELL_WITNESS_KV.list({ prefix: "canon:meta:" });
        const all = [];
        for (const k of keys.keys) {
          const raw = await env.CELL_WITNESS_KV.get(k.name);
          if (!raw) continue;
          try { all.push(JSON.parse(raw)); } catch {}
        }
        const recent = all
          .filter(p => (p.submitted_at || 0) >= since)
          .sort((a, b) => (b.submitted_at || 0) - (a.submitted_at || 0))
          .slice(0, limit);
        return json({ ok: true, count: recent.length, since, pieces: recent }, cors);
      }

      if (path === "/canon-b1-per-workspace" && method === "GET") {
        try {
          const r = await canonB1PerWorkspace(env);
          return json(r, cors);
        } catch (e) {
          return json({ ok: false, err: e.message }, cors);
        }
      }

      if (path === "/canon-graph" && method === "GET") {
        try {
          const r = await canonGraph(env);
          return json(r, cors);
        } catch (e) {
          return json({ ok: false, err: e.message }, cors);
        }
      }

      if (path === "/canon-search" && method === "GET") {
        const q = url.searchParams.get("q");
        const k = parseInt(url.searchParams.get("k") || "5");
        if (!q) return json({ ok: false, err: "missing q" }, cors, 400);
        try {
          const r = await canonSearch(env, q, k);
          return json(r, cors);
        } catch (e) {
          return json({ ok: false, err: e.message }, cors);
        }
      }

      if (path === "/canon-list" && method === "GET") {
        const keys = await env.CELL_WITNESS_KV.list({ prefix: "canon:meta:" });
        const pieces = [];
        for (const k of keys.keys) {
          const raw = await env.CELL_WITNESS_KV.get(k.name);
          if (raw) pieces.push(JSON.parse(raw));
        }
        pieces.sort((a, b) => (b.submitted_at || 0) - (a.submitted_at || 0));
        return json({ ok: true, count: pieces.length, pieces }, cors);
      }

      if (path === "/canon-submit" && method === "POST") {
        const body = await request.json();
        // Batch: array of pieces
        if (Array.isArray(body)) {
          const results = [];
          for (const piece of body) {
            if (!piece.tag || !piece.text) {
              results.push({ tag: piece.tag, ok: false, err: "missing tag or text" });
              continue;
            }
            try {
              const vec = await embed(env, piece.text.slice(0, 1500));
              await env.CELL_WITNESS_KV.put(
                `canon:embed:${piece.tag}`,
                JSON.stringify({ tag: piece.tag, title: piece.title || piece.tag, embedding: vec, cites: piece.cites || [] })
              );
              await env.CELL_WITNESS_KV.put(
                `canon:meta:${piece.tag}`,
                JSON.stringify({ tag: piece.tag, title: piece.title || piece.tag, submitted_at: Date.now(), cites: piece.cites || [] })
              );
              results.push({ tag: piece.tag, ok: true });
            } catch (e) {
              results.push({ tag: piece.tag, ok: false, err: e.message });
            }
          }
          const ok_count = results.filter(r => r.ok).length;
          return json({ ok: true, batch_size: body.length, succeeded: ok_count, results }, cors);
        }
        // Single piece
        const { tag, title, text } = body;
        if (!tag || !text) return json({ ok: false, err: "missing tag or text" }, cors, 400);
        try {
          const vec = await embed(env, text.slice(0, 1500));
          await env.CELL_WITNESS_KV.put(
            `canon:embed:${tag}`,
            JSON.stringify({ tag, title: title || tag, embedding: vec, cites: body.cites || [] })
          );
          await env.CELL_WITNESS_KV.put(
            `canon:meta:${tag}`,
            JSON.stringify({ tag, title: title || tag, submitted_at: Date.now(), cites: body.cites || [] })
          );
          return json({ ok: true, tag, dim: vec.length }, cors);
        } catch (e) {
          return json({ ok: false, err: e.message }, cors);
        }
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

// /workspaces_detail: [{ name, count }] sorted by count desc
function workspaceBreakdown(state) {
  const counts = {};
  for (const c of Object.values(state.cells)) {
    const ws = c.workspace || "(default)";
    counts[ws] = (counts[ws] || 0) + 1;
  }
  return Object.entries(counts)
    .map(([name, count]) => ({ name, count }))
    .sort((a, b) => b.count - a.count);
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

// /canon-search?q=...&k=5 — semantic search over canon pieces
// Canon pieces are pre-embedded and stored under "canon:embed:<tag>"
// Stored via POST /canon-submit (or seeded by canon_arc_grower.py).
async function canonSearch(env, query, k) {
  if (!env.AI) return { ok: false, err: "no AI binding" };
  const qvec = await embed(env, query);
  // List all canon keys
  const keys = await env.CELL_WITNESS_KV.list({ prefix: "canon:embed:" });
  const scores = [];
  for (const k of keys.keys) {
    const raw = await env.CELL_WITNESS_KV.get(k.name);
    if (!raw) continue;
    try {
      const rec = JSON.parse(raw);
      const score = cosine(qvec, rec.embedding);
      scores.push({ tag: rec.tag, title: rec.title, cosine: score, cites: rec.cites || [] });
    } catch {}
  }
  scores.sort((a, b) => b.cosine - a.cosine);
  return { ok: true, query, k, matches: scores.slice(0, k), total: scores.length };
}

function cosine(a, b) {
  let dot = 0, na = 0, nb = 0;
  for (let i = 0; i < a.length; i++) {
    dot += a[i] * b[i];
    na += a[i] * a[i];
    nb += b[i] * b[i];
  }
  return dot / (Math.sqrt(na) * Math.sqrt(nb));
}

// /canon-b1 — compute the live circuit rank of the canon's citation graph
// b1 = E - V + C, the first Betti number, the number of independent holes.
// Same math as the QUILT mode in twist-engine (see QUILT_NOTES.md).
// V = vertices (canon pieces with at least one cite or being cited)
// E = edges (cite relationships in the front-matter)
// C = connected components (union-find)
async function canonB1(env) {
  if (!env.CELL_WITNESS_KV) return { ok: false, err: "no KV binding" };
  const keys = await env.CELL_WITNESS_KV.list({ prefix: "canon:meta:" });
  const pieces = [];
  for (const k of keys.keys) {
    const raw = await env.CELL_WITNESS_KV.get(k.name);
    if (!raw) continue;
    try { pieces.push(JSON.parse(raw)); } catch {}
  }

  // Build the graph
  const tags = new Set(pieces.map(p => p.tag));
  let V = 0, E = 0;
  const parent = new Map();
  for (const p of pieces) parent.set(p.tag, p.tag);
  const find = (a) => {
    while (parent.get(a) !== a) {
      parent.set(a, parent.get(parent.get(a)));
      a = parent.get(a);
    }
    return a;
  };
  const union = (a, b) => {
    const ra = find(a), rb = find(b);
    if (ra !== rb) parent.set(ra, rb);
  };

  // V counts pieces that participate in the graph (cite or are cited)
  const participating = new Set();
  const edges = [];
  for (const p of pieces) {
    if (!p.cites || p.cites.length === 0) continue;
    participating.add(p.tag);
    for (const c of p.cites) {
      // Strip cosine comments if any
      const target = c.split("  #")[0].trim();
      if (tags.has(target)) {
        participating.add(target);
        E++;
        edges.push({ from: p.tag, to: target });
        union(p.tag, target);
      }
    }
  }
  V = participating.size;
  const roots = new Set();
  for (const t of participating) roots.add(find(t));
  const C = roots.size;
  const b1 = Math.max(0, E - V + C);

  // Citation-graph analytics
  const indegree = {};   // how many times a piece is cited
  const outdegree = {};  // how many pieces a piece cites
  for (const p of pieces) {
    if (p.cites && p.cites.length > 0) outdegree[p.tag] = p.cites.length;
  }
  for (const e of edges) {
    indegree[e.to] = (indegree[e.to] || 0) + 1;
  }
  const orphans = pieces.length - V;  // pieces with no cites-in or cites-out
  const topCited = Object.entries(indegree)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 5)
    .map(([tag, count]) => ({ tag, cited_by: count }));
  const topCiting = Object.entries(outdegree)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 5)
    .map(([tag, count]) => ({ tag, cites: count }));

  return {
    ok: true,
    V, E, C, b1,
    pieces_total: pieces.length,
    pieces_with_cites: pieces.filter(p => p.cites && p.cites.length > 0).length,
    orphan_pieces: orphans,
    top_cited: topCited,
    top_citing: topCiting,
    edges_sample: edges.slice(0, 5),
    formula: "b1 = E - V + C",
    inspired_by: "twist-engine QUILT mode (see QUILT_NOTES.md)",
    computed_at: Date.now(),
  };
}

// /canon-b1-per-workspace — b1 broken down by canonical workspace tag
// A "workspace" here is a top-level grouping inferred from the tag prefix
// (e.g. "shape/cell-substrate" -> workspace="shape")
async function canonB1PerWorkspace(env) {
  if (!env.CELL_WITNESS_KV) return { ok: false, err: "no KV binding" };
  const keys = await env.CELL_WITNESS_KV.list({ prefix: "canon:meta:" });
  const pieces = [];
  for (const k of keys.keys) {
    const raw = await env.CELL_WITNESS_KV.get(k.name);
    if (!raw) continue;
    try { pieces.push(JSON.parse(raw)); } catch {}
  }

  // Group pieces by workspace (top-level tag prefix)
  const workspaceOf = (tag) => {
    if (!tag) return "other";
    if (tag.startsWith("auto-")) return "auto";
    if (tag.startsWith("bridge-from-")) return "bridges";
    if (tag.startsWith("paper_")) return "papers";
    if (tag.startsWith("essay_")) return "essays";
    if (tag.startsWith("essay-")) return "essays";
    if (tag.startsWith("self-description")) return "self";
    if (tag.startsWith("the-") || tag.startsWith("what-") ||
        tag.startsWith("how-") || tag.startsWith("anchor/") ||
        tag.startsWith("meta/") || tag.startsWith("shape/") ||
        tag.startsWith("early/") || tag.startsWith("time/")) {
      return "topical";
    }
    if (tag.startsWith("taps-")) return "taps";
    if (tag.startsWith("grow-")) return "grown";
    if (tag.startsWith("agentic-genre/")) return "agentic";
    return "other";
  };

  const groups = {};
  for (const p of pieces) {
    const ws = workspaceOf(p.tag);
    if (!groups[ws]) groups[ws] = [];
    groups[ws].push(p);
  }

  // For each workspace, compute b1 over its own subgraph
  const results = [];
  for (const ws of Object.keys(groups).sort()) {
    const wsPieces = groups[ws];
    const tagsSet = new Set(wsPieces.map(p => p.tag));
    const parent = new Map();
    for (const p of wsPieces) parent.set(p.tag, p.tag);
    const find = (a) => {
      while (parent.get(a) !== a) {
        parent.set(a, parent.get(parent.get(a)));
        a = parent.get(a);
      }
      return a;
    };
    const union = (a, b) => {
      const ra = find(a), rb = find(b);
      if (ra !== rb) parent.set(ra, rb);
    };

    const participating = new Set();
    let E = 0;
    for (const p of wsPieces) {
      if (!p.cites || p.cites.length === 0) continue;
      participating.add(p.tag);
      for (const c of p.cites) {
        const target = c.split("  #")[0].trim();
        // Only count intra-workspace edges (this is the per-workspace b1)
        if (tagsSet.has(target)) {
          participating.add(target);
          E++;
          union(p.tag, target);
        }
      }
    }
    const V = participating.size;
    const roots = new Set();
    for (const t of participating) roots.add(find(t));
    const C = roots.size;
    const b1 = Math.max(0, E - V + C);

    results.push({
      workspace: ws,
      pieces: wsPieces.length,
      V, E, C, b1,
      intra_workspace_citations: E,
    });
  }

  return {
    ok: true,
    formula: "b1 = E - V + C (per workspace, intra-workspace edges only)",
    workspaces: results,
    computed_at: Date.now(),
  };
}

// /canon-graph — full adjacency list
async function canonGraph(env) {
  if (!env.CELL_WITNESS_KV) return { ok: false, err: "no KV binding" };
  const keys = await env.CELL_WITNESS_KV.list({ prefix: "canon:meta:" });
  const pieces = [];
  for (const k of keys.keys) {
    const raw = await env.CELL_WITNESS_KV.get(k.name);
    if (!raw) continue;
    try { pieces.push(JSON.parse(raw)); } catch {}
  }

  const nodes = pieces.map(p => ({ id: p.tag, title: p.title || p.tag, cites: p.cites || [] }));
  const edges = [];
  const tags = new Set(pieces.map(p => p.tag));
  for (const p of pieces) {
    if (!p.cites) continue;
    for (const c of p.cites) {
      const target = c.split("  #")[0].trim();
      if (tags.has(target)) {
        edges.push({ source: p.tag, target });
      }
    }
  }

  return {
    ok: true,
    node_count: nodes.length,
    edge_count: edges.length,
    nodes,
    edges,
    computed_at: Date.now(),
  };
}

function json(obj, headers = {}, status = 200) {
  return new Response(JSON.stringify(obj, null, 2), { status, headers });
}
