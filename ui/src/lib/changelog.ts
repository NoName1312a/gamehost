// Parses the repo-root CHANGELOG.md (bundled at build time) into structured
// entries the app can render. CHANGELOG.md stays the single source of truth —
// it also feeds the GitHub release notes via scripts/release.ps1.
import raw from "../../../CHANGELOG.md?raw";

export interface ChangeGroup {
  type: string; // "Added" | "Changed" | "Fixed" | "Removed" | "Security" | …
  items: string[];
}

export interface ChangelogEntry {
  version: string; // e.g. "0.4.2"
  date?: string; // e.g. "2026-06-14"
  groups: ChangeGroup[];
}

// Matches a version heading: "## [0.4.2] - 2026-06-14" (date optional).
const VERSION_RE = /^##\s*\[([^\]]+)\](?:\s*-\s*(.+))?\s*$/;
const GROUP_RE = /^###\s+(.+?)\s*$/;
const ITEM_RE = /^[-*]\s+(.+?)\s*$/;

function parse(md: string): ChangelogEntry[] {
  const entries: ChangelogEntry[] = [];
  let entry: ChangelogEntry | null = null;
  let group: ChangeGroup | null = null;

  for (const line of md.split(/\r?\n/)) {
    const v = VERSION_RE.exec(line);
    if (v) {
      // Skip the rolling "Unreleased" section — only show shipped versions.
      if (/unreleased/i.test(v[1])) {
        entry = null;
        group = null;
        continue;
      }
      entry = { version: v[1].trim(), date: v[2]?.trim(), groups: [] };
      group = null;
      entries.push(entry);
      continue;
    }
    if (!entry) continue;
    const g = GROUP_RE.exec(line);
    if (g) {
      group = { type: g[1].trim(), items: [] };
      entry.groups.push(group);
      continue;
    }
    const it = ITEM_RE.exec(line);
    if (it && group) group.items.push(it[1].trim());
  }
  return entries;
}

// All shipped versions, newest first (CHANGELOG lists them that way).
export const changelog: ChangelogEntry[] = parse(raw);

// Compares two dotted numeric versions; returns >0 if a is newer than b.
export function cmpVersion(a: string, b: string): number {
  const pa = a.split(".").map((n) => parseInt(n, 10) || 0);
  const pb = b.split(".").map((n) => parseInt(n, 10) || 0);
  for (let i = 0; i < Math.max(pa.length, pb.length); i++) {
    const d = (pa[i] ?? 0) - (pb[i] ?? 0);
    if (d !== 0) return d;
  }
  return 0;
}

// Entries strictly newer than `version` — i.e. what changed since the user was
// last on that version.
export function entriesSince(version: string): ChangelogEntry[] {
  return changelog.filter((e) => cmpVersion(e.version, version) > 0);
}
