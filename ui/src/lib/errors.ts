// Maps raw backend / Docker error strings to short, gamer-readable messages with
// a next step. Falls back to the original text so details are never hidden.

type Rule = { match: RegExp; message: string };

const RULES: Rule[] = [
  {
    match: /docker daemon|cannot connect to the docker|docker.*not running|is the docker daemon running/i,
    message: "Docker isn't running. Open Docker Desktop, wait for it to start, then try again.",
  },
  {
    match: /port is already allocated|address already in use|bind: |ports are not available|already in use/i,
    message: "That port is already in use by another server or app. Pick a different port.",
  },
  {
    match: /no space left on device|disk.*full/i,
    message: "Your disk is full. Free up some space and try again.",
  },
  {
    match: /pull access denied|manifest unknown|manifest.*not found|repository.*not found|no such image|image.*not found/i,
    message: "Couldn't find the game files to download. Check your internet connection and try again.",
  },
  {
    match: /pull|registry|failed to download|i\/o timeout/i,
    message: "Couldn't download the game files. Check your internet connection and try again.",
  },
  {
    match: /permission denied|access is denied|operation not permitted/i,
    message: "Permission denied. Try running GameNest as administrator.",
  },
  {
    match: /email not confirmed|email_not_confirmed/i,
    message: "Please confirm your email — check your inbox for the confirmation link.",
  },
  {
    match: /authentication required|unauthorized|\b401\b/i,
    message: "You're signed out. Sign in again and retry.",
  },
  {
    match: /too many (login )?attempts|\b429\b/i,
    message: "Too many attempts. Wait a minute, then try again.",
  },
  {
    match: /request entity too large|\b413\b|too (big|large)/i,
    message: "That file or request is too large.",
  },
  {
    match: /failed to fetch|networkerror|load failed|connection refused|econnrefused|engine (at|not)/i,
    message: "Can't reach the GameNest engine. Make sure GameNest is still running, then retry.",
  },
  {
    match: /out of memory|\boom\b|cannot allocate memory/i,
    message: "Not enough memory to run this. Lower the server's memory or close other apps.",
  },
  {
    match: /context deadline exceeded|timeout|timed out/i,
    message: "That took too long and timed out. Try again in a moment.",
  },
];

// rawMessage extracts a string from any thrown value.
function rawMessage(e: unknown): string {
  if (e instanceof Error) return e.message;
  if (typeof e === "string") return e;
  try {
    return String(e);
  } catch {
    return "Unknown error";
  }
}

// friendlyError turns any thrown value into a short, human-friendly message.
// Unmatched errors fall through to their original text so nothing is lost.
export function friendlyError(e: unknown): string {
  const raw = rawMessage(e).trim();
  for (const r of RULES) {
    if (r.match.test(raw)) return r.message;
  }
  return raw || "Something went wrong. Please try again.";
}
