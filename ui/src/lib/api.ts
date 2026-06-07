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

export interface Relay {
  installed: boolean;
  linked: boolean;
  running: boolean;
  setupUrl: string;
  dashboardUrl: string;
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
  steamAppId?: number;
  cover?: string;
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

export interface Connectivity {
  running: boolean;
  port: number;
  protocol: string;
  externalIP?: string;
  externalAddress?: string;
  localIP?: string;
  upnpAvailable: boolean;
  forwarded: boolean;
}

export interface Reachable {
  open: boolean;
  checked: boolean;
  detail: string;
}

export interface FileEntry {
  name: string;
  isDir: boolean;
  size: number;
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
  cpus?: number; // CPU-core cap; 0/absent = uncapped
  dataPath: string;
  commandMethod: string;
  createdAt: string;
  status: string;
  running: boolean;
  // Connectivity (populated for running servers once UPnP discovery resolves).
  externalAddress?: string; // public "host:port" friends connect to
  shared?: boolean; // port currently forwarded on the router
  relayAddress?: string; // playit relay address the user pasted back for sharing
  restartAt?: string; // daily auto-restart time "HH:MM" (local), "" = off
  backupAt?: string; // daily backup time "HH:MM" (local), "" = off
  pulling?: boolean; // first-start image download in progress
  pullPercent?: number;
  pullStatus?: string;
}

export interface BackupInfo {
  name: string;
  size: number;
}

export interface AuthStatus {
  authenticated: boolean;
  hasPassword: boolean;
  loopback: boolean;
}

export interface RemoteAccess {
  enabled: boolean;
  port: number;
  addr?: string;
  hasPassword: boolean;
  externalIP?: string;
}

export interface LicenseInfo {
  tier: string;
  email?: string;
  pro: boolean;
}

export interface Stats {
  cpuPercent: number;
  memUsedMB: number;
  memLimitMB: number;
  memPercent: number;
}

export interface CreateServerRequest {
  templateId: string;
  name: string;
  memoryMB: number;
  cpus?: number; // CPU-core cap; 0/omitted = uncapped
  port: number;
  variables: Record<string, string>;
}

export interface UpdateServerRequest {
  name: string;
  memoryMB: number;
  cpus?: number; // CPU-core cap; 0/omitted = keep current
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
  authStatus: () => get<AuthStatus>("/api/auth/status"),
  login: (password: string) => send<{ status: string; token: string }>("POST", "/api/auth/login", { password }),
  logout: () => send<{ status: string }>("POST", "/api/auth/logout"),
  setPassword: (newPassword: string, currentPassword?: string) =>
    send<{ status: string }>("POST", "/api/auth/password", { newPassword, currentPassword }),
  remoteAccess: () => get<RemoteAccess>("/api/system/remote-access"),
  setRemoteAccess: (enabled: boolean, port?: number) =>
    send<RemoteAccess>("POST", "/api/system/remote-access", { enabled, port }),
  license: () => get<LicenseInfo>("/api/license"),
  setLicense: (key: string) => send<LicenseInfo>("POST", "/api/license", { key }),
  clearLicense: () => send<LicenseInfo>("DELETE", "/api/license"),
  runtime: () => get<Runtime>("/api/system/runtime"),
  setup: () => get<Setup>("/api/system/setup"),
  // endpoint is the absolute API path from a SetupStep's action (e.g. /api/system/setup/start-docker).
  runSetupStep: (endpoint: string) => send<SetupActionResult>("POST", endpoint),
  network: () => get<Network>("/api/system/network"),
  relay: () => get<Relay>("/api/system/relay"),
  relayAction: (action: string) => send<{ status: string }>("POST", `/api/system/relay/${action}`),
  relayLink: (secret: string) => send<{ status: string }>("POST", "/api/system/relay/link", { secret }),
  setRelayAddress: (id: string, address: string) =>
    send<{ status: string }>("PUT", `/api/servers/${id}/relay-address`, { address }),
  connectivity: (id: string) => get<Connectivity>(`/api/servers/${id}/connectivity`),
  testConnectivity: (id: string) => send<Reachable>("POST", `/api/servers/${id}/connectivity/test`),
  listFiles: (id: string, path: string) =>
    get<{ path: string; entries: FileEntry[] }>(`/api/servers/${id}/files?path=${encodeURIComponent(path)}`),
  readFile: (id: string, path: string) =>
    get<{ path: string; content: string; truncated: boolean }>(`/api/servers/${id}/files/read?path=${encodeURIComponent(path)}`),
  writeFile: (id: string, path: string, content: string) =>
    send<{ status: string }>("PUT", `/api/servers/${id}/files`, { path, content }),
  makeDir: (id: string, path: string) =>
    send<{ status: string }>("POST", `/api/servers/${id}/files/mkdir`, { path }),
  deleteFile: (id: string, path: string) =>
    send<{ status: string }>("DELETE", `/api/servers/${id}/files?path=${encodeURIComponent(path)}`),
  listBackups: (id: string) => get<{ backups: BackupInfo[] }>(`/api/servers/${id}/backups`),
  createBackup: (id: string) => send<{ file: string }>("POST", `/api/servers/${id}/backups`),
  restoreBackup: (id: string, file: string) =>
    send<{ status: string }>("POST", `/api/servers/${id}/backups/restore`, { file }),
  deleteBackup: (id: string, file: string) =>
    send<{ status: string }>("DELETE", `/api/servers/${id}/backups?file=${encodeURIComponent(file)}`),
  setSchedule: (id: string, restartAt: string, backupAt: string) =>
    send<{ status: string }>("PUT", `/api/servers/${id}/schedule`, { restartAt, backupAt }),
  stats: (id: string) => get<Stats>(`/api/servers/${id}/stats`),
  templates: () => get<Template[]>("/api/templates"),
  servers: () => get<ServerSummary[]>("/api/servers"),
  createServer: (req: CreateServerRequest) => send<ServerSummary>("POST", "/api/servers", req),
  updateServer: (id: string, req: UpdateServerRequest) =>
    send<ServerSummary>("PATCH", `/api/servers/${id}`, req),
  startServer: (id: string) => send<{ status: string }>("POST", `/api/servers/${id}/start`),
  stopServer: (id: string) => send<{ status: string }>("POST", `/api/servers/${id}/stop`),
  deleteServer: (id: string) => send<{ status: string }>("DELETE", `/api/servers/${id}`),
  consoleURL: (id: string) => `${ENGINE_WS}/api/servers/${id}/console`,
};
