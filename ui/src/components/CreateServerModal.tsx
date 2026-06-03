import { useState, type FormEvent } from "react";
import { api, type Template, type CreateServerRequest } from "../lib/api";

export function CreateServerModal({
  template,
  onClose,
  onCreated,
}: {
  template: Template;
  onClose: () => void;
  onCreated: () => void;
}) {
  const [name, setName] = useState(template.name);
  const [port, setPort] = useState<number>(template.ports?.[0]?.default ?? 0);
  const [memory, setMemory] = useState<number>(template.recMemoryMB);
  const [vars, setVars] = useState<Record<string, string>>(() => {
    const v: Record<string, string> = {};
    for (const variable of template.variables ?? []) v[variable.key] = variable.default ?? "";
    return v;
  });
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      const req: CreateServerRequest = {
        templateId: template.id,
        name: name.trim() || template.name,
        memoryMB: memory,
        port,
        variables: vars,
      };
      await api.createServer(req);
      onCreated();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      setSubmitting(false);
    }
  }

  const field =
    "w-full rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-emerald-500";
  const label = "mb-1 block text-xs font-medium text-zinc-400";

  return (
    <div className="fixed inset-0 z-50 grid place-items-center bg-black/60 p-4" onClick={onClose}>
      <form
        onClick={(e) => e.stopPropagation()}
        onSubmit={submit}
        className="max-h-[90vh] w-full max-w-lg overflow-y-auto rounded-2xl border border-zinc-800 bg-zinc-950 p-6 shadow-2xl"
      >
        <div className="mb-4">
          <h2 className="text-lg font-semibold text-zinc-100">New {template.name} server</h2>
          <p className="text-sm text-zinc-500">{template.description}</p>
        </div>

        <div className="space-y-4">
          <div>
            <label className={label}>Server name</label>
            <input className={field} value={name} onChange={(e) => setName(e.target.value)} />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className={label}>Host port</label>
              <input
                className={field}
                type="number"
                value={port}
                onChange={(e) => setPort(Number(e.target.value))}
              />
            </div>
            <div>
              <label className={label}>Memory (MB)</label>
              <input
                className={field}
                type="number"
                value={memory}
                onChange={(e) => setMemory(Number(e.target.value))}
                min={template.minMemoryMB}
              />
            </div>
          </div>

          {(template.variables ?? []).map((v) => (
            <div key={v.key}>
              <label className={label}>
                {v.label}
                {v.required && <span className="text-rose-400"> *</span>}
              </label>
              {v.type === "enum" && v.options ? (
                <select
                  className={field}
                  value={vars[v.key]}
                  onChange={(e) => setVars((s) => ({ ...s, [v.key]: e.target.value }))}
                >
                  {v.options.map((o) => (
                    <option key={o} value={o}>
                      {o}
                    </option>
                  ))}
                </select>
              ) : (
                <input
                  className={field}
                  value={vars[v.key]}
                  onChange={(e) => setVars((s) => ({ ...s, [v.key]: e.target.value }))}
                />
              )}
              {v.description && <p className="mt-1 text-xs text-zinc-600">{v.description}</p>}
            </div>
          ))}
        </div>

        {error && (
          <p className="mt-4 rounded-lg border border-rose-500/30 bg-rose-500/10 px-3 py-2 text-sm text-rose-300">
            {error}
          </p>
        )}

        <div className="mt-6 flex justify-end gap-3">
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg px-4 py-2 text-sm text-zinc-400 hover:text-zinc-200"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={submitting}
            className="rounded-lg bg-emerald-500 px-4 py-2 text-sm font-semibold text-zinc-950 transition hover:bg-emerald-400 disabled:opacity-50"
          >
            {submitting ? "Creating…" : "Create server"}
          </button>
        </div>
      </form>
    </div>
  );
}
