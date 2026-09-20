import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { analyzeRepo, deployProject, DeployPayload, ServiceSpec } from "../services/api";

export default function RegistrarProyecto() {
  const navigate = useNavigate();

  // Paso 1: Análisis
  const [repo, setRepo] = useState("");
  const [branch, setBranch] = useState("main");
  const [loadingAnalyze, setLoadingAnalyze] = useState(false);
  const [errorAnalyze, setErrorAnalyze] = useState<string | null>(null);
  const [analysisData, setAnalysisData] = useState<any | null>(null);

  // Paso 2: Configuración del Despliegue
  const [projectName, setProjectName] = useState("");
  const [projectType, setProjectType] = useState("compose");
  const [requiresDatabase, setRequiresDatabase] = useState(false);
  const [services, setServices] = useState<ServiceSpec[]>([]);
  const [envVars, setEnvVars] = useState<{ key: string; value: string }[]>([]);
  const [newEnvKey, setNewEnvKey] = useState("");
  const [newEnvVal, setNewEnvVal] = useState("");

  // Paso 3: Despliegue
  const [loadingDeploy, setLoadingDeploy] = useState(false);
  const [deployError, setDeployError] = useState<string | null>(null);
  const [deploySuccess, setDeploySuccess] = useState<any | null>(null);

  const handleAnalyze = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!repo.trim()) {
      setErrorAnalyze("Por favor ingrese la URL del repositorio.");
      return;
    }
    setLoadingAnalyze(true);
    setErrorAnalyze(null);
    setDeploySuccess(null);
    setDeployError(null);

    try {
      const cleanRepo = repo.trim().replace(/\/+$/, "");
      const cleanBranch = branch.trim();
      const data = await analyzeRepo(cleanRepo, cleanBranch || "main");
      setAnalysisData(data);
      const name = data.name || "app";
      setProjectName(name);
      setProjectType(data.type || "compose");
      setRequiresDatabase(Boolean(data.requires_database));

      // Poblar servicios desde el análisis
      if (data.services && data.services.length > 0) {
        setServices(data.services.map((s: ServiceSpec) => ({ ...s })));
      } else {
        // Fallback: un servicio genérico
        setServices([{ name: "app", role: "app", port: 8080, build_context: ".", dockerfile: "Dockerfile" }]);
      }
    } catch (err: any) {
      setErrorAnalyze(err.message || "Error al analizar el repositorio.");
      setAnalysisData(null);
    } finally {
      setLoadingAnalyze(false);
    }
  };

  const handleServiceChange = (index: number, field: keyof ServiceSpec, value: string | number) => {
    const updated = [...services];
    (updated[index] as any)[field] = value;
    setServices(updated);
  };

  const handleAddEnv = () => {
    if (newEnvKey.trim()) {
      setEnvVars([...envVars, { key: newEnvKey.trim(), value: newEnvVal.trim() }]);
      setNewEnvKey("");
      setNewEnvVal("");
    }
  };

  const handleRemoveEnv = (index: number) => {
    setEnvVars(envVars.filter((_, i) => i !== index));
  };

  const handleDeploy = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!projectName.trim()) {
      setDeployError("El nombre del proyecto es obligatorio.");
      return;
    }

    setLoadingDeploy(true);
    setDeployError(null);

    const buildArgsObj: Record<string, string> = {};
    for (const item of envVars) {
      if (item.key) buildArgsObj[item.key] = item.value;
    }

    const payload: DeployPayload = {
      name: projectName.trim().toLowerCase(),
      repo: repo.trim(),
      branch: branch.trim() || "main",
      type: projectType,
      requires_database: requiresDatabase,
      services: services,
      build_args: Object.keys(buildArgsObj).length > 0 ? buildArgsObj : undefined,
    };

    try {
      const res = await deployProject(payload);
      setDeploySuccess(res);
    } catch (err: any) {
      setDeployError(err.message || "Error al iniciar el despliegue.");
    } finally {
      setLoadingDeploy(false);
    }
  };

  // Derivar dominios de los servicios
  const webService = services.find(s => s.role === "web");
  const apiService = services.find(s => s.role === "api" || s.role === "app");

  const cardStyle: React.CSSProperties = {
    background: "#1c1c24",
    borderRadius: "8px",
    padding: "24px",
    marginBottom: "24px",
    border: "1px solid #2d2d3a",
    boxShadow: "0 4px 12px rgba(0,0,0,0.3)",
  };

  const inputStyle: React.CSSProperties = {
    padding: "10px 14px",
    background: "#121216",
    border: "1px solid #3d3d4d",
    color: "#fff",
    borderRadius: "6px",
    fontSize: "0.95rem",
    width: "100%",
    boxSizing: "border-box",
  };

  const labelStyle: React.CSSProperties = {
    display: "block",
    marginBottom: "6px",
    fontSize: "0.85rem",
    fontWeight: 600,
    color: "#b0b0c0",
  };

  const roleColors: Record<string, string> = {
    web: "#22c55e",
    api: "#3b82f6",
    worker: "#a855f7",
    app: "#f59e0b",
  };

  return (
    <div style={{ maxWidth: "900px", margin: "30px auto", padding: "0 20px", color: "#e0e0e8", fontFamily: "system-ui, sans-serif" }}>
      <div style={{ marginBottom: "28px" }}>
        <h1 style={{ fontSize: "1.8rem", margin: "0 0 8px 0", color: "#ffffff" }}>
          🚀 Registrar y Desplegar Proyecto
        </h1>
        <p style={{ margin: 0, color: "#8a8a9e", fontSize: "0.95rem" }}>
          Inspecciona automáticamente repositorios de GitHub, detecta servicios y puertos, construye imágenes vía Kaniko y publica en el clúster K3s de SAMMCORE.
        </p>
      </div>

      {/* PASO 1: Formulario de Análisis */}
      <div style={cardStyle}>
        <h3 style={{ margin: "0 0 16px 0", fontSize: "1.15rem", color: "#a5b4fc", display: "flex", alignItems: "center", gap: "8px" }}>
          <span>1.</span> Inspección del Repositorio
        </h3>
        <form onSubmit={handleAnalyze}>
          <div style={{ display: "grid", gridTemplateColumns: "3fr 1fr auto", gap: "12px", alignItems: "flex-end" }}>
            <div>
              <label style={labelStyle}>URL del Repositorio (GitHub HTTPS)</label>
              <input
                type="text"
                placeholder="https://github.com/a81Biz/backroom"
                value={repo}
                onChange={(e) => setRepo(e.target.value)}
                style={inputStyle}
                disabled={loadingAnalyze || loadingDeploy}
              />
            </div>
            <div>
              <label style={labelStyle}>Rama (Branch)</label>
              <input
                type="text"
                placeholder="main o master"
                value={branch}
                onChange={(e) => setBranch(e.target.value)}
                style={inputStyle}
                disabled={loadingAnalyze || loadingDeploy}
              />
            </div>
            <div>
              <button
                type="submit"
                disabled={loadingAnalyze || loadingDeploy}
                style={{
                  padding: "10px 20px",
                  background: loadingAnalyze ? "#4b5563" : "#3b82f6",
                  color: "#fff",
                  border: "none",
                  borderRadius: "6px",
                  fontWeight: 600,
                  cursor: loadingAnalyze ? "not-allowed" : "pointer",
                  height: "42px",
                  whiteSpace: "nowrap",
                }}
              >
                {loadingAnalyze ? "Analizando..." : "🔍 Analizar"}
              </button>
            </div>
          </div>
        </form>

        {errorAnalyze && (
          <div style={{ marginTop: "14px", padding: "12px", background: "rgba(239, 68, 68, 0.15)", border: "1px solid #ef4444", borderRadius: "6px", color: "#fca5a5", fontSize: "0.9rem" }}>
            ⚠️ <strong>Error en análisis:</strong> {errorAnalyze}
            {(errorAnalyze.includes("Authorization") || errorAnalyze.includes("401") || errorAnalyze.includes("clave de API") || errorAnalyze.includes("inválida")) && (
              <div style={{ marginTop: "8px", fontSize: "0.85rem", color: "#fed7aa" }}>
                💡 <em>Tip: Haz clic en el botón <strong>"🔑 Clave API"</strong> en la esquina superior derecha e ingresa tu token para autorizar la sesión.</em>
              </div>
            )}
          </div>
        )}
      </div>

      {/* PASO 2: Configuración del Despliegue (Aparece tras analizar) */}
      {analysisData && !deploySuccess && (
        <div style={cardStyle}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "18px" }}>
            <h3 style={{ margin: 0, fontSize: "1.15rem", color: "#a5b4fc", display: "flex", alignItems: "center", gap: "8px" }}>
              <span>2.</span> Configuración de Despliegue en K3s
            </h3>
            <span style={{
              background: "#312e81",
              color: "#c7d2fe",
              padding: "4px 10px",
              borderRadius: "12px",
              fontSize: "0.8rem",
              fontWeight: 600,
              textTransform: "uppercase"
            }}>
              Arquetipo: {analysisData.type}
            </span>
          </div>

          <form onSubmit={handleDeploy}>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "16px", marginBottom: "16px" }}>
              <div>
                <label style={labelStyle}>Nombre del Proyecto (RFC 1123)</label>
                <input
                  type="text"
                  value={projectName}
                  onChange={(e) => setProjectName(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ""))}
                  style={inputStyle}
                  required
                />
                <span style={{ fontSize: "0.75rem", color: "#8a8a9e" }}>
                  Solo minúsculas alfanuméricas y guiones. 3-35 caracteres.
                </span>
              </div>
              <div>
                <label style={labelStyle}>Tipo de Arquitectura</label>
                <select
                  value={projectType}
                  onChange={(e) => setProjectType(e.target.value)}
                  style={{ ...inputStyle, cursor: "pointer" }}
                >
                  <option value="compose">Docker Compose (Microservicios)</option>
                  <option value="dockerfile">Dockerfile (Contenedor Único)</option>
                  <option value="static">Sitio Estático (Frontend NGINX)</option>
                </select>
              </div>
            </div>

            {/* Previsualización de Dominios Ingress */}
            <div style={{ padding: "12px 16px", background: "rgba(59, 130, 246, 0.08)", border: "1px solid #1e3a8a", borderRadius: "6px", marginBottom: "18px" }}>
              <div style={{ fontSize: "0.85rem", color: "#93c5fd", fontWeight: 600, marginBottom: "4px" }}>
                🌐 Subdominios Dinámicos que se publicarán:
              </div>
              {webService && (
                <div style={{ fontSize: "0.9rem", color: "#bfdbfe" }}>
                  • Frontend: <code>https://{projectName || "<proyecto>"}.sammcore.local</code> → puerto {webService.port}
                </div>
              )}
              {apiService && projectType === "compose" && (
                <div style={{ fontSize: "0.9rem", color: "#bfdbfe" }}>
                  • API Backend: <code>https://{projectName || "<proyecto>"}-api.sammcore.local</code> → puerto {apiService.port}
                </div>
              )}
              {!webService && apiService && (
                <div style={{ fontSize: "0.9rem", color: "#bfdbfe" }}>
                  • App: <code>https://{projectName || "<proyecto>"}.sammcore.local</code> → puerto {apiService.port}
                </div>
              )}
            </div>

            {/* Checkbox Base de Datos */}
            <div style={{ marginBottom: "20px", display: "flex", alignItems: "center", gap: "10px", padding: "10px 14px", background: "#121216", borderRadius: "6px", border: "1px solid #2d2d3a" }}>
              <input
                type="checkbox"
                id="reqDB"
                checked={requiresDatabase}
                onChange={(e) => setRequiresDatabase(e.target.checked)}
                style={{ width: "18px", height: "18px", cursor: "pointer" }}
              />
              <label htmlFor="reqDB" style={{ cursor: "pointer", fontSize: "0.9rem", color: "#e2e8f0" }}>
                <strong>Aprovisionar Base de Datos Dedicada (Modelo A):</strong> Crea <code>{projectName}_db</code> y rol <code>{projectName}_user</code> en el motor central PostgreSQL con credenciales inyectadas vía Kubernetes Secret.
              </label>
            </div>

            {/* Tabla de Servicios Detectados */}
            <div style={{ marginBottom: "20px" }}>
              <label style={{ ...labelStyle, marginBottom: "10px" }}>📦 Servicios Detectados ({services.length})</label>
              <div style={{ background: "#121216", borderRadius: "6px", border: "1px solid #2d2d3a", overflow: "hidden" }}>
                <table style={{ width: "100%", borderCollapse: "collapse", fontSize: "0.88rem" }}>
                  <thead>
                    <tr style={{ background: "#1a1a25", borderBottom: "1px solid #2d2d3a" }}>
                      <th style={{ padding: "10px 12px", textAlign: "left", color: "#8a8a9e", fontWeight: 600 }}>Servicio</th>
                      <th style={{ padding: "10px 12px", textAlign: "left", color: "#8a8a9e", fontWeight: 600 }}>Rol</th>
                      <th style={{ padding: "10px 12px", textAlign: "left", color: "#8a8a9e", fontWeight: 600 }}>Puerto</th>
                      <th style={{ padding: "10px 12px", textAlign: "left", color: "#8a8a9e", fontWeight: 600 }}>Contexto Build</th>
                    </tr>
                  </thead>
                  <tbody>
                    {services.map((svc, idx) => (
                      <tr key={idx} style={{ borderBottom: idx < services.length - 1 ? "1px solid #22222a" : "none" }}>
                        <td style={{ padding: "8px 12px" }}>
                          <code style={{ color: "#e0e0e8" }}>{svc.name}</code>
                        </td>
                        <td style={{ padding: "8px 12px" }}>
                          <select
                            value={svc.role}
                            onChange={(e) => handleServiceChange(idx, "role", e.target.value)}
                            style={{
                              padding: "4px 8px",
                              background: "#1c1c24",
                              border: `1px solid ${roleColors[svc.role] || "#3d3d4d"}`,
                              color: roleColors[svc.role] || "#e0e0e8",
                              borderRadius: "4px",
                              fontSize: "0.85rem",
                              cursor: "pointer",
                            }}
                          >
                            <option value="web">🌐 Web</option>
                            <option value="api">⚙️ API</option>
                            <option value="worker">🔧 Worker</option>
                            <option value="app">📦 App</option>
                          </select>
                        </td>
                        <td style={{ padding: "8px 12px" }}>
                          <input
                            type="number"
                            value={svc.port || ""}
                            onChange={(e) => handleServiceChange(idx, "port", Number(e.target.value) || 0)}
                            placeholder={svc.role === "worker" ? "—" : "80"}
                            style={{
                              ...inputStyle,
                              width: "80px",
                              padding: "4px 8px",
                              fontSize: "0.85rem",
                            }}
                          />
                        </td>
                        <td style={{ padding: "8px 12px" }}>
                          <code style={{ color: "#8a8a9e", fontSize: "0.82rem" }}>
                            {svc.build_context || "."}
                            {svc.dockerfile ? `/${svc.dockerfile}` : ""}
                          </code>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                {services.length === 0 && (
                  <div style={{ padding: "16px", textAlign: "center", color: "#6b7280" }}>
                    No se detectaron servicios. Ejecute el análisis primero.
                  </div>
                )}
              </div>
              <span style={{ fontSize: "0.75rem", color: "#6b7280", marginTop: "4px", display: "block" }}>
                Las imágenes se construyen automáticamente vía Kaniko en el clúster. Los puertos se leen del Dockerfile de cada servicio.
              </span>
            </div>

            {/* Commit detectado */}
            {analysisData?.commit && (
              <div style={{ marginBottom: "16px", fontSize: "0.85rem", color: "#6b7280" }}>
                📎 Commit detectado: <code style={{ color: "#a5b4fc" }}>{analysisData.commit.substring(0, 12)}</code>
              </div>
            )}

            {/* Variables de Entorno / Build Args */}
            <div style={{ marginBottom: "24px" }}>
              <label style={labelStyle}>Variables de Entorno y Configuración (Opcional)</label>
              <div style={{ display: "flex", gap: "10px", marginBottom: "10px" }}>
                <input
                  type="text"
                  placeholder="CLAVE (ej: VITE_API_BASE)"
                  value={newEnvKey}
                  onChange={(e) => setNewEnvKey(e.target.value)}
                  style={{ ...inputStyle, width: "40%" }}
                />
                <input
                  type="text"
                  placeholder="VALOR (ej: https://backroom-api.sammcore.local)"
                  value={newEnvVal}
                  onChange={(e) => setNewEnvVal(e.target.value)}
                  style={{ ...inputStyle, width: "50%" }}
                />
                <button
                  type="button"
                  onClick={handleAddEnv}
                  style={{
                    padding: "0 16px",
                    background: "#22c55e",
                    color: "#fff",
                    border: "none",
                    borderRadius: "6px",
                    cursor: "pointer",
                    fontWeight: 600,
                  }}
                >
                  + Agregar
                </button>
              </div>

              {envVars.length > 0 && (
                <div style={{ background: "#121216", borderRadius: "6px", padding: "8px 12px", border: "1px solid #2d2d3a" }}>
                  {envVars.map((env, idx) => (
                    <div key={idx} style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "4px 0", borderBottom: idx < envVars.length - 1 ? "1px solid #22222a" : "none", fontSize: "0.85rem" }}>
                      <span><code>{env.key}</code> = <code>{env.value}</code></span>
                      <button
                        type="button"
                        onClick={() => handleRemoveEnv(idx)}
                        style={{ background: "transparent", border: "none", color: "#f87171", cursor: "pointer", fontSize: "1rem" }}
                      >
                        ×
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </div>

            {deployError && (
              <div style={{ marginBottom: "16px", padding: "12px", background: "rgba(239, 68, 68, 0.15)", border: "1px solid #ef4444", borderRadius: "6px", color: "#fca5a5", fontSize: "0.9rem" }}>
                ⚠️ <strong>Error en el despliegue:</strong> {deployError}
              </div>
            )}

            <button
              type="submit"
              disabled={loadingDeploy}
              style={{
                width: "100%",
                padding: "14px",
                background: loadingDeploy ? "#4b5563" : "linear-gradient(135deg, #2563eb, #1d4ed8)",
                color: "#fff",
                border: "none",
                borderRadius: "6px",
                fontSize: "1.05rem",
                fontWeight: 700,
                cursor: loadingDeploy ? "not-allowed" : "pointer",
                boxShadow: "0 4px 14px rgba(37,99,235,0.4)",
              }}
            >
              {loadingDeploy ? "⏳ Construyendo imágenes y desplegando en SAMMCORE..." : "🚀 Confirmar y Desplegar en SAMMCORE"}
            </button>
          </form>
        </div>
      )}

      {/* PASO 3: Confirmación de Despliegue Exitoso */}
      {deploySuccess && (
        <div style={{ ...cardStyle, border: "1px solid #22c55e", background: "rgba(34, 197, 94, 0.06)" }}>
          <h3 style={{ margin: "0 0 12px 0", color: "#4ade80", fontSize: "1.3rem", display: "flex", alignItems: "center", gap: "8px" }}>
            ✅ ¡Despliegue Iniciado Correctamente!
          </h3>
          <p style={{ margin: "0 0 16px 0", color: "#bbf7d0", fontSize: "0.95rem", lineHeight: 1.5 }}>
            El proyecto <strong>{projectName}</strong> ha sido enviado a la cola de orquestación de Kubernetes.
            El deployer construirá las imágenes vía Kaniko, creará el Namespace, aprovisionará la base de datos y expondrá los subdominios Ingress.
          </p>
          <div style={{ display: "flex", gap: "12px" }}>
            <button
              onClick={() => navigate("/estado")}
              style={{
                padding: "10px 20px",
                background: "#22c55e",
                color: "#fff",
                border: "none",
                borderRadius: "6px",
                fontWeight: 600,
                cursor: "pointer",
              }}
            >
              📊 Ver Estado de Proyectos y Métricas
            </button>
            <button
              onClick={() => {
                setDeploySuccess(null);
                setAnalysisData(null);
                setRepo("");
                setServices([]);
              }}
              style={{
                padding: "10px 20px",
                background: "#272732",
                color: "#ddd",
                border: "1px solid #3d3d4a",
                borderRadius: "6px",
                fontWeight: 600,
                cursor: "pointer",
              }}
            >
              Registrar Otro Proyecto
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
