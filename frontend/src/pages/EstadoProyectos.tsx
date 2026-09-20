import { useEffect, useState } from "react";
import { getProjects, getProjectLogs, redeployProject, deleteProject } from "../services/api";

export default function EstadoProyectos() {
  const [proyectos, setProyectos] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

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
                  <button onClick={() => handleLogs(p.id)} style={{ marginRight: "5px" }}>Logs</button>
                  <button onClick={() => handleRedeploy(p.id)} style={{ marginRight: "5px" }}>Redeploy</button>
                  <button onClick={() => handleDelete(p.id)} style={{ color: "red" }}>Eliminar</button>
                </td>
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  );
}
