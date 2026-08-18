import { useCallback, useEffect, useState } from "react";
import { GetProductivityStats } from "../../wailsjs/go/main/App";
import type { ProductivityStats } from "../types";

// ponytail: plain CSS bars instead of a chart library — 30 bars need no d3.
function Bars({ stats }: { stats: ProductivityStats }) {
  const max = Math.max(1, ...stats.daily.map((d) => d.count));
  return (
    <div className="productivity-bars">
      {stats.daily.map((d) => (
        <div key={d.day} className="productivity-bar-col" title={`${d.day}: ${d.count}`}>
          <div
            data-testid="productivity-bar"
            className="productivity-bar"
            style={{ height: `${(d.count / max) * 100}%` }}
          />
          <span className="productivity-bar-label">{d.day.slice(8)}</span>
        </div>
      ))}
    </div>
  );
}

function BaselineCard({
  testId,
  label,
  baseline,
  today,
  toBeat,
}: {
  testId: string;
  label: string;
  baseline: string;
  today: number;
  toBeat: number;
}) {
  return (
    <div className="productivity-card" data-testid={testId}>
      <div className="productivity-card-label">{label}</div>
      <div className="productivity-card-value">
        {today} <span className="productivity-card-vs">vs {baseline}</span>
      </div>
      <div className="productivity-card-hint">
        {toBeat === 0 ? "Ahead" : `${toBeat} more to beat it`}
      </div>
    </div>
  );
}

function headline(s: ProductivityStats): string {
  const tasks = `You closed ${s.today} ${s.today === 1 ? "task" : "tasks"} today`;
  const hours = `${s.taskHours}h each, ${s.hoursToday}h of ${s.workdayHours}h`;
  const rest =
    s.tasksToNorm === 0
      ? "Workday norm met."
      : `${s.tasksToNorm} more ${s.tasksToNorm === 1 ? "task" : "tasks"} to close the day.`;
  return `${tasks} — ${hours}. ${rest}`;
}

export default function ProductivityPage() {
  const [stats, setStats] = useState<ProductivityStats | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const s = (await GetProductivityStats()) as unknown as ProductivityStats;
      setStats(s);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <div className="productivity-page">
      <div className="productivity-toolbar">
        <h2 className="productivity-title">Closed tasks</h2>
        <button className="btn-ghost" onClick={() => void load()} disabled={loading}>
          {loading ? "Loading…" : "Refresh"}
        </button>
        {error && <span className="timeline-error">{error}</span>}
      </div>

      {stats && (
        <>
          <p className="productivity-headline">{headline(stats)}</p>

          <div className="productivity-cards">
            <BaselineCard
              testId="baseline-yesterday"
              label="Today vs yesterday"
              baseline={String(stats.yesterday)}
              today={stats.today}
              toBeat={stats.tasksToBeatYesterday}
            />
            <BaselineCard
              testId="baseline-week"
              label="Today vs last week (avg/day)"
              baseline={stats.weekAvg.toFixed(1)}
              today={stats.today}
              toBeat={stats.tasksToBeatWeek}
            />
            <BaselineCard
              testId="baseline-month"
              label="Today vs last month (avg/day)"
              baseline={stats.monthAvg.toFixed(1)}
              today={stats.today}
              toBeat={stats.tasksToBeatMonth}
            />
          </div>

          <Bars stats={stats} />
        </>
      )}
    </div>
  );
}
