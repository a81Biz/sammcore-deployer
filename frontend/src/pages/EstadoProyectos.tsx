import { useEffect, useState } from "react";
import { getProjects, getProjectLogs, redeployProject, deleteProject, getProjectMetrics } from "../services/api";

export default function EstadoProyectos() {
  const [proyectos, setProyectos] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [metricsData, setMetricsData] = useState<any | null>(null);
  const [loadingMetrics, setLoadingMetrics] = useState(false);

  const fetchData = () => {
    setLoading(true);
    setError(null);
    getProjects()
      .then((data) => {
        setProyectos(Array.isArray(data) ? data : []);
        setLoading(false);
      })
      .catch((err) => {
        setError(err.message);
        setProyectos([]);
        setLoading(false);
      });
  };

  useEffect(() => {
    fetchData();
  }, []);

  const handleLogs = async (id: string) => {
    try {
      const data = await getProjectLogs(id);
      alert(data.logs);
    } catch (err: any) {
      alert(`Error: ${err.message}`);
    }
  };

  const handleMetrics = async (id: string) => {
    setLoadingMetrics(true);
    setMetricsData(null);
    try {
      const data = await getProjectMetrics(id);
      setMetricsData(data);
    } catch (err: any) {
      alert(`Error al obtener métricas: ${err.message}`);
    } finally {
      setLoadingMetrics(false);
    }
  };

  const handleRedeploy = async (id: string) => {
    try {
      const data = await redeployProject(id);
      alert(data.status || "Redeploy solicitado");
    } catch (err: any) {
      alert(`Error: ${err.message}`);
    }
  };

  const handleDelete = async (id: string) => {
    const confirmDelete = window.confirm("¿Desea eliminar este proyecto?");
    if (!confirmDelete) return;

    const deleteDB = window.confirm("¿Desea eliminar también la base de datos en PostgreSQL?\n- Aceptar = Eliminar BD permanentemente\n- Cancelar = Conservar BD");
    try {
      await deleteProject(id, deleteDB);
      fetchData();
    } catch (err: any) {
      alert(`Error al eliminar: ${err.message}`);
    }
  };

  return (
    <div style={{ padding: "20px" }}>
      <h2>Estado de Proyectos</h2>
      {loading && <p>Cargando proyectos...</p>}
      {error && <p style={{ color: "red" }}>Error: {error}</p>}
      <table border={1} cellPadding={5} style={{ width: "100%", textAlign: "left", marginTop: "10px" }}>
        <thead>
          <tr>
            <th>Nombre</th>
            <th>Repo</th>
            <th>Branch</th>
            <th>Tipo</th>
            <th>Estado</th>
            <th>Subdominio</th>
            <th>Acciones</th>
          </tr>
        </thead>
        <tbody>
          {proyectos.length === 0 && !loading ? (
            <tr>
              <td colSpan={7} style={{ textAlign: "center" }}>No hay proyectos registrados.</td>
            </tr>
          ) : (
            proyectos.map((p) => (
              <tr key={p.id}>
                <td><strong>{p.name || p.id}</strong></td>
                <td>{p.repo}</td>
                <td>{p.branch}</td>
                <td><code>{p.type}</code></td>
                <td>{p.status}</td>
                <td>
                  {p.domain ? (
                    <a href={`https://${p.domain}`} target="_blank" rel="noreferrer">
                      {p.domain}
                    </a>
                  ) : "-"}
                </td>
                <td>
                  <button onClick={() => handleMetrics(p.id)} style={{ marginRight: "5px" }}>📊 Métricas</button>
                  <button onClick={() => handleLogs(p.id)} style={{ marginRight: "5px" }}>Logs</button>
                  <button onClick={() => handleRedeploy(p.id)} style={{ marginRight: "5px" }}>Redeploy</button>
                  <button onClick={() => handleDelete(p.id)} style={{ color: "red" }}>Eliminar</button>
                </td>
              </tr>
            ))
          )}
        </tbody>
      </table>

      {loadingMetrics && <p style={{ marginTop: "20px" }}>Cargando métricas de infraestructura en tiempo real...</p>}

      {metricsData && (
        <div style={{ marginTop: "25px", padding: "15px", border: "1px solid #007acc", borderRadius: "6px", backgroundColor: "#f0f7ff" }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
            <h3 style={{ margin: 0 }}>📊 Métricas de Infraestructura: {metricsData.project_name}</h3>
            <button onClick={() => setMetricsData(null)} style={{ cursor: "pointer" }}>Cerrar</button>
          </div>
          <p style={{ margin: "5px 0" }}>
            <strong>Namespace:</strong> <code>{metricsData.namespace}</code> | <strong>Estado:</strong> {metricsData.status} | <strong>Total Pods:</strong> {metricsData.pods_count}
          </p>

          <h4 style={{ marginTop: "10px", marginBottom: "5px" }}>Pods y Consumo Dinámico:</h4>
          {metricsData.pods.length === 0 ? (
            <p>No hay pods activos en este momento.</p>
          ) : (
            <table border={1} cellPadding={4} style={{ width: "100%", backgroundColor: "#fff", textAlign: "left" }}>
              <thead>
                <tr>
                  <th>Pod</th>
                  <th>Fase</th>
                  <th>Listo</th>
                  <th>Reinicios</th>
                  <th>CPU</th>
                  <th>Memoria</th>
                </tr>
              </thead>
              <tbody>
                {metricsData.pods.map((pod: any) => (
                  <tr key={pod.name}>
                    <td><code>{pod.name}</code></td>
                    <td>{pod.status}</td>
                    <td>{pod.ready ? "✅ Sí" : "❌ No"}</td>
                    <td>{pod.restarts}</td>
                    <td><strong>{pod.cpu_usage || "0m"}</strong></td>
                    <td><strong>{pod.memory_usage || "0Mi"}</strong></td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}

          {metricsData.quota_limits && Object.keys(metricsData.quota_limits).length > 0 && (
            <div style={{ marginTop: "15px" }}>
              <h4 style={{ margin: "5px 0" }}>Límites y Uso de ResourceQuota:</h4>
              <ul style={{ margin: "5px 0", paddingLeft: "20px" }}>
                {Object.keys(metricsData.quota_limits).map((k) => (
                  <li key={k}>
                    <strong>{k}:</strong> Uso <code>{metricsData.quota_usage[k] || "0"}</code> / Límite <code>{metricsData.quota_limits[k]}</code>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
