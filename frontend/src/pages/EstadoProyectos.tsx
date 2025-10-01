import { useEffect, useState } from "react";

export default function EstadoProyectos() {
  const [proyectos, setProyectos] = useState<any[]>([]);

  const fetchData = () => {
    fetch(`${import.meta.env.VITE_API_BASE}/history`)
      .then((res) => res.json())
      .then((data) => setProyectos(data))
      .catch(() => setProyectos([]));
  };

  useEffect(() => {
    fetchData();
  }, []);

  const handleLogs = async (id: string) => {
    const res = await fetch(`${import.meta.env.VITE_API_BASE}/logs/${id}`);
    const data = await res.json();
    alert(data.logs);
  };

  const handleRedeploy = async (id: string) => {
    const res = await fetch(`${import.meta.env.VITE_API_BASE}/redeploy/${id}`, { method: "POST" });
    const data = await res.json();
    alert(data.status);
  };

  const handleDelete = async (id: string) => {
    await fetch(`${import.meta.env.VITE_API_BASE}/history/${id}`, { method: "DELETE" });
    fetchData();
  };

  return (
    <div style={{ padding: "20px" }}>
      <h2>Estado de Proyectos</h2>
      <table border={1} cellPadding={5}>
        <thead>
          <tr>
            <th>Repo</th>
            <th>Branch</th>
            <th>Tipo</th>
            <th>Estado</th>
            <th>Acciones</th>
          </tr>
        </thead>
        <tbody>
          {proyectos.map((p) => (
            <tr key={p.id}>
              <td>{p.repo}</td>
              <td>{p.branch}</td>
              <td>{p.type}</td>
              <td>{p.status}</td>
              <td>
                <button onClick={() => handleLogs(p.id)}>Logs</button>
                <button onClick={() => handleRedeploy(p.id)}>Redeploy</button>
                <button onClick={() => handleDelete(p.id)}>Eliminar</button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
