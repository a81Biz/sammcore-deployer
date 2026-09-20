import { useEffect, useState, useRef } from "react";
import { getProjects, getProjectLogs, redeployProject, deleteProject, getProjectMetrics, Project } from "../services/api";
import Modal from "../components/Modal";
import DeploymentProgress from "../components/DeploymentProgress";

export default function EstadoProyectos() {
  const [proyectos, setProyectos] = useState<Project[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Modal States
  const [modalState, setModalState] = useState<{
    isOpen: boolean;
    title: string;
    content: React.ReactNode;
    variant?: "primary" | "danger" | "info";
    confirmText?: string;
    onConfirm?: () => void;
    width?: string;
  }>({
    isOpen: false,
    title: "",
    content: null,
  });

  // Métricas
  const [metricsData, setMetricsData] = useState<any | null>(null);
  const [loadingMetrics, setLoadingMetrics] = useState(false);

  // Polling ref
  const pollTimerRef = useRef<NodeJS.Timeout | null>(null);

  const fetchData = (silent = false) => {
    if (!silent) setLoading(true);
    setError(null);
    getProjects()
      .then((data) => {
        const list = Array.isArray(data) ? data : [];
        setProyectos(list);
        if (!silent) setLoading(false);
      })
      .catch((err) => {
        setError(err.message);
        if (!silent) setLoading(false);
      });
  };

  useEffect(() => {
    fetchData();
  }, []);

  // Polling automático si hay algún proyecto en despliegue
  useEffect(() => {
    const hasActiveDeploy = proyectos.some(
      (p) => p.status === "provisioning_db" || p.status === "building_image" || p.status === "deploying_k8s"
    );

    if (hasActiveDeploy) {
      pollTimerRef.current = setTimeout(() => {
        fetchData(true);
      }, 3000);
    }

    return () => {
      if (pollTimerRef.current) clearTimeout(pollTimerRef.current);
    };
  }, [proyectos]);

  const closeModal = () => {
    setModalState((prev) => ({ ...prev, isOpen: false, onConfirm: undefined }));
  };

  const showAlert = (title: string, message: string, variant: "primary" | "danger" | "info" = "primary") => {
    setModalState({
      isOpen: true,
      title,
      content: <p style={{ margin: 0, color: "#e0e0e8" }}>{message}</p>,
      variant,
    });
  };

  const handleLogs = async (id: string, name: string) => {
    try {
      const data = await getProjectLogs(id);
      setModalState({
        isOpen: true,
        title: `📋 Logs de Pod: ${name}`,
        width: "780px",
        content: (
          <pre
            style={{
              background: "#0d0d12",
              padding: "14px",
              borderRadius: "6px",
              color: "#38bdf8",
              fontFamily: "monospace",
              fontSize: "0.85rem",
              whiteSpace: "pre-wrap",
              wordBreak: "break-all",
              maxHeight: "450px",
              overflowY: "auto",
              margin: 0,
              border: "1px solid #222230",
            }}
          >
            {data.logs || "No se encontraron logs disponibles."}
          </pre>
        ),
      });
    } catch (err: any) {
      showAlert("Error al obtener logs", err.message, "danger");
    }
  };

  const showErrorDetails = (p: Project) => {
    setModalState({
      isOpen: true,
      title: `⚠️ Detalle de Fallo: ${p.name}`,
      variant: "danger",
      width: "650px",
      content: (
        <div>
          <div style={{ marginBottom: "12px", color: "#fca5a5", fontSize: "0.95rem" }}>
            <strong>Último error registrado en el clúster:</strong>
          </div>
          <pre
            style={{
              background: "#1c1214",
              border: "1px solid #7f1d1d",
              padding: "12px",
              borderRadius: "6px",
              color: "#fca5a5",
              fontFamily: "monospace",
              fontSize: "0.85rem",
              whiteSpace: "pre-wrap",
              wordBreak: "break-word",
              maxHeight: "300px",
              overflowY: "auto",
              margin: "0 0 16px 0",
            }}
          >
            {p.last_error || "Timeout de pods sin mensaje adicional."}
          </pre>
          <div style={{ fontSize: "0.85rem", color: "#8a8a9e" }}>
            Paso donde ocurrió: <code>Paso {p.current_step || "?"} de {p.total_steps || "?"}</code> ({p.step_description || "Desconocido"})
          </div>
        </div>
      ),
      confirmText: "🔄 Reintentar Despliegue",
      onConfirm: () => {
        closeModal();
        handleRedeploy(p.id, p.name);
      },
    });
  };

  const handleMetrics = async (id: string, name: string) => {
    setLoadingMetrics(true);
    setMetricsData(null);
    try {
      const data = await getProjectMetrics(id);
      setMetricsData(data);
    } catch (err: any) {
      showAlert("Error al obtener métricas", err.message, "danger");
    } finally {
      setLoadingMetrics(false);
    }
  };

  const handleRedeploy = async (id: string, name: string) => {
    try {
      await redeployProject(id);
      showAlert("Re-despliegue Solicitado", `El proyecto ${name} ha comenzado su re-despliegue en K3s.`, "info");
      fetchData(true);
    } catch (err: any) {
      showAlert("Error al re-desplegar", err.message, "danger");
    }
  };

  const handleDeletePrompt = (p: Project) => {
    let deleteDBChecked = true;

    setModalState({
      isOpen: true,
      title: `🗑️ Confirmar Eliminación: ${p.name}`,
      variant: "danger",
      confirmText: "Eliminar Proyecto",
      content: (
        <div>
          <p style={{ margin: "0 0 16px 0", color: "#e0e0e8" }}>
            ¿Estás seguro de que deseas eliminar el proyecto <strong>{p.name}</strong> del clúster K3s?
            Se destruirá el namespace <code>{p.namespace}</code>, sus servicios, pods e ingress.
          </p>
          <div
            style={{
              background: "#161622",
              padding: "12px 14px",
              borderRadius: "6px",
              border: "1px solid #2d2d3a",
              display: "flex",
              alignItems: "center",
              gap: "10px",
            }}
          >
            <input
              type="checkbox"
              id="deleteDBCheck"
              defaultChecked={deleteDBChecked}
              onChange={(e) => {
                deleteDBChecked = e.target.checked;
              }}
              style={{ width: "18px", height: "18px", cursor: "pointer" }}
            />
            <label htmlFor="deleteDBCheck" style={{ cursor: "pointer", fontSize: "0.9rem", color: "#fca5a5" }}>
              <strong>Eliminar también la Base de Datos dedicada en PostgreSQL Supabase</strong> (<code>{p.name}_db</code> y usuario <code>{p.name}_user</code>).
            </label>
          </div>
        </div>
      ),
      onConfirm: async () => {
        closeModal();
        try {
          await deleteProject(p.id, deleteDBChecked);
          showAlert("Proyecto Eliminado", `El proyecto ${p.name} fue desmantelado correctamente.`, "primary");
          fetchData();
        } catch (err: any) {
          showAlert("Error al eliminar", err.message, "danger");
        }
      },
    });
  };

  const activeOrFailedProjects = proyectos.filter(
    (p) => p.status === "provisioning_db" || p.status === "building_image" || p.status === "deploying_k8s" || p.status === "failed"
  );

  return (
    <div style={{ maxWidth: "1100px", margin: "30px auto", padding: "0 20px", color: "#e0e0e8", fontFamily: "system-ui, sans-serif" }}>
      {/* Header */}
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "24px" }}>
        <div>
          <h1 style={{ fontSize: "1.8rem", margin: "0 0 6px 0", color: "#ffffff" }}>
            📊 Estado de Proyectos y Despliegues
          </h1>
          <p style={{ margin: 0, color: "#8a8a9e", fontSize: "0.95rem" }}>
            Supervisión en tiempo real de servicios, compilaciones de Kaniko, pods y métricas en el clúster SAMMCORE K3s.
          </p>
        </div>
        <button
          onClick={() => fetchData()}
          disabled={loading}
          style={{
            padding: "8px 16px",
            background: "#22222e",
            color: "#e0e0e8",
            border: "1px solid #3d3d4d",
            borderRadius: "6px",
            fontWeight: 600,
            cursor: loading ? "not-allowed" : "pointer",
            fontSize: "0.9rem",
          }}
        >
          {loading ? "⏳ Actualizando..." : "🔄 Actualizar"}
        </button>
      </div>

      {error && (
        <div style={{ padding: "12px 16px", background: "rgba(239, 68, 68, 0.15)", border: "1px solid #ef4444", borderRadius: "6px", color: "#fca5a5", marginBottom: "20px" }}>
          ⚠️ <strong>Error al cargar proyectos:</strong> {error}
        </div>
      )}

      {/* Widget de Progreso Unificado para proyectos activos o fallidos recientes */}
      {activeOrFailedProjects.length > 0 && (
        <div style={{ marginBottom: "28px" }}>
          <h3 style={{ margin: "0 0 12px 0", fontSize: "1.1rem", color: "#a5b4fc" }}>
            🚀 Despliegues en Curso / Notificaciones de Estado
          </h3>
          {activeOrFailedProjects.map((p) => (
            <DeploymentProgress
              key={p.id}
              project={p}
              onRetry={() => handleRedeploy(p.id, p.name)}
              onViewLogs={() => handleLogs(p.id, p.name)}
            />
          ))}
        </div>
      )}

      {/* Tabla Principal de Proyectos */}
      <div style={{ background: "#1c1c24", borderRadius: "8px", border: "1px solid #2d2d3a", overflow: "hidden", boxShadow: "0 4px 12px rgba(0,0,0,0.3)" }}>
        <table style={{ width: "100%", borderCollapse: "collapse", fontSize: "0.9rem", textAlign: "left" }}>
          <thead>
            <tr style={{ background: "#15151c", borderBottom: "1px solid #2d2d3a", color: "#8a8a9e" }}>
              <th style={{ padding: "12px 14px", fontWeight: 600 }}>Proyecto</th>
              <th style={{ padding: "12px 14px", fontWeight: 600 }}>Repositorio</th>
              <th style={{ padding: "12px 14px", fontWeight: 600 }}>Tipo</th>
              <th style={{ padding: "12px 14px", fontWeight: 600 }}>Estado (Etapa)</th>
              <th style={{ padding: "12px 14px", fontWeight: 600 }}>Subdominio</th>
              <th style={{ padding: "12px 14px", fontWeight: 600, textAlign: "right" }}>Acciones</th>
            </tr>
          </thead>
          <tbody>
            {proyectos.length === 0 && !loading ? (
              <tr>
                <td colSpan={6} style={{ padding: "30px", textAlign: "center", color: "#6b7280" }}>
                  No hay proyectos registrados en SAMMCORE Deployer.
                </td>
              </tr>
            ) : (
              proyectos.map((p, idx) => {
                const isRunning = p.status === "running";
                const isFailed = p.status === "failed";
                const isProgress = !isRunning && !isFailed;

                return (
                  <tr
                    key={p.id}
                    style={{
                      borderBottom: idx < proyectos.length - 1 ? "1px solid #242430" : "none",
                      background: isFailed ? "rgba(239, 68, 68, 0.03)" : "transparent",
                    }}
                  >
                    <td style={{ padding: "12px 14px" }}>
                      <strong>{p.name}</strong>
                      {p.commit && (
                        <div style={{ fontSize: "0.75rem", color: "#8a8a9e", marginTop: "2px" }}>
                          commit: <code>{p.commit.substring(0, 7)}</code>
                        </div>
                      )}
                    </td>
                    <td style={{ padding: "12px 14px" }}>
                      <a href={p.repo} target="_blank" rel="noreferrer" style={{ color: "#93c5fd", textDecoration: "none" }}>
                        {p.repo.replace("https://github.com/", "")}
                      </a>
                      <span style={{ fontSize: "0.75rem", color: "#8a8a9e", display: "block" }}>
                        branch: {p.branch || "main"}
                      </span>
                    </td>
                    <td style={{ padding: "12px 14px" }}>
                      <span style={{ padding: "2px 8px", background: "#22222e", borderRadius: "4px", fontSize: "0.8rem", color: "#c7d2fe" }}>
                        {p.type}
                      </span>
                    </td>
                    <td style={{ padding: "12px 14px" }}>
                      {isRunning && (
                        <span style={{ padding: "3px 8px", background: "#14532d", color: "#86efac", borderRadius: "10px", fontSize: "0.8rem", fontWeight: 600 }}>
                          🟢 Running (Listo)
                        </span>
                      )}
                      {isFailed && (
                        <button
                          onClick={() => showErrorDetails(p)}
                          style={{
                            padding: "3px 8px",
                            background: "#7f1d1d",
                            color: "#fca5a5",
                            border: "1px solid #dc2626",
                            borderRadius: "10px",
                            fontSize: "0.8rem",
                            fontWeight: 600,
                            cursor: "pointer",
                          }}
                          title="Haz clic para ver el motivo del fallo"
                        >
                          🔴 Fallido (Paso {p.current_step || "?"}/{p.total_steps || "?"}) 🔍
                        </button>
                      )}
                      {isProgress && (
                        <span style={{ padding: "3px 8px", background: "#1e3a8a", color: "#93c5fd", borderRadius: "10px", fontSize: "0.8rem", fontWeight: 600 }}>
                          ⏳ Paso {p.current_step || 1}/{p.total_steps || 4}: {p.status}
                        </span>
                      )}
                    </td>
                    <td style={{ padding: "12px 14px" }}>
                      {p.domain ? (
                        <a href={`https://${p.domain}`} target="_blank" rel="noreferrer" style={{ color: "#60a5fa", textDecoration: "none", fontWeight: 500 }}>
                          {p.domain}
                        </a>
                      ) : (
                        "—"
                      )}
                    </td>
                    <td style={{ padding: "12px 14px", textAlign: "right" }}>
                      <div style={{ display: "flex", gap: "6px", justifyContent: "flex-end" }}>
                        <button
                          onClick={() => handleMetrics(p.id, p.name)}
                          style={{
                            padding: "6px 10px",
                            background: "#22222e",
                            color: "#e2e8f0",
                            border: "1px solid #3d3d4d",
                            borderRadius: "4px",
                            cursor: "pointer",
                            fontSize: "0.8rem",
                          }}
                        >
                          📊 Métricas
                        </button>
                        <button
                          onClick={() => handleLogs(p.id, p.name)}
                          style={{
                            padding: "6px 10px",
                            background: "#22222e",
                            color: "#e2e8f0",
                            border: "1px solid #3d3d4d",
                            borderRadius: "4px",
                            cursor: "pointer",
                            fontSize: "0.8rem",
                          }}
                        >
                          📋 Logs
                        </button>
                        <button
                          onClick={() => handleRedeploy(p.id, p.name)}
                          style={{
                            padding: "6px 10px",
                            background: "#1e3a8a",
                            color: "#93c5fd",
                            border: "none",
                            borderRadius: "4px",
                            cursor: "pointer",
                            fontSize: "0.8rem",
                            fontWeight: 600,
                          }}
                        >
                          🔄 Redeploy
                        </button>
                        <button
                          onClick={() => handleDeletePrompt(p)}
                          style={{
                            padding: "6px 10px",
                            background: "#7f1d1d",
                            color: "#fca5a5",
                            border: "none",
                            borderRadius: "4px",
                            cursor: "pointer",
                            fontSize: "0.8rem",
                            fontWeight: 600,
                          }}
                        >
                          🗑️
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>

      {/* Métricas dinámicas */}
      {loadingMetrics && (
        <p style={{ marginTop: "20px", color: "#93c5fd" }}>⏳ Cargando métricas en tiempo real desde K3s...</p>
      )}

      {metricsData && (
        <div style={{ marginTop: "28px", padding: "20px", background: "#1c1c24", border: "1px solid #3b82f6", borderRadius: "8px", boxShadow: "0 4px 14px rgba(0,0,0,0.4)" }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "12px" }}>
            <h3 style={{ margin: 0, color: "#93c5fd", fontSize: "1.15rem" }}>
              📊 Métricas en Vivo de Pods: {metricsData.project_name}
            </h3>
            <button
              onClick={() => setMetricsData(null)}
              style={{ padding: "4px 10px", background: "#22222e", color: "#cbd5e1", border: "1px solid #3d3d4d", borderRadius: "4px", cursor: "pointer" }}
            >
              ✕ Cerrar
            </button>
          </div>

          <p style={{ margin: "0 0 14px 0", fontSize: "0.85rem", color: "#8a8a9e" }}>
            Namespace: <code>{metricsData.namespace}</code> | Total Pods: <strong>{metricsData.pods_count}</strong> | Estado: {metricsData.status}
          </p>

          {metricsData.pods.length === 0 ? (
            <p style={{ color: "#f87171", fontSize: "0.9rem" }}>No se encontraron pods activos en este namespace.</p>
          ) : (
            <table style={{ width: "100%", borderCollapse: "collapse", fontSize: "0.85rem", textAlign: "left", background: "#121216", borderRadius: "6px", overflow: "hidden" }}>
              <thead>
                <tr style={{ background: "#15151c", borderBottom: "1px solid #2d2d3a", color: "#8a8a9e" }}>
                  <th style={{ padding: "8px 12px" }}>Pod</th>
                  <th style={{ padding: "8px 12px" }}>Fase</th>
                  <th style={{ padding: "8px 12px" }}>Listo</th>
                  <th style={{ padding: "8px 12px" }}>Reinicios</th>
                  <th style={{ padding: "8px 12px" }}>CPU</th>
                  <th style={{ padding: "8px 12px" }}>Memoria</th>
                </tr>
              </thead>
              <tbody>
                {metricsData.pods.map((pod: any, i: number) => (
                  <tr key={pod.name} style={{ borderBottom: i < metricsData.pods.length - 1 ? "1px solid #22222a" : "none" }}>
                    <td style={{ padding: "8px 12px" }}><code>{pod.name}</code></td>
                    <td style={{ padding: "8px 12px" }}>{pod.status}</td>
                    <td style={{ padding: "8px 12px" }}>{pod.ready ? "✅ Sí" : "❌ No"}</td>
                    <td style={{ padding: "8px 12px" }}>{pod.restarts}</td>
                    <td style={{ padding: "8px 12px", color: "#38bdf8" }}><strong>{pod.cpu_usage || "0m"}</strong></td>
                    <td style={{ padding: "8px 12px", color: "#a78bfa" }}><strong>{pod.memory_usage || "0Mi"}</strong></td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      {/* Modal Reutilizable */}
      <Modal
        isOpen={modalState.isOpen}
        title={modalState.title}
        onClose={closeModal}
        onConfirm={modalState.onConfirm}
        confirmText={modalState.confirmText}
        variant={modalState.variant}
        width={modalState.width}
      >
        {modalState.content}
      </Modal>
    </div>
  );
}
