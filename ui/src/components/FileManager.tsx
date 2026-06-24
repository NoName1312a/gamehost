import { useCallback, useEffect, useState } from "react";
import { api, type FileEntry, type ServerSummary } from "../lib/api";
import { friendlyError } from "../lib/errors";

function fmtSize(n: number): string {
  if (n < 1024) return n + " B";
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + " KB";
  return (n / 1024 / 1024).toFixed(1) + " MB";
}

const ghostBtn =
  "rounded-lg border border-zinc-700 px-3 py-1.5 text-sm text-zinc-200 hover:bg-zinc-800 disabled:opacity-50";
const primaryBtn =
  "rounded-lg bg-emerald-500 px-3 py-1.5 text-sm font-semibold text-zinc-950 hover:bg-emerald-400 disabled:opacity-50";

// FileManager browses and edits a server's data volume (configs, mods, worlds)
// through the engine's docker-backed file API. Works whether the server is
// running or stopped.
export function FileManager({ server }: { server: ServerSummary }) {
  const [path, setPath] = useState("");
  const [entries, setEntries] = useState<FileEntry[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [editing, setEditing] = useState<{ path: string; content: string; truncated: boolean } | null>(null);
  const [saving, setSaving] = useState(false);

  const load = useCallback(
    async (p: string) => {
      setLoading(true);
      setError(null);
      try {
        const r = await api.listFiles(server.id, p);
        setEntries(r.entries);
        setPath(p);
      } catch (e) {
        setError(friendlyError(e));
      } finally {
        setLoading(false);
      }
    },
    [server.id],
  );

  useEffect(() => {
    load("");
  }, [load]);

  const join = (p: string, name: string) => (p ? p + "/" + name : name);
  const fail = (e: unknown) => setError(friendlyError(e));

  async function openFile(name: string) {
    setError(null);
    try {
      const r = await api.readFile(server.id, join(path, name));
      setEditing({ path: join(path, name), content: r.content, truncated: r.truncated });
    } catch (e) {
      fail(e);
    }
  }
  async function save() {
    if (!editing) return;
    setSaving(true);
    try {
      await api.writeFile(server.id, editing.path, editing.content);
      setEditing(null);
      load(path);
    } catch (e) {
      fail(e);
    } finally {
      setSaving(false);
    }
  }
  async function del(e: FileEntry) {
    if (!confirm(`Delete "${e.name}"${e.isDir ? " and everything inside it" : ""}? This can't be undone.`)) return;
    try {
      await api.deleteFile(server.id, join(path, e.name));
      load(path);
    } catch (err) {
      fail(err);
    }
  }
  async function newFolder() {
    const name = prompt("New folder name:");
    if (!name) return;
    try {
      await api.makeDir(server.id, join(path, name));
      load(path);
    } catch (e) {
      fail(e);
    }
  }
  async function newFile() {
    const name = prompt("New file name:");
    if (!name) return;
    try {
      await api.writeFile(server.id, join(path, name.trim()), "");
      load(path);
    } catch (e) {
      fail(e);
    }
  }

  const crumbs = path ? path.split("/") : [];
  const sorted = entries
    ? [...entries].sort((a, b) => (a.isDir === b.isDir ? a.name.localeCompare(b.name) : a.isDir ? -1 : 1))
    : [];

  return (
    <div className="flex h-full flex-col bg-zinc-950">
      <header className="flex items-center justify-between gap-3 border-b border-zinc-800 px-6 py-3">
        <div className="flex items-center gap-3">
          <h2 className="font-semibold text-zinc-100">Files — {server.name}</h2>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={newFile} className={ghostBtn}>+ File</button>
          <button onClick={newFolder} className={ghostBtn}>+ Folder</button>
          <button onClick={() => load(path)} className={ghostBtn} title="Refresh">↻</button>
        </div>
      </header>

      {/* Breadcrumb */}
      <div className="flex flex-wrap items-center gap-1 border-b border-zinc-900 px-6 py-2 text-sm text-zinc-400">
        <button onClick={() => load("")} className="hover:text-zinc-100">/</button>
        {crumbs.map((c, i) => (
          <span key={i} className="flex items-center gap-1">
            <span className="text-zinc-600">/</span>
            <button onClick={() => load(crumbs.slice(0, i + 1).join("/"))} className="hover:text-zinc-100">
              {c}
            </button>
          </span>
        ))}
      </div>

      {error && (
        <div className="mx-6 mt-3 rounded-lg border border-rose-500/30 bg-rose-500/10 px-3 py-2 text-sm text-rose-300">
          {error}
        </div>
      )}

      <div className="flex-1 overflow-y-auto px-6 py-3">
        {path !== "" && (
          <button
            onClick={() => load(crumbs.slice(0, -1).join("/"))}
            className="flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-sm text-zinc-400 hover:bg-zinc-900"
          >
            <span>📁</span> ..
          </button>
        )}
        {loading && !entries && <p className="px-2 py-2 text-sm text-zinc-500">Loading…</p>}
        {entries && sorted.length === 0 && <p className="px-2 py-2 text-sm text-zinc-600">This folder is empty.</p>}
        {sorted.map((e) => (
          <div key={e.name} className="group flex items-center gap-2 rounded-lg px-2 py-1.5 hover:bg-zinc-900">
            <button
              onClick={() => (e.isDir ? load(join(path, e.name)) : openFile(e.name))}
              className="flex min-w-0 flex-1 items-center gap-2 text-left text-sm text-zinc-200"
            >
              <span>{e.isDir ? "📁" : "📄"}</span>
              <span className="truncate">{e.name}</span>
            </button>
            {!e.isDir && <span className="shrink-0 text-xs text-zinc-600">{fmtSize(e.size)}</span>}
            <button
              onClick={() => del(e)}
              className="shrink-0 rounded px-2 py-0.5 text-xs text-rose-400/70 opacity-0 hover:bg-rose-500/10 hover:text-rose-300 group-hover:opacity-100"
            >
              delete
            </button>
          </div>
        ))}
      </div>

      {/* Editor */}
      {editing && (
        <div className="fixed inset-0 z-50 flex flex-col bg-zinc-950/95 p-6" onClick={() => setEditing(null)}>
          <div
            className="mx-auto flex h-full w-full max-w-4xl flex-col rounded-xl border border-zinc-800 bg-zinc-950"
            onClick={(ev) => ev.stopPropagation()}
          >
            <div className="flex items-center justify-between border-b border-zinc-800 px-4 py-2">
              <code className="text-sm text-zinc-300">{editing.path}</code>
              <div className="flex items-center gap-2">
                {editing.truncated && (
                  <span className="text-xs text-amber-400">file truncated — saving would cut it; download instead</span>
                )}
                <button onClick={() => setEditing(null)} className={ghostBtn}>Cancel</button>
                <button onClick={save} disabled={saving || editing.truncated} className={primaryBtn}>
                  {saving ? "Saving…" : "Save"}
                </button>
              </div>
            </div>
            <textarea
              value={editing.content}
              onChange={(e) => setEditing({ ...editing, content: e.target.value })}
              spellCheck={false}
              className="flex-1 resize-none bg-black px-4 py-3 font-mono text-xs leading-relaxed text-zinc-200 outline-none"
            />
          </div>
        </div>
      )}
    </div>
  );
}
