import { useCallback, useEffect, useState } from "react";
import { GetAnalyticsDashboard } from "../../wailsjs/go/main/App";
import type { AnalyticsDashboard, DayRow, PeriodCard, Slice } from "../types";

type RangeDays = 7 | 30 | 90;

const RANGE_OPTIONS: RangeDays[] = [7, 30, 90];

const METRICS: { metric: string; label: string }[] = [
  { metric: "tasksClosed", label: "Tasks closed" },
  { metric: "prsMerged", label: "PRs merged" },
  { metric: "comments", label: "Comments" },
];

const PERIODS: { period: string; label: string }[] = [
  { period: "yesterday", label: "Yesterday" },
  { period: "week", label: "Week" },
  { period: "month", label: "Month" },
];

// True when the loaded window has no activity at all — the state every
// user sees on first run, since these tables only start collecting from
// when this feature shipped (no backfill). Mirrors ProductivityPage's
// isEmptyHistory.
function isEmptyHistory(d: AnalyticsDashboard): boolean {
  return (
    d.cards.every((c) => c.count === 0) &&
    d.daily.every((row) => row.tasks === 0 && row.prs === 0 && row.comments === 0)
  );
}

function deltaClass(pct: number): string {
  if (pct > 0) return "analytics-delta-up";
  if (pct < 0) return "analytics-delta-down";
  return "analytics-delta-flat";
}

function deltaLabel(pct: number): string {
  if (pct > 0) return `▲${Math.round(pct)}%`;
  if (pct < 0) return `▼${Math.round(Math.abs(pct))}%`;
  return "0%";
}

function findCard(cards: PeriodCard[], metric: string, period: string): PeriodCard | undefined {
  return cards.find((c) => c.metric === metric && c.period === period);
}

function MetricColumn({ metric, label, cards }: { metric: string; label: string; cards: PeriodCard[] }) {
  return (
    <div className="analytics-metric-col" data-testid={`analytics-metric-${metric}`}>
      <div className="analytics-metric-title">{label}</div>
      {PERIODS.map(({ period, label: periodLabel }) => {
        const card = findCard(cards, metric, period);
        const count = card?.count ?? 0;
        const pct = card?.deltaPct ?? 0;
        return (
          <div
            key={period}
            className="analytics-period-card"
            data-testid={`analytics-card-${metric}-${period}`}
          >
            <div className="analytics-period-label">{periodLabel}</div>
            <div className="analytics-period-value">{count}</div>
            <span className={`analytics-delta-badge ${deltaClass(pct)}`}>{deltaLabel(pct)}</span>
          </div>
        );
      })}
    </div>
  );
}

// formatHours rounds to 1 decimal but drops a trailing ".0" so whole-number
// hours stay terse ("3h" not "3.0h"), matching ProductivityPage's convention.
function formatHours(n: number): string {
  return n.toFixed(1).replace(/\.0$/, "");
}

function AvgTaskTile({ dashboard }: { dashboard: AnalyticsDashboard }) {
  const { avgTask } = dashboard;
  return (
    <div className="analytics-avg-task" data-testid="analytics-avg-task">
      {avgTask.assumed ? (
        <>
          «{formatHours(avgTask.medianHours)}h» (assumed — not enough measured tasks yet)
        </>
      ) : (
        <>
          «{formatHours(avgTask.medianHours)}h median» (measured, N={avgTask.samples})
        </>
      )}
    </div>
  );
}

function TrendChart({ daily }: { daily: DayRow[] }) {
  const max = Math.max(1, ...daily.map((d) => Math.max(d.tasks, d.prs, d.comments)));
  return (
    <div className="analytics-trend">
      <div className="analytics-trend-legend">
        <span className="analytics-legend-item">
          <span className="analytics-legend-swatch analytics-legend-tasks" /> Tasks
        </span>
        <span className="analytics-legend-item">
          <span className="analytics-legend-swatch analytics-legend-prs" /> PRs
        </span>
        <span className="analytics-legend-item">
          <span className="analytics-legend-swatch analytics-legend-comments" /> Comments
        </span>
      </div>
      <div className="analytics-trend-bars">
        {daily.map((d) => (
          <div key={d.day} className="analytics-trend-col" title={`${d.day}: ${d.tasks} tasks, ${d.prs} PRs, ${d.comments} comments`}>
            <div className="analytics-trend-group">
              <div
                data-testid="analytics-trend-bar"
                className="analytics-trend-bar analytics-trend-bar-tasks"
                style={{ height: `${(d.tasks / max) * 100}%` }}
              />
              <div
                className="analytics-trend-bar analytics-trend-bar-prs"
                style={{ height: `${(d.prs / max) * 100}%` }}
              />
              <div
                className="analytics-trend-bar analytics-trend-bar-comments"
                style={{ height: `${(d.comments / max) * 100}%` }}
              />
            </div>
            <span className="analytics-trend-label">{d.day.slice(8)}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

function BreakdownList({ title, slices }: { title: string; slices: Slice[] }) {
  const max = Math.max(1, ...slices.map((s) => s.count));
  return (
    <div className="analytics-breakdown" data-testid={`analytics-breakdown-${title}`}>
      <div className="analytics-breakdown-title">{title}</div>
      {slices.length === 0 ? (
        <div className="analytics-breakdown-empty">No data</div>
      ) : (
        slices.map((s) => (
          <div key={s.label} className="analytics-breakdown-row">
            <span className="analytics-breakdown-label">{s.label}</span>
            <div className="analytics-breakdown-bar-track">
              <div
                className="analytics-breakdown-bar"
                style={{ width: `${(s.count / max) * 100}%` }}
              />
            </div>
            <span className="analytics-breakdown-count">{s.count}</span>
          </div>
        ))
      )}
    </div>
  );
}

export default function AnalyticsPage() {
  const [range, setRange] = useState<RangeDays>(30);
  const [dashboard, setDashboard] = useState<AnalyticsDashboard | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async (rangeDays: RangeDays) => {
    setLoading(true);
    setError(null);
    try {
      const d = (await GetAnalyticsDashboard(rangeDays)) as unknown as AnalyticsDashboard;
      setDashboard(d);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load(range);
  }, [load, range]);

  return (
    <div className="analytics-page">
      <div className="analytics-toolbar">
        <h2 className="analytics-title">Analytics</h2>
        <div className="analytics-range-switcher">
          {RANGE_OPTIONS.map((r) => (
            <button
              key={r}
              className={`btn-ghost analytics-range-btn${r === range ? " analytics-range-btn-active" : ""}`}
              onClick={() => setRange(r)}
              disabled={loading}
            >
              {r}d
            </button>
          ))}
        </div>
        <button className="btn-ghost" onClick={() => void load(range)} disabled={loading}>
          {loading ? "Loading…" : "Refresh"}
        </button>
        {error && <span className="timeline-error">{error}</span>}
      </div>

      {dashboard && (
        <>
          {isEmptyHistory(dashboard) ? (
            <p className="analytics-headline">
              History starts now — merges, comments and closed tasks are recorded from here on,
              nothing is missing.
            </p>
          ) : (
            <>
              <div className="analytics-cards-grid">
                {METRICS.map(({ metric, label }) => (
                  <MetricColumn key={metric} metric={metric} label={label} cards={dashboard.cards} />
                ))}
              </div>

              <AvgTaskTile dashboard={dashboard} />

              <TrendChart daily={dashboard.daily} />

              <div className="analytics-breakdowns">
                <BreakdownList title="By issue type" slices={dashboard.byType} />
                <BreakdownList title="By org" slices={dashboard.byOrg} />
              </div>
            </>
          )}
        </>
      )}
    </div>
  );
}
