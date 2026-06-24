import { useEffect, useRef, useState, type FormEvent } from "react";
import { api, type ServerSummary } from "../lib/api";

export function ServerConsole({ server }: { server: ServerSummary }) {
  const [lines, setLines] = useState<string[]>([]);
  const [connected, setConnected] = useState(false);
  const [input, setInput] = useState("");
  const wsRef = useRef<WebSocket | null>(null);
  const endRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const ws = new WebSocket(api.consoleURL(server.id));
    wsRef.current = ws;
    ws.onopen = () => setConnected(true);
    ws.onclose = () => setConnected(false);
    ws.onerror = () => setConnected(false);
    ws.onmessage = (e) =>
      setLines((prev) => {
        const next = [...prev, String(e.data)];
        return next.length > 1000 ? next.slice(next.length - 1000) : next;
      });
    return () => ws.close();
  }, [server.id]);

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [lines]);

  const canSend = server.commandMethod === "rcon-cli";

  function sendCmd(e: FormEvent) {
    e.preventDefault();
    const cmd = input.trim();
    const ws = wsRef.current;
    if (!cmd || !ws || ws.readyState !== WebSocket.OPEN) return;
    ws.send(cmd);
    setInput("");
  }

  return (
    <div className="flex h-full flex-col bg-zinc-950">
      <header className="flex items-center justify-between border-b border-zinc-800 px-6 py-3">
        <div className="flex items-center gap-3">
          <h2 className="font-semibold text-zinc-100">{server.name}</h2>
          <span
            className={`inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs ${
              connected ? "bg-emerald-400/10 text-emerald-400" : "bg-zinc-700/40 text-zinc-400"
            }`}
          >
            <span
              className={`h-1.5 w-1.5 rounded-full ${connected ? "bg-emerald-400" : "bg-zinc-500"}`}
            />
            {connected ? "live" : "disconnected"}
          </span>
        </div>
        <span className="text-xs text-zinc-600">{server.image}</span>
      </header>

      <div className="flex-1 overflow-y-auto bg-black px-4 py-3 font-mono text-xs leading-relaxed text-zinc-300">
        {lines.length === 0 && (
          <p className="text-zinc-600">
            Waiting for output… (the console shows the last 200 lines, then streams live)
          </p>
        )}
        {lines.map((l, i) => (
          <div key={i} className="whitespace-pre-wrap break-words">
            {l}
          </div>
        ))}
        <div ref={endRef} />
      </div>

      <form onSubmit={sendCmd} className="flex gap-2 border-t border-zinc-800 p-3">
        <input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          disabled={!canSend}
          placeholder={
            canSend ? "Type a console command and press Enter…" : "Console input isn't supported for this game yet"
          }
          className="flex-1 rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-2 font-mono text-sm text-zinc-100 outline-none focus:border-emerald-500 disabled:opacity-50"
        />
        <button
          type="submit"
          disabled={!canSend}
          className="rounded-lg bg-emerald-500 px-4 py-2 text-sm font-semibold text-zinc-950 transition hover:bg-emerald-400 disabled:opacity-40"
        >
          Send
        </button>
      </form>
    </div>
  );
}
