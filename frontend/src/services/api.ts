export async function analyzeRepo(repo: string, branch: string) {
  const response = await fetch(`${import.meta.env.VITE_API_BASE}/analyzeRepo`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ repo, branch }),
  });

  if (!response.ok) {
    throw new Error("Error al analizar el repositorio");
  }

  return response.json();
}
