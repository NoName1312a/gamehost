// Typed client for the GameHost engine API.
// In dev the engine listens on loopback; later the desktop shell injects the
// real address.
export const ENGINE_BASE = "http://127.0.0.1:8723";
export const ENGINE_WS = "ws://127.0.0.1:8723";

export interface Health {
  status: string;
  service: string;
  version: string;
}

export interface Runtime {
  connected: boolean;
  serverVersion?: string;
  message: string;
}

export interface SetupAction {
  label: string;
  endpoint: string;
  command: string;
  needsAdmin: boolean;
  needsReboot: boolean;
}

export interface SetupStep {
  id: string;
  title: string;
  status: "ok" | "todo";
  detail: string;
  action?: SetupAction;
}

export interface Setup {
  platform: string;
  ready: boolean;
  steps: SetupStep[];
}

export interface SetupActionResult {
  started: boolean;
  needsReboot?: boolean;
  hint: string;
}

export interface Network {
  upnpAvailable: boolean;
  externalIP?: string;
  message: string;
}

export interface Port {
  name: string;
  container: number;
  protocol: string;
  default: number;
}

export interface Variable {
  key: string;
  label: string;
  description?: string;
  default: string;
  type: string;
  options?: string[];
  required: boolean;
}

export interface Template {
  id: string;
  name: string;
  game: string;
  category: string;
  description: string;
  icon: string;
  image: string;
  runtime: string;
  stopCommand: string;
  dataPath: string;
  commandMethod: string;
  minMemoryMB: number;
  recMemoryMB: number;
  ports: Port[];
  env: Record<string, string>;
  variables: Variable[];
}

export interface PortMapping {
  host: number;
  container: number;
  protocol: string;
}

export interface ServerSummary {
  id: string;
  name: string;
  templateId: string;
  game: string;
  image: string;
  env: Record<string, string>;
  ports: PortMapping[];
  memoryMB: number;
  dataPath: string;
  commandMethod: string;
  createdAt: string;
  status: string;
  running: boolean;
  // Connectivity (populated for running servers once UPnP discovery resolves).
  externalAddress?: string; // public "host:port" friends connect to
  shared?: boolean; // port currently forwarded on the router
}

export interface CreateServerRequest {
  templateId: string;
  name: string;
  memoryMB: number;
  port: number;
  variables: Record<string, string>;
}

async function parseError(res: Response, path: string): Promise<string> {
  try {
    const j = await res.json();
    if (j && typeof j.error === "string") return j.error;
  } catch {
    /* not JSON */
  }
  return `Request to ${path} failed (${res.status})`;
}

async function get<T>(path: string): Promise<T> {
  const res = await fetch(`${ENGINE_BASE}${path}`);
  if (!res.ok) throw new Error(await parseError(res, path));
  return (await res.json()) as T;
}

async function send<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${ENGINE_BASE}${path}`, {
    method,
    headers: body !== undefined ? { "Content-Type": "application/json" } : undefined,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) throw new Error(await parseError(res, path));
  const text = await res.text();
  return (text ? JSON.parse(text) : null) as T;
}

export const api = {
  base: ENGINE_BASE,
  health: () => get<Health>("/api/health"),
  runtime: () => get<Runtime>("/api/system/runtime"),
  setup: () => get<Setup>("/api/system/setup"),
  // endpoint is the absolute API path from a SetupStep's action (e.g. /api/system/setup/start-docker).
  runSetupStep: (endpoint: string) => send<SetupActionResult>("POST", endpoint),
  network: () => get<Network>("/api/system/network"),
  templates: () => get<Template[]>("/api/templates"),
  servers: () => get<ServerSummary[]>("/api/servers"),
  createServer: (req: CreateServerRequest) => send<ServerSummary>("POST", "/api/servers", req),
  startServer: (id: string) => send<{ status: string }>("POST", `/api/servers/${id}/start`),
  stopServer: (id: string) => send<{ status: string }>("POST", `/api/servers/${id}/stop`),
  deleteServer: (id: string) => send<{ status: string }>("DELETE", `/api/servers/${id}`),
  consoleURL: (id: string) => `${ENGINE_WS}/api/servers/${id}/console`,
};
