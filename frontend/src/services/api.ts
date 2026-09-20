const API_BASE = import.meta.env.VITE_API_BASE || "/api";
const STORAGE_KEY = "sammcore_deployer_api_key";

export function getApiKey(): string {
  return localStorage.getItem(STORAGE_KEY) || "";
}

export function setApiKey(key: string): void {
  localStorage.setItem(STORAGE_KEY, key.trim());
}

export async function authFetch(endpoint: string, options: RequestInit = {}): Promise<Response> {
  const url = endpoint.startsWith("http") ? endpoint : `${API_BASE}${endpoint}`;
  const key = getApiKey();

  const headers = new Headers(options.headers || {});
  if (!headers.has("Content-Type") && !(options.body instanceof FormData)) {
    headers.set("Content-Type", "application/json");
  }
  if (key) {
    headers.set("Authorization", `Bearer ${key}`);
  }

  const response = await fetch(url, { ...options, headers });

  if (response.status === 401) {
    const userKey = prompt("Autenticación requerida. Ingrese la DEPLOYER_API_KEY:");
    if (userKey) {
      setApiKey(userKey);
      headers.set("Authorization", `Bearer ${userKey.trim()}`);
      return fetch(url, { ...options, headers });
    }
  }

  return response;
}

export async function analyzeRepo(repo: string, branch: string) {
  const res = await authFetch("/analyzeRepo", {
    method: "POST",
    body: JSON.stringify({ repo, branch }),
  });

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: "Error de red al analizar repositorio" }));
    throw new Error(err.error || `Error ${res.status}`);
  }

  return res.json();
}

export async function getProjects() {
  const res = await authFetch("/projects");
  if (!res.ok) {
    throw new Error(`Error al cargar proyectos: ${res.statusText}`);
  }
  return res.json();
}

export async function getProjectLogs(id: string) {
  const res = await authFetch(`/projects/${id}/logs`);
  if (!res.ok) {
    throw new Error(`Error al obtener logs: ${res.statusText}`);
  }
  return res.json();
}

export async function redeployProject(id: string) {
  const res = await authFetch(`/projects/${id}/redeploy`, { method: "POST" });
  const data = await res.json();
  if (!res.ok) {
    throw new Error(data.error || `Error ${res.status}`);
  }
  return data;
}

export async function deleteProject(id: string, deleteDB: boolean = false) {
  const res = await authFetch(`/projects/${id}?delete_db=${deleteDB}`, { method: "DELETE" });
  const data = await res.json();
  if (!res.ok) {
    throw new Error(data.error || `Error ${res.status}`);
  }
  return data;
}
