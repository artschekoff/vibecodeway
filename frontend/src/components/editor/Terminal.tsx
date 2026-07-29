import { useEffect, useRef } from "react";
import { Terminal as XTerm } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { TerminalStart, TerminalWrite, TerminalResize, TerminalClose } from "../../../wailsjs/go/main/App";
import { EventsOn, EventsOff } from "../../../wailsjs/runtime/runtime";

export function Terminal({ cwd, onSession }: { cwd: string; onSession?: (sessionId: string) => void }) {
  const hostRef = useRef<HTMLDivElement>(null);
  const onSessionRef = useRef(onSession);
  onSessionRef.current = onSession;

  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;

    const term = new XTerm({ fontSize: 12, cursorBlink: true, convertEol: true });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(host);
    if (host.offsetWidth && host.offsetHeight) fit.fit();

    let sessionID = "";
    let cancelled = false;
    let offData: (() => void) | undefined;
    let offExit: (() => void) | undefined;

    void TerminalStart(cwd).then((id) => {
      if (cancelled) {
        void TerminalClose(id);
        return;
      }
      sessionID = id;
      onSessionRef.current?.(id);
      offData = EventsOn(`term:data:${id}`, (chunk: string) => term.write(chunk));
      offExit = EventsOn(`term:exit:${id}`, () => term.write("\r\n[process exited]\r\n"));
      onResize(); // guarded fit + PTY resize (skips if the tab is hidden)
    });

    const disposeInput = term.onData((data) => {
      if (sessionID) void TerminalWrite(sessionID, data);
    });

    const onResize = () => {
      // Skip while the host has no layout box — e.g. the editor tab is hidden
      // (display:none), which collapses it to 0×0. FitAddon then proposes 1×1,
      // and resizing the PTY to one column makes the shell redraw its prompt
      // one char per line with split escape codes. Refit on return to visible.
      if (!host.offsetWidth || !host.offsetHeight) return;
      fit.fit();
      if (sessionID) void TerminalResize(sessionID, term.cols, term.rows);
    };
    window.addEventListener("resize", onResize);

    // Refit when the host element itself resizes — e.g. dragging the pane
    // splitter, which changes the container height without firing a window
    // 'resize' event. Without this the terminal keeps its old rows/cols and
    // leaves empty space in the enlarged pane.
    const resizeObserver = new ResizeObserver(() => onResize());
    resizeObserver.observe(host);

    return () => {
      cancelled = true;
      resizeObserver.disconnect();
      window.removeEventListener("resize", onResize);
      disposeInput.dispose();
      offData?.();
      offExit?.();
      if (sessionID) {
        onSessionRef.current?.("");
        void TerminalClose(sessionID);
        EventsOff(`term:data:${sessionID}`);
        EventsOff(`term:exit:${sessionID}`);
      }
      term.dispose();
    };
  }, [cwd]);

  return <div ref={hostRef} style={{ width: "100%", height: "100%" }} />;
}
