import { useEffect, useRef } from "react";
import { Terminal as XTerm } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { WebLinksAddon } from "@xterm/addon-web-links";
import "@xterm/xterm/css/xterm.css";
import { TerminalStart, TerminalWrite, TerminalResize, TerminalClose } from "../../../wailsjs/go/main/App";
import { BrowserOpenURL, EventsOn, EventsOff } from "../../../wailsjs/runtime/runtime";
import { useTabVisible } from "../../contexts/TabVisibleContext";

export function Terminal({ cwd, onSession, onExit }: {
  cwd: string;
  onSession?: (sessionId: string) => void;
  // Fires when the shell itself ends (the user typed `exit`, or it was killed).
  // Kept in a ref so a changing callback never re-runs the effect and restarts
  // the PTY.
  onExit?: () => void;
}) {
  const hostRef = useRef<HTMLDivElement>(null);
  const onSessionRef = useRef(onSession);
  onSessionRef.current = onSession;
  const onExitRef = useRef(onExit);
  onExitRef.current = onExit;
  // Set by the main effect; lets the visibility effect below force a refit.
  const refitRef = useRef<() => void>(() => {});

  // Anything can change size while this tab is hidden (display:none = no
  // ResizeObserver events, onResize guarded out), leaving the xterm grid and
  // PTY sized for a stale layout — content then draws past the visible pane
  // until a window resize happens to poke the observers. Refit explicitly on
  // every show, after the browser has laid the pane out (rAF).
  const tabVisible = useTabVisible();
  useEffect(() => {
    if (!tabVisible) return;
    const raf = requestAnimationFrame(() => refitRef.current());
    return () => cancelAnimationFrame(raf);
  }, [tabVisible]);

  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;

    // Links must go through BrowserOpenURL — window.open inside the Wails
    // webview does not reach the system browser. linkHandler covers OSC 8
    // hyperlinks (e.g. Claude Code output); WebLinksAddon covers plain-text
    // URLs.
    const openLink = (_e: MouseEvent, uri: string) => BrowserOpenURL(uri);
    const term = new XTerm({
      fontSize: 12,
      cursorBlink: true,
      convertEol: true,
      linkHandler: { activate: openLink },
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.loadAddon(new WebLinksAddon(openLink));
    term.open(host);
    if (host.offsetWidth && host.offsetHeight) fit.fit();

    let sessionID = "";
    let cancelled = false;
    let offData: (() => void) | undefined;
    let offExit: (() => void) | undefined;

    void TerminalStart(cwd).catch((err: unknown) => {
      // Attached BEFORE .then so it only ever sees a failed start, never a
      // throw from the success path. A restored tab whose folder no longer
      // exists (a deleted worktree) rejects here: pty.Manager.Start's chdir
      // fails before a session exists, so there is no onSession and no exit
      // event to react to. Leave the tab visible with a message rather than a
      // silent blank rectangle — no onExit, since retiring the tab would erase
      // the message before the user could read which folder vanished.
      if (cancelled) return undefined;
      const message = err instanceof Error ? err.message : String(err);
      term.write(`\r\n[cannot open a terminal in ${cwd}: ${message}]\r\n`);
      return undefined;
    }).then((id) => {
      if (id === undefined) return; // start failed; the catch above reported it
      if (cancelled) {
        void TerminalClose(id);
        return;
      }
      sessionID = id;
      onSessionRef.current?.(id);
      // Sticky bottom: follow new output only while the viewport is at the
      // bottom. TUI redraws (cursor addressing, scroll regions) can leave the
      // viewport mid-scrollback, after which xterm stops auto-following and
      // the terminal looks "stuck at the top". If the user scrolled up
      // themselves, atBottom() is false BEFORE the write and we leave their
      // position alone.
      offData = EventsOn(`term:data:${id}`, (chunk: string) => {
        const stick = atBottom();
        term.write(chunk, () => { if (stick && !atBottom()) term.scrollToBottom(); });
      });
      offExit = EventsOn(`term:exit:${id}`, () => {
        term.write("\r\n[process exited]\r\n");
        onExitRef.current?.();
      });
      onResize(); // guarded fit + PTY resize (skips if the tab is hidden)
    });

    const disposeInput = term.onData((data) => {
      if (sessionID) void TerminalWrite(sessionID, data);
    });

    // Whether the viewport currently shows the live bottom of the buffer.
    // Defensive optional chain: test doubles (and disposed terminals) have no
    // buffer — treat that as "at bottom" so resize/write paths never throw.
    const atBottom = () => {
      const b = term.buffer?.active;
      if (!b) return true;
      return b.viewportY >= b.baseY;
    };

    const onResize = () => {
      // Skip while the host has no layout box — e.g. the editor tab is hidden
      // (display:none), which collapses it to 0×0. FitAddon then proposes 1×1,
      // and resizing the PTY to one column makes the shell redraw its prompt
      // one char per line with split escape codes. Refit on return to visible.
      if (!host.offsetWidth || !host.offsetHeight) return;
      // Reflow can shift the viewport; keep it pinned to the bottom if that's
      // where it was (a user parked mid-scrollback keeps their spot).
      const stick = atBottom();
      fit.fit();
      if (sessionID) void TerminalResize(sessionID, term.cols, term.rows);
      if (stick && !atBottom()) term.scrollToBottom();
    };
    refitRef.current = onResize;
    window.addEventListener("resize", onResize);

    // Refit when the host element itself resizes — e.g. dragging the pane
    // splitter, which changes the container height without firing a window
    // 'resize' event. Without this the terminal keeps its old rows/cols and
    // leaves empty space in the enlarged pane.
    const resizeObserver = new ResizeObserver(() => onResize());
    resizeObserver.observe(host);

    return () => {
      cancelled = true;
      refitRef.current = () => {};
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
