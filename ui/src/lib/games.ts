// Groups the flat template list into "games" (by the template `game` field) so
// the library shows one card per game. A game with several templates (e.g.
// Minecraft: Java, Bedrock, modpacks) becomes one card whose editions the user
// picks when configuring.
import type { Template } from "./api";

export interface GameMeta {
  name: string;
  blurb: string;
  category: string;
  gradient: string; // tailwind from-…/to-… for the icon tile
  glyph: string; // emoji shown on the tile
}

export interface GameGroup extends GameMeta {
  game: string;
  templates: Template[];
}

// Curated display metadata for the known games. Unknown games fall back to
// values derived from their template.
const META: Record<string, GameMeta> = {
  minecraft: {
    name: "Minecraft",
    blurb: "Java or Bedrock, Vanilla/Paper/Fabric/Forge, or any modpack — pick an edition.",
    category: "Sandbox",
    gradient: "from-lime-500 to-emerald-700",
    glyph: "⛏️",
  },
  valheim: {
    name: "Valheim",
    blurb: "Co-op Viking survival in a brutal procedural world.",
    category: "Survival",
    gradient: "from-amber-500 to-orange-700",
    glyph: "🛡️",
  },
  palworld: {
    name: "Palworld",
    blurb: "Creature-collecting open-world survival, up to 32 players.",
    category: "Survival",
    gradient: "from-sky-500 to-blue-700",
    glyph: "🐾",
  },
  cs2: {
    name: "Counter-Strike 2",
    blurb: "Competitive 5v5 shooter on a dedicated server.",
    category: "Shooter",
    gradient: "from-rose-500 to-red-700",
    glyph: "🎯",
  },
  factorio: {
    name: "Factorio",
    blurb: "Build and defend sprawling automated factories.",
    category: "Sandbox",
    gradient: "from-orange-500 to-amber-700",
    glyph: "⚙️",
  },
  satisfactory: {
    name: "Satisfactory",
    blurb: "First-person co-op factory building on an alien planet.",
    category: "Sandbox",
    gradient: "from-orange-400 to-yellow-600",
    glyph: "🏭",
  },
  enshrouded: {
    name: "Enshrouded",
    blurb: "Survival-crafting voxel action-RPG for up to 16 players.",
    category: "Survival",
    gradient: "from-violet-500 to-purple-700",
    glyph: "🌫️",
  },
};

// A few deterministic gradients for games we don't have curated metadata for.
const FALLBACK_GRADIENTS = [
  "from-teal-500 to-cyan-700",
  "from-fuchsia-500 to-pink-700",
  "from-indigo-500 to-blue-700",
  "from-emerald-500 to-teal-700",
];

function deriveMeta(game: string, first: Template): GameMeta {
  // Strip any parenthetical / "Modpack" qualifier to get a clean game name.
  const name = first.name.replace(/\s*\(.*\)\s*$/, "").replace(/\s+Modpack.*$/, "").trim() || first.name;
  let hash = 0;
  for (const c of game) hash = (hash + c.charCodeAt(0)) % FALLBACK_GRADIENTS.length;
  return {
    name,
    blurb: first.description,
    category: first.category,
    gradient: FALLBACK_GRADIENTS[hash],
    glyph: (name[0] ?? "?").toUpperCase(),
  };
}

export function groupGames(templates: Template[]): GameGroup[] {
  const byGame = new Map<string, Template[]>();
  for (const t of templates) {
    const list = byGame.get(t.game) ?? [];
    list.push(t);
    byGame.set(t.game, list);
  }
  const groups: GameGroup[] = [];
  for (const [game, list] of byGame) {
    // Base editions before modpacks, then by name.
    list.sort((a, b) => {
      const am = a.category === "Modded" ? 1 : 0;
      const bm = b.category === "Modded" ? 1 : 0;
      return am !== bm ? am - bm : a.name.localeCompare(b.name);
    });
    const meta = META[game] ?? deriveMeta(game, list[0]);
    groups.push({ game, templates: list, ...meta });
  }
  // Flagship first, then alphabetical.
  groups.sort((a, b) => (a.game === "minecraft" ? -1 : b.game === "minecraft" ? 1 : a.name.localeCompare(b.name)));
  return groups;
}

// A short edition label for a template within a game group (e.g. the Minecraft
// template "Minecraft (Java • Paper)" -> "Java • Paper").
export function editionLabel(group: GameGroup, t: Template): string {
  let s = t.name.replace(group.name, "").trim();
  if (s.startsWith("(") && s.endsWith(")")) s = s.slice(1, -1).trim(); // "(Java • Paper)" -> "Java • Paper"
  return s || t.name;
}
