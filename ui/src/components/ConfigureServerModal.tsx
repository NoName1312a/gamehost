import { useState, type FormEvent } from "react";
import { api, type CreateServerRequest, type ServerSummary, type Template } from "../lib/api";
import { editionLabel, type GameGroup } from "../lib/games";
import { friendlyError } from "../lib/errors";

const field =
  "w-full rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-emerald-500";
const labelCls = "mb-1 block text-xs font-medium text-zinc-400";
const catAccent: Record<string, string> = {
  Sandbox: "text-emerald-300 bg-emerald-400/10 ring-emerald-400/20",
  Survival: "text-amber-300 bg-amber-400/10 ring-amber-400/20",
  Shooter: "text-rose-300 bg-rose-400/10 ring-rose-400/20",
  Modded: "text-violet-300 bg-violet-400/10 ring-violet-400/20",
};
const catClass = (c: string) => catAccent[c] ?? "text-zinc-300 bg-zinc-400/10 ring-zinc-400/20";

// EditionPicker is shown for games with more than one template (e.g. Minecraft:
// Java, Bedrock, modpacks). Each edition is a selectable tile.
function EditionPicker({ group, onPick }: { group: GameGroup; onPick: (t: Template) => void }) {
  return (
    <div className="space-y-2">
      <p className="text-sm text-zinc-400">Choose an edition:</p>
      {group.templates.map((t) => (
        <button
          key={t.id}
          onClick={() => onPick(t)}
          className="group flex w-full items-start gap-3 rounded-xl border border-zinc-800 bg-zinc-900/50 p-4 text-left transition hover:border-emerald-500/40 hover:bg-zinc-900"
        >
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <h4 className="font-medium text-zinc-100">{editionLabel(group, t)}</h4>
              <span className={`rounded-full px-2 py-0.5 text-[10px] font-medium ring-1 ring-inset ${catClass(t.category)}`}>
                {t.category}
              </span>
            </div>
            <p className="mt-1 text-xs text-zinc-500">{t.description}</p>
          </div>
          <span className="mt-1 shrink-0 text-zinc-600 transition group-hover:translate-x-0.5 group-hover:text-emerald-400">→</span>
        </button>
      ))}
    </div>
  );
}

function OptionsForm({
  template,
  onBack,
  onCreated,
}: {
  template: Template;
  onBack?: () => void;
  onCreated: (server: ServerSummary) => void;
}) {
  const [name, setName] = useState(template.name);
  const [port, setPort] = useState<number>(template.ports?.[0]?.default ?? 0);
  const [memory, setMemory] = useState<number>(template.recMemoryMB);
  const [cpus, setCpus] = useState<number>(0); // 0 = no CPU limit
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
        cpus: cpus > 0 ? cpus : undefined,
        port,
        variables: vars,
      };
      const created = await api.createServer(req);
      onCreated(created);
    } catch (err) {
      setError(friendlyError(err));
      setSubmitting(false);
    }
  }

  return (
    <form onSubmit={submit} className="space-y-4">
      {onBack && (
        <button type="button" onClick={onBack} className="text-xs text-zinc-400 hover:text-zinc-200">
          ← editions
        </button>
      )}
      <div>
        <label className={labelCls}>Server name</label>
        <input className={field} value={name} onChange={(e) => setName(e.target.value)} />
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className={labelCls}>Host port</label>
          <input className={field} type="number" value={port} onChange={(e) => setPort(Number(e.target.value))} />
        </div>
        <div>
          <label className={labelCls}>Memory (MB)</label>
          <input
            className={field}
            type="number"
            value={memory}
            onChange={(e) => setMemory(Number(e.target.value))}
            min={template.minMemoryMB}
          />
        </div>
      </div>
      <div>
        <label className={labelCls}>CPU limit (cores)</label>
        <input
          className={field}
          type="number"
          value={cpus}
          onChange={(e) => setCpus(Number(e.target.value))}
          min={0}
          step={0.5}
        />
        <p className="mt-1 text-xs text-zinc-600">0 = no limit. Cap cores so one server can't starve the others.</p>
      </div>
      {(template.variables ?? []).map((v) => (
        <div key={v.key}>
          <label className={labelCls}>
            {v.label}
            {v.required && <span className="text-rose-400"> *</span>}
          </label>
          {v.type === "enum" && v.options ? (
            <select className={field} value={vars[v.key] ?? ""} onChange={(e) => setVars((s) => ({ ...s, [v.key]: e.target.value }))}>
              {v.options.map((o) => (
                <option key={o} value={o}>
                  {o}
                </option>
              ))}
            </select>
          ) : (
            <input className={field} value={vars[v.key] ?? ""} onChange={(e) => setVars((s) => ({ ...s, [v.key]: e.target.value }))} />
          )}
          {v.description && <p className="mt-1 text-xs text-zinc-600">{v.description}</p>}
        </div>
      ))}
      {error && (
        <p className="rounded-lg border border-rose-500/30 bg-rose-500/10 px-3 py-2 text-sm text-rose-300">{error}</p>
      )}
      <button
        type="submit"
        disabled={submitting}
        className="w-full rounded-lg bg-emerald-500 px-4 py-2.5 text-sm font-semibold text-zinc-950 transition hover:bg-emerald-400 disabled:opacity-50"
      >
        {submitting ? "Creating…" : "Create server"}
      </button>
    </form>
  );
}

// ConfigureServerModal walks the user from a game card to a created server:
// pick an edition (if the game has several) then set options.
export function ConfigureServerModal({
  group,
  onClose,
  onCreated,
}: {
  group: GameGroup;
  onClose: () => void;
  onCreated: (server: ServerSummary) => void;
}) {
  const multi = group.templates.length > 1;
  const [template, setTemplate] = useState<Template | null>(multi ? null : group.templates[0]);

  return (
    <div className="fixed inset-0 z-50 grid place-items-center bg-black/70 p-4 backdrop-blur-sm" onClick={onClose}>
      <div
        onClick={(e) => e.stopPropagation()}
        className="max-h-[90vh] w-full max-w-lg overflow-y-auto rounded-2xl border border-zinc-800 bg-zinc-950 shadow-2xl"
      >
        <header className="flex items-center gap-3 border-b border-zinc-800 px-6 py-4">
          <div className={`grid h-11 w-11 shrink-0 place-items-center rounded-xl bg-gradient-to-br ${group.gradient} text-xl`}>
            {group.glyph}
          </div>
          <div className="min-w-0 flex-1">
            <h2 className="font-display font-semibold text-zinc-100">New {group.name} server</h2>
            <p className="truncate text-xs text-zinc-500">
              {template && multi ? editionLabel(group, template) : group.blurb}
            </p>
          </div>
          <button onClick={onClose} className="rounded-lg px-2 py-1 text-sm text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200">
            ✕
          </button>
        </header>
        <div className="px-6 py-5">
          {template === null ? (
            <EditionPicker group={group} onPick={setTemplate} />
          ) : (
            <OptionsForm template={template} onBack={multi ? () => setTemplate(null) : undefined} onCreated={onCreated} />
          )}
        </div>
      </div>
    </div>
  );
}
