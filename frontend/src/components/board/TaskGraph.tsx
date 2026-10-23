import { useEffect, useMemo, useRef, useState } from "react";
import {
  forceSimulation, forceLink, forceManyBody, forceCollide, forceX, forceY,
  type Simulation, type SimulationNodeDatum, type SimulationLinkDatum,
} from "d3-force";
import { type GNode, type GEdge, priorityColor, nodeRadius, loadPositions, savePositions, type Positions } from "../../lib/taskGraph";

type SimNode = SimulationNodeDatum & { key: string; n: GNode };
type SimLink = SimulationLinkDatum<SimNode> & { kind: string };

const trunc = (s: string, n: number) => (s.length > n ? s.slice(0, n - 1) + "…" : s);

/**
 * Force-directed SVG graph (Obsidian-style): pan with the background, zoom with
 * the wheel, drag nodes (they stay pinned where dropped, persisted across
 * refreshes), hover dims everything but the neighbourhood, click selects,
 * double-click on an epic collapses it. No d3-selection: React renders the
 * SVG, d3-force only computes positions.
 */
export function TaskGraph({ nodes, edges, selected, matches, pulse, collapsed, resetToken, onSelect, onToggleCollapse }: {
  nodes: GNode[];
  edges: GEdge[];
  selected: string | null;
  matches: Set<string>;
  pulse: Set<string>;
  collapsed: Set<string>;
  resetToken: number;
  onSelect: (key: string | null) => void;
  onToggleCollapse: (key: string) => void;
}) {
  const svgRef = useRef<SVGSVGElement>(null);
  const simRef = useRef<Simulation<SimNode, SimLink> | null>(null);
  const simNodes = useRef<Map<string, SimNode>>(new Map());
  const [pos, setPos] = useState<Positions>({});
  const [view, setView] = useState({ x: 0, y: 0, k: 1 });
  const [hover, setHover] = useState<string | null>(null);
  const [size, setSize] = useState({ w: 800, h: 520 });
  const drag = useRef<{ key?: string; sx: number; sy: number; ox: number; oy: number; moved: boolean } | null>(null);

  useEffect(() => {
    const el = svgRef.current;
    if (!el) return;
    const upd = () => { const r = el.getBoundingClientRect(); if (r.width && r.height) setSize({ w: r.width, h: r.height }); };
    upd();
    if (typeof ResizeObserver === "undefined") return;
    const ro = new ResizeObserver(upd);
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  const persist = () => {
    const saved = loadPositions();
    for (const s of simNodes.current.values()) if (s.fx != null && s.fy != null) saved[s.key] = { x: s.fx, y: s.fy };
    savePositions(saved);
  };

  // Rebuild the simulation when the node/edge SET changes (not on every
  // re-render): existing nodes keep their coordinates, new ones start on a
  // ring and settle; dragged/saved nodes stay pinned.
  const nodeSig = nodes.map((n) => n.key).join("|");
  const edgeSig = edges.map((e) => `${e.src}>${e.dst}:${e.kind}`).join("|");
  useEffect(() => {
    const saved = loadPositions();
    const prev = simNodes.current;
    const next = new Map<string, SimNode>();
    const list: SimNode[] = nodes.map((n, i) => {
      const old = prev.get(n.key);
      if (old) { old.n = n; next.set(n.key, old); return old; }
      const s = saved[n.key];
      const sn: SimNode = { key: n.key, n, x: s?.x ?? Math.cos(i) * 220, y: s?.y ?? Math.sin(i) * 220 };
      if (s) { sn.fx = s.x; sn.fy = s.y; }
      next.set(n.key, sn);
      return sn;
    });
    simNodes.current = next;
    const links: SimLink[] = edges.map((e) => ({ source: e.src, target: e.dst, kind: e.kind }));
    simRef.current?.stop();
    const sim = forceSimulation<SimNode>(list)
      .force("link", forceLink<SimNode, SimLink>(links).id((d) => d.key)
        .distance((l) => (l.kind === "parent" ? 70 : 130)).strength((l) => (l.kind === "parent" ? 0.7 : 0.15)))
      .force("charge", forceManyBody<SimNode>().strength((d) => (d.n.epic ? -650 : -230)))
      .force("collide", forceCollide<SimNode>((d) => nodeRadius(d.n) + 12))
      .force("x", forceX<SimNode>(0).strength(0.03))
      .force("y", forceY<SimNode>(0).strength(0.03));
    let raf = 0;
    const flush = () => {
      raf = 0;
      const p: Positions = {};
      for (const s of list) p[s.key] = { x: s.x ?? 0, y: s.y ?? 0 };
      setPos(p);
    };
    sim.on("tick", () => { if (!raf) raf = requestAnimationFrame(flush); });
    sim.on("end", flush);
    flush();
    simRef.current = sim;
    return () => { sim.stop(); if (raf) cancelAnimationFrame(raf); };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [nodeSig, edgeSig]);

  // "Reset layout": forget saved positions, unpin everything, re-settle.
  const firstReset = useRef(true);
  useEffect(() => {
    if (firstReset.current) { firstReset.current = false; return; }
    savePositions({});
    for (const s of simNodes.current.values()) { s.fx = null; s.fy = null; }
    setView({ x: 0, y: 0, k: 1 });
    simRef.current?.alpha(1).restart();
  }, [resetToken]);

  const toGraph = (cx: number, cy: number) => {
    const r = svgRef.current?.getBoundingClientRect() ?? { left: 0, top: 0 };
    return { x: (cx - r.left - size.w / 2 - view.x) / view.k, y: (cy - r.top - size.h / 2 - view.y) / view.k };
  };

  const onBgDown = (e: React.PointerEvent) => {
    drag.current = { sx: e.clientX, sy: e.clientY, ox: view.x, oy: view.y, moved: false };
    (e.currentTarget as Element).setPointerCapture?.(e.pointerId);
  };
  const onNodeDown = (key: string) => (e: React.PointerEvent) => {
    e.stopPropagation();
    const sn = simNodes.current.get(key);
    if (!sn) return;
    drag.current = { key, sx: e.clientX, sy: e.clientY, ox: sn.x ?? 0, oy: sn.y ?? 0, moved: false };
    svgRef.current?.setPointerCapture?.(e.pointerId);
    simRef.current?.alphaTarget(0.3).restart();
  };
  const onMove = (e: React.PointerEvent) => {
    const d = drag.current;
    if (!d) return;
    const dx = e.clientX - d.sx, dy = e.clientY - d.sy;
    if (Math.abs(dx) > 3 || Math.abs(dy) > 3) d.moved = true;
    if (d.key) {
      const sn = simNodes.current.get(d.key);
      if (sn) { const g = toGraph(e.clientX, e.clientY); sn.fx = g.x; sn.fy = g.y; }
    } else {
      setView((v) => ({ ...v, x: d.ox + dx, y: d.oy + dy }));
    }
  };
  const onUp = () => {
    const d = drag.current;
    drag.current = null;
    if (!d) return;
    if (d.key) { simRef.current?.alphaTarget(0); persist(); }
    else if (!d.moved) onSelect(null);
  };
  const onWheel = (e: React.WheelEvent) => {
    const f = Math.exp(-e.deltaY * 0.0015);
    setView((v) => {
      const k = Math.min(4, Math.max(0.2, v.k * f));
      const r = svgRef.current?.getBoundingClientRect() ?? { left: 0, top: 0 };
      // keep the graph point under the cursor fixed
      const px = e.clientX - r.left - size.w / 2, py = e.clientY - r.top - size.h / 2;
      return { k, x: px - ((px - v.x) * k) / v.k, y: py - ((py - v.y) * k) / v.k };
    });
  };

  const neighbors = useMemo(() => {
    const m = new Map<string, Set<string>>();
    const add = (a: string, b: string) => { if (!m.has(a)) m.set(a, new Set()); m.get(a)!.add(b); };
    for (const e of edges) { add(e.src, e.dst); add(e.dst, e.src); }
    return m;
  }, [edges]);
  const focus = hover ?? selected;
  const dim = (k: string) =>
    focus ? !(k === focus || neighbors.get(focus)?.has(k)) : matches.size > 0 && !matches.has(k);
  const radii = useMemo(() => new Map(nodes.map((n) => [n.key, nodeRadius(n)])), [nodes]);

  return (
    <svg ref={svgRef} className="tg-svg" data-testid="task-graph"
      onPointerDown={onBgDown} onPointerMove={onMove} onPointerUp={onUp} onPointerLeave={onUp} onWheel={onWheel}>
      <defs>
        <marker id="tg-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
          <path d="M0,0 L10,5 L0,10 z" fill="var(--danger)" />
        </marker>
      </defs>
      <g transform={`translate(${size.w / 2 + view.x},${size.h / 2 + view.y}) scale(${view.k})`}>
        {edges.map((e) => {
          const a = pos[e.src], b = pos[e.dst];
          if (!a || !b) return null;
          // stop the line at the target's rim so arrowheads sit on the circle
          const dx = b.x - a.x, dy = b.y - a.y, len = Math.hypot(dx, dy) || 1;
          const rb = (radii.get(e.dst) ?? 10) + (e.kind === "blocks" ? 7 : 2);
          const x2 = b.x - (dx / len) * rb, y2 = b.y - (dy / len) * rb;
          return (
            <line key={`${e.src}>${e.dst}:${e.kind}`} className={`tg-edge tg-edge--${e.kind}`}
              x1={a.x} y1={a.y} x2={x2} y2={y2}
              markerEnd={e.kind === "blocks" ? "url(#tg-arrow)" : undefined}
              opacity={dim(e.src) || dim(e.dst) ? 0.12 : 1} />
          );
        })}
        {nodes.map((n) => {
          const p = pos[n.key];
          if (!p) return null;
          const r = radii.get(n.key) ?? 10;
          const color = priorityColor(n.priorityRank);
          const showSummary = n.epic || selected === n.key || hover === n.key;
          return (
            <g key={n.key} transform={`translate(${p.x},${p.y})`} data-key={n.key}
              data-blocked={n.blocked || undefined} data-mine={n.mine || undefined}
              className={`tg-node${n.terminal ? " tg-node--done" : ""}${pulse.has(n.key) ? " tg-node--pulse" : ""}`}
              role="button" aria-label={`${n.key} ${n.summary}`} opacity={dim(n.key) ? 0.18 : 1}
              onPointerDown={onNodeDown(n.key)}
              onClick={(e) => { e.stopPropagation(); if (!drag.current?.moved) onSelect(n.key); }}
              onDoubleClick={(e) => { e.stopPropagation(); if (n.epic) onToggleCollapse(n.key); }}
              onPointerEnter={() => setHover(n.key)} onPointerLeave={() => setHover(null)}>
              {selected === n.key && <circle r={r + 9} className="tg-ring-selected" />}
              {n.blocked && <circle r={r + 5} className="tg-ring-blocked" />}
              <circle r={r} fill={n.mine ? color : "var(--surface2)"} stroke={color}
                strokeWidth={n.epic ? 3 : 2} strokeDasharray={collapsed.has(n.key) ? "4 3" : undefined} />
              {n.epic && <text className="tg-epic-mark" textAnchor="middle" dy={4}>{collapsed.has(n.key) ? "+" : "E"}</text>}
              {n.blocked && <text className="tg-lock" textAnchor="middle" dy={-r - 8}>⛔</text>}
              {n.contextKey && (
                <g transform={`translate(${r - 4},${-r - 2})`} className="tg-ctx">
                  <rect width={22} height={11} rx={3} />
                  <text x={11} y={8.5} textAnchor="middle">ctx</text>
                </g>
              )}
              <text className="tg-label" textAnchor="middle" dy={r + 13}>{n.key}</text>
              {showSummary && <text className="tg-summary" textAnchor="middle" dy={r + 26}>{trunc(n.summary, 40)}</text>}
            </g>
          );
        })}
      </g>
    </svg>
  );
}
