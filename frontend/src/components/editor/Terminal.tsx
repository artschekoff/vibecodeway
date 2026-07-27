import { useEffect, useRef } from "react";
import { Terminal as XTerm } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { TerminalStart, TerminalWrite, TerminalResize, TerminalClose } from "../../../wailsjs/go/main/App";
import { EventsOn, EventsOff } from "../../../wailsjs/runtime/runtime";

export function Terminal({ cwd }: { cwd: string }) {
  const hostRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;

    const term = new XTerm({ fontSize: 12, cursorBlink: true, convertEol: true });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(host);
    fit.fit();

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
      offData = EventsOn(`term:data:${id}`, (chunk: string) => term.write(chunk));
      offExit = EventsOn(`term:exit:${id}`, () => term.write("\r\n[process exited]\r\n"));
      void TerminalResize(id, term.cols, term.rows);
    });

    const disposeInput = term.onData((data) => {
      if (sessionID) void TerminalWrite(sessionID, data);
    });

    const onResize = () => {
      fit.fit();
      if (sessionID) void TerminalResize(sessionID, term.cols, term.rows);
    };
    window.addEventListener("resize", onResize);

    return () => {
      cancelled = true;
      window.removeEventListener("resize", onResize);
      disposeInput.dispose();
      offData?.();
      offExit?.();
      if (sessionID) {
        void TerminalClose(sessionID);
        EventsOff(`term:data:${sessionID}`);
        EventsOff(`term:exit:${sessionID}`);
      }
      term.dispose();
    };
  }, [cwd]);

  return <div ref={hostRef} style={{ width: "100%", height: "100%" }} />;
}
