import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { RefreshCw, ExternalLink, FolderOpen, FolderPlus, Lock, RotateCcw, Search } from "lucide-react";
import { GetTaskGraph, RefreshTaskGraph, ListTaskProviders } from "../../../wailsjs/go/main/App";
import { BrowserOpenURL, EventsOn } from "../../../wailsjs/runtime/runtime";
import { TaskGraph } from "./TaskGraph";
import {
  type GGraph, type GNode, visibleGraph, newlyUnblocked, priorityColor, parentChain,
} from "../../lib/taskGraph";

const OPTS_KEY = "vibecodeway:board:graph:opts";
type Opts = { onlyMine: boolean; hideDone: boolean; showRelates: boolean };
const loadOpts = (): Opts => {
  try { return { onlyMine: true, hideDone: true, showRelates: false, ...JSON.parse(localStorage.getItem(OPTS_KEY) ?? "{}") }; }
  catch { return { onlyMine: true, hideDone: true, showRelates: false }; }
};

type Org = { id: string; name: string };

/**
 * Graph view of my tracker work: the force graph plus a rail with the
 * selected issue, "Ready now" and "Waiting on". Data comes from the Go task
 * graph (provider-neutral); the provider select appears once more than one
 * tracker is registered.
 */
export function TaskGraphView({ onOpenContext, onCreateContext, orgs, selectedOrgId, onSelectOrg, creatingKey, reloadToken, refreshToken = 0, onRankChange }: {
  onOpenContext: (contextId: string, contextKey: string) => void;
  onCreateContext: (key: string) => void | Promise<void>;
  orgs: Org[];
  selectedOrgId: string;
  onSelectOrg: (id: string) => void;
  creatingKey: string;
  reloadToken: number;
  /** Bumped by the page header's refresh button: pull fresh tracker data. */
  refreshToken?: number;
  onRankChange?: (ready: string[], waiting: string[]) => void;
}) {
  const [providers, setProviders] = useState<string[]>(["jira"]);
  const [provider, setProvider] = useState("jira");
  const [graph, setGraph] = useState<GGraph | null>(null);
  const [loading, setLoading] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [progress, setProgress] = useState<{ current: number; total: number; key: string } | null>(null);
  const [error, setError] = useState("");
  const [opts, setOpts] = useState<Opts>(loadOpts);
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<string | null>(null);
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  const [pulse, setPulse] = useState<Set<string>>(new Set());
  const [banner, setBanner] = useState<string[]>([]);
  const [resetToken, setResetToken] = useState(0);
  const prevGraph = useRef<GGraph | null>(null);

  const setOpt = (patch: Partial<Opts>) =>
    setOpts((o) => { const n = { ...o, ...patch }; try { localStorage.setItem(OPTS_KEY, JSON.stringify(n)); } catch { /* ignore */ } return n; });

  const apply = useCallback((g: GGraph) => {
    const unblocked = newlyUnblocked(prevGraph.current, g);
    prevGraph.current = g;
    setGraph(g);
    onRankChange?.(g.ready ?? [], g.waiting ?? []);
    if (unblocked.length) {
      setPulse(new Set(unblocked));
      setBanner(unblocked);
      window.setTimeout(() => { setPulse(new Set()); setBanner([]); }, 8000);
    }
  }, [onRankChange]);

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const g = (await GetTaskGraph(provider)) as unknown as GGraph;
      apply({ ...g, nodes: g.nodes ?? [], edges: g.edges ?? [], ready: g.ready ?? [], waiting: g.waiting ?? [] });
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, [provider, apply]);

  const refresh = useCallback(async (force: boolean) => {
    setRefreshing(true);
    setError("");
    setProgress(null);
    try {
      const g = (await RefreshTaskGraph(provider, force)) as unknown as GGraph;
      apply({ ...g, nodes: g.nodes ?? [], edges: g.edges ?? [], ready: g.ready ?? [], waiting: g.waiting ?? [] });
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setRefreshing(false);
      setProgress(null);
    }
  }, [provider, apply]);

  useEffect(() => {
    try { void ListTaskProviders().then((p) => { if (p?.length) setProviders(p); }).catch(() => {}); } catch { /* binding absent */ }
  }, []);
  useEffect(() => { void load(); }, [load, reloadToken]);
  useEffect(() => { if (refreshToken) void refresh(false); }, [refresh, refreshToken]);
  useEffect(() => {
    try {
      return EventsOn("taskgraph:progress", (p: { current: number; total: number; key: string }) => setProgress(p));
    } catch { return undefined; }
  }, []);

  const view = useMemo(
    () => (graph ? visibleGraph(graph, { ...opts, query, collapsed }) : { nodes: [], edges: [], matches: new Set<string>() }),
    [graph, opts, query, collapsed],
  );
  const byKey = useMemo(() => new Map((graph?.nodes ?? []).map((n) => [n.key, n])), [graph]);
  const sel = selected ? byKey.get(selected) : undefined;
  const toggleCollapse = (key: string) =>
    setCollapsed((c) => { const n = new Set(c); if (n.has(key)) n.delete(key); else n.add(key); return n; });

  const Row = ({ k, note }: { k: string; note?: string }) => {
    const n = byKey.get(k);
    if (!n) return null;
    return (
      <button type="button" className={`tg-row${selected === k ? " tg-row--selected" : ""}`} onClick={() => setSelected(k)}>
        <span className="tg-dot" style={{ background: priorityColor(n.priorityRank) }} title={n.priority} />
        <span className="tg-row-key">{n.key}</span>
        <span className="tg-row-summary">{n.summary}</span>
        {n.contextKey && <span className="board-chip" title="Linked context">{n.contextKey}</span>}
        {note && <span className="tg-row-note">{note}</span>}
      </button>
    );
  };

  const blockerNote = (n: GNode) =>
    (n.blockedBy ?? []).map((b) => { const x = byKey.get(b); return x ? `${b} (${x.status})` : b; }).join(", ");

  return (
    <div className="tg-view">
      <div className="tg-canvas">
        <div className="tg-toolbar">
          {providers.length > 1 && (
            <select aria-label="Tracker" value={provider} onChange={(e) => setProvider(e.target.value)}>
              {providers.map((p) => <option key={p} value={p}>{p}</option>)}
            </select>
          )}
          <label className="tg-search"><Search size={12} />
            <input aria-label="Search issues" value={query} onChange={(e) => setQuery(e.target.value)} placeholder="Search key or summary…" />
          </label>
          <label className="tg-check"><input type="checkbox" checked={opts.onlyMine} onChange={(e) => setOpt({ onlyMine: e.target.checked })} /> Only mine</label>
          <label className="tg-check"><input type="checkbox" checked={opts.hideDone} onChange={(e) => setOpt({ hideDone: e.target.checked })} /> Hide done</label>
          <label className="tg-check"><input type="checkbox" checked={opts.showRelates} onChange={(e) => setOpt({ showRelates: e.target.checked })} /> Relates</label>
          <span className="tg-spacer" />
          {progress && refreshing && <span className="tg-progress">{progress.current}/{progress.total} · {progress.key}</span>}
          <button type="button" className="board-name-save" title="Reset layout" aria-label="Reset layout" onClick={() => setResetToken((t) => t + 1)}><RotateCcw size={12} /></button>
          <button type="button" className="board-name-save" disabled={refreshing || loading} title="Pull fresh data (⇧-click: re-read every issue)"
            onClick={(e) => void refresh(e.shiftKey)}>
            <RefreshCw size={12} className={refreshing ? "spinning" : ""} /> {refreshing ? "Refreshing…" : "Refresh"}
          </button>
        </div>
        {error && <div className="board-error">{error}</div>}
        {banner.length > 0 && (
          <div className="tg-banner" role="status">Unblocked: {banner.join(", ")} — ready to pick up.</div>
        )}
        {graph && graph.nodes.length === 0 && !loading && (
          <div className="board-section-empty">No issues yet — hit Refresh to pull your tracker.</div>
        )}
        <TaskGraph nodes={view.nodes} edges={view.edges} matches={view.matches} pulse={pulse} collapsed={collapsed}
          selected={selected} resetToken={resetToken} onSelect={setSelected} onToggleCollapse={toggleCollapse} />
      </div>

      <aside className="tg-rail">
        {sel && (
          <section className="tg-detail">
            <div className="tg-detail-head">
              <span className="tg-dot" style={{ background: priorityColor(sel.priorityRank) }} />
              <strong>{sel.key}</strong>
              {sel.url && <button type="button" className="board-icon-btn" title="Open in tracker" onClick={() => BrowserOpenURL(sel.url!)}><ExternalLink size={12} /></button>}
            </div>
            <div className="tg-detail-summary">{sel.summary}</div>
            <div className="tg-chips">
              <span className="board-chip">{sel.type}</span>
              <span className="board-chip">{sel.status}</span>
              {sel.priority && <span className="board-chip">{sel.priority}</span>}
              {sel.assignee && <span className="board-chip">{sel.assignee}</span>}
              {!sel.mine && <span className="board-chip">not mine</span>}
            </div>
            {sel.blocked && (
              <div className="tg-detail-row tg-detail-row--blocked"><Lock size={12} /> Blocked by {blockerNote(sel)}</div>
            )}
            {parentChain(sel.key, graph?.edges ?? []).length > 0 && (
              <div className="tg-detail-row">Under {parentChain(sel.key, graph?.edges ?? []).map((k) => {
                const p = byKey.get(k);
                return <button type="button" key={k} className="tg-link" onClick={() => setSelected(k)}>{k}{p?.epic ? " (epic)" : ""}</button>;
              })}</div>
            )}
            <div className="tg-detail-actions">
              {sel.contextId ? (
                <button type="button" className="board-open-btn" onClick={() => onOpenContext(sel.contextId!, sel.contextKey ?? "")}><FolderOpen size={12} /> Open context {sel.contextKey}</button>
              ) : (
                <>
                  {orgs.length > 1 && (
                    <select aria-label="Organization for the new context" value={selectedOrgId} onChange={(e) => onSelectOrg(e.target.value)}>
                      <option value="">Organization…</option>
                      {orgs.map((o) => <option key={o.id} value={o.id}>{o.name}</option>)}
                    </select>
                  )}
                  <button type="button" className="board-open-btn" disabled={creatingKey === sel.key || (orgs.length > 0 && !selectedOrgId)}
                    onClick={() => void onCreateContext(sel.key)}><FolderPlus size={12} /> {creatingKey === sel.key ? "Creating…" : "Create context"}</button>
                </>
              )}
            </div>
          </section>
        )}
        <section>
          <div className="board-section-header"><span className="board-section-title">Ready now</span>
            <span className="board-section-count">{graph?.ready.length ?? 0}</span></div>
          {graph && graph.ready.length === 0 && <div className="board-section-empty">Nothing unblocked.</div>}
          <div className="tg-rows">{graph?.ready.map((k) => <Row key={k} k={k} />)}</div>
        </section>
        <section>
          <div className="board-section-header"><span className="board-section-title">Waiting on</span>
            <span className="board-section-count">{graph?.waiting.length ?? 0}</span></div>
          {graph && graph.waiting.length === 0 && <div className="board-section-empty">Nothing blocked.</div>}
          <div className="tg-rows">{graph?.waiting.map((k) => { const n = byKey.get(k); return n ? <Row key={k} k={k} note={blockerNote(n)} /> : null; })}</div>
        </section>
      </aside>
    </div>
  );
}
