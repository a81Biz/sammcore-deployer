const API_BASE = (import.meta.env.VITE_API_BASE || "/api").replace(/\/$/, "");
const STORAGE_KEY = "sammcore_deployer_api_key";

export function getApiKey(): string {
  return localStorage.getItem(STORAGE_KEY) || "";
}

export function setApiKey(key: string): void {
  const trimmed = key.trim();
  if (trimmed) {
    localStorage.setItem(STORAGE_KEY, trimmed);
  } else {
    localStorage.removeItem(STORAGE_KEY);
  }
}

export async function authFetch(endpoint: string, options: RequestInit = {}): Promise<Response> {
  const cleanEndpoint = endpoint.startsWith("/") ? endpoint : `/${endpoint}`;
  const url = cleanEndpoint.startsWith("http") ? cleanEndpoint : `${API_BASE}${cleanEndpoint}`;
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
    // Disparar evento para que la UI abra el modal de configuración de clave
    if (typeof window !== "undefined") {
      window.dispatchEvent(new CustomEvent("deployer:auth-required", {
        detail: { message: "Se requiere autenticación. Por favor ingrese su DEPLOYER_API_KEY." }
      }));
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
    const err = await res.json().catch(() => ({ error: "Error al cargar proyectos" }));
    throw new Error(err.error || `Error ${res.status}`);
  }
  return res.json();
}

export async function getProjectLogs(id: string) {
  const res = await authFetch(`/projects/${id}/logs`);
  const data = await res.json().catch(() => ({ error: "Error al obtener logs" }));
  if (!res.ok) {
    throw new Error(data.error || `Error ${res.status}`);
  }
  return data;
}

export async function redeployProject(id: string) {
  const res = await authFetch(`/projects/${id}/redeploy`, { method: "POST" });
  const data = await res.json().catch(() => ({ error: "Error al re-desplegar" }));
  if (!res.ok) {
    throw new Error(data.error || `Error ${res.status}`);
  }
  return data;
}

export async function deleteProject(id: string, deleteDB: boolean = false) {
  const res = await authFetch(`/projects/${id}?delete_db=${deleteDB}`, { method: "DELETE" });
  const data = await res.json().catch(() => ({ error: "Error al eliminar proyecto" }));
  if (!res.ok) {
    throw new Error(data.error || `Error ${res.status}`);
  }
  return data;
}
