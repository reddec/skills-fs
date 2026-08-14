// Typed client for the /api/v1 admin API. Errors carry the server's message.
const BASE = "/api/v1";

export type Metadata = Record<string, string>;

export interface SkillSummary {
  id: number;
  name: string;
  description: string;
  license: string;
  compatibility: string;
  allowedTools: string;
  metadata: Metadata;
  createdAt: string;
  updatedAt: string;
}

export interface Skill extends SkillSummary {
  body: string;
}

export interface SkillWrite {
  name: string;
  description: string;
  body: string;
  license?: string;
  compatibility?: string;
  allowedTools?: string;
  metadata?: Metadata;
}

export interface Token {
  id: number;
  label: string;
  prefix: string;
  lastUsedAt: string | null;
  createdAt: string;
}

export interface TokenCreated extends Token {
  token: string;
}

export interface ServerConfig {
  llm: {
    enabled: boolean;
    model: string;
  };
}

export type GenerationStatus = "running" | "done" | "error";

export interface Generation {
  id: string;
  status: GenerationStatus;
  error?: string;
  skillId?: number;
  skillName?: string;
  createdAt: string;
  updatedAt: string;
}

export interface GenerationCreated {
  id: string;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const resp = await fetch(BASE + path, {
    ...init,
    headers: { "Content-Type": "application/json", ...(init?.headers ?? {}) },
  });
  const text = await resp.text();
  const data = text ? JSON.parse(text) : null;
  if (!resp.ok) {
    const message = (data && typeof data === "object" && "message" in data ? String((data as { message: unknown }).message) : null) ?? resp.statusText;
    throw new Error(message);
  }
  return data as T;
}

export const api = {
  listSkills: () => request<SkillSummary[]>("/skills"),
  getSkill: (id: number) => request<Skill>(`/skills/${id}`),
  createSkill: (body: SkillWrite) => request<Skill>("/skills", { method: "POST", body: JSON.stringify(body) }),
  updateSkill: (id: number, body: SkillWrite) => request<Skill>(`/skills/${id}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteSkill: (id: number) => request<void>(`/skills/${id}`, { method: "DELETE" }),

  listTokens: () => request<Token[]>("/tokens"),
  createToken: (label: string) => request<TokenCreated>("/tokens", { method: "POST", body: JSON.stringify({ label }) }),
  deleteToken: (id: number) => request<void>(`/tokens/${id}`, { method: "DELETE" }),

  getConfig: () => request<ServerConfig>("/config"),
  createGeneration: (idea: string) => request<GenerationCreated>("/generate", { method: "POST", body: JSON.stringify({ idea }) }),
  getGeneration: (id: string) => request<Generation>(`/generate/${id}`),
};
