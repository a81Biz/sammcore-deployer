import { useState } from "react";
import { analyzeRepo } from "../services/api";

export default function RegistrarProyecto() {
  const [repo, setRepo] = useState("");
  const [branch, setBranch] = useState("main");
  const [result, setResult] = useState<any>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      const data = await analyzeRepo(repo, branch);
      setResult(data);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ padding: "20px" }}>
      <h2>Registrar Proyecto</h2>
      <form onSubmit={handleSubmit}>
        <input
          type="text"
          placeholder="Repo URL"
          value={repo}
          onChange={(e) => setRepo(e.target.value)}
          style={{ marginRight: "10px" }}
        />
        <input
          type="text"
          placeholder="Branch"
          value={branch}
          onChange={(e) => setBranch(e.target.value)}
          style={{ marginRight: "10px" }}
        />
        <button type="submit" disabled={loading}>
          {loading ? "Analizando..." : "Analizar"}
        </button>
      </form>

      {error && <p style={{ color: "red" }}>Error: {error}</p>}
      {result && (
        <pre style={{ marginTop: "20px", background: "#eee", padding: "10px" }}>
          {JSON.stringify(result, null, 2)}
        </pre>
      )}
    </div>
  );
}
