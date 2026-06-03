// Typed client for the GameHost engine API.
// In dev the engine listens on loopback; later the desktop shell will inject
// the real address. Override here if you change GAMEHOST_ADDR.
export const ENGINE_BASE = "http://127.0.0.1:8723";

export interface Health {
  status: string;
  service: string;
  version: string;
}

export interface Runtime {
  connected: boolean;
  serverVersion?: string;
  apiVersion?: string;
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
  minMemoryMB: number;
  recMemoryMB: number;
  ports: Port[];
  env: Record<string, string>;
  variables: Variable[];
}

async function get<T>(path: string): Promise<T> {
  const res = await fetch(`${ENGINE_BASE}${path}`);
  if (!res.ok) throw new Error(`Request to ${path} failed (${res.status})`);
  return (await res.json()) as T;
}

export const api = {
  base: ENGINE_BASE,
  health: () => get<Health>("/api/health"),
  runtime: () => get<Runtime>("/api/system/runtime"),
  templates: () => get<Template[]>("/api/templates"),
};
