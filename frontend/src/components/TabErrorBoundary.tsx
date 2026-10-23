import { Component, type ReactNode } from "react";

/**
 * Catches render crashes of one tab so they show an inline error instead of
 * unmounting the whole app into a black window. `resetKey` (the tab id) drops
 * the error state when the user switches to another tab.
 */
export class TabErrorBoundary extends Component<
  { children: ReactNode; resetKey?: string },
  { error: Error | null }
> {
  state = { error: null as Error | null };

  static getDerivedStateFromError(error: Error) {
    return { error };
  }

  componentDidUpdate(prev: { resetKey?: string }) {
    if (prev.resetKey !== this.props.resetKey && this.state.error) {
      this.setState({ error: null });
    }
  }

  render() {
    if (this.state.error) {
      return (
        <div role="alert" style={{ padding: 24, fontSize: 12, color: "var(--text)" }}>
          <div style={{ fontWeight: 600, marginBottom: 6 }}>This tab crashed.</div>
          <div style={{ color: "var(--danger, #e5534b)", fontFamily: "monospace", marginBottom: 10, whiteSpace: "pre-wrap" }}>
            {this.state.error.message}
          </div>
          <button className="btn-secondary" style={{ fontSize: 11 }} onClick={() => this.setState({ error: null })}>
            Try again
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}
