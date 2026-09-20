import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { analyzeRepo, deployProject, DeployPayload } from "../services/api";

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
  const [webPort, setWebPort] = useState<number>(80);
  const [apiPort, setApiPort] = useState<number>(8000);
  const [appPort, setAppPort] = useState<number>(8080);
  const [webImage, setWebImage] = useState("");
  const [apiImage, setApiImage] = useState("");
  const [appImage, setAppImage] = useState("");
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
      const data = await analyzeRepo(repo.trim(), branch.trim() || "main");
      setAnalysisData(data);
      const name = data.name || "app";
      setProjectName(name);
      setProjectType(data.type || "compose");
      setRequiresDatabase(Boolean(data.requires_database));

      if (data.type === "compose") {
        setWebPort(80);
        setApiPort(8000);
        setWebImage(`ghcr.io/a81biz/${name}-web:latest`);
        setApiImage(`ghcr.io/a81biz/${name}-api:latest`);
      } else {
        setAppPort(8080);
        setAppImage(`ghcr.io/a81biz/${name}:latest`);
      }
    } catch (err: any) {
      setErrorAnalyze(err.message || "Error al analizar el repositorio.");
      setAnalysisData(null);
    } finally {
      setLoadingAnalyze(false);
    }
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
      build_args: Object.keys(buildArgsObj).length > 0 ? buildArgsObj : undefined,
    };

    if (projectType === "compose") {
      payload.web_image = webImage || `ghcr.io/a81biz/${projectName}-web:latest`;
      payload.api_image = apiImage || `ghcr.io/a81biz/${projectName}-api:latest`;
      payload.web_port = Number(webPort) || 80;
      payload.api_port = Number(apiPort) || 8000;
    } else if (projectType === "dockerfile") {
      payload.app_image = appImage || `ghcr.io/a81biz/${projectName}:latest`;
      payload.app_port = Number(appPort) || 8080;
    } else {
      payload.static_image = "nginx:alpine";
    }

    try {
      const res = await deployProject(payload);
      setDeploySuccess(res);
    } catch (err: any) {
      setDeployError(err.message || "Error al iniciar el despliegue.");
    } finally {
      setLoadingDeploy(false);
    }
  };

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

  return (
    <div style={{ maxWidth: "900px", margin: "30px auto", padding: "0 20px", color: "#e0e0e8", fontFamily: "system-ui, sans-serif" }}>
      <div style={{ marginBottom: "28px" }}>
        <h1 style={{ fontSize: "1.8rem", margin: "0 0 8px 0", color: "#ffffff" }}>
          🚀 Registrar y Desplegar Proyecto
        </h1>
        <p style={{ margin: 0, color: "#8a8a9e", fontSize: "0.95rem" }}>
          Inspecciona automáticamente repositorios de GitHub, configura subdominios dinámicos y publica aplicaciones en el clúster K3s de SAMMCORE.
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
                  Solo minúsculas alfanuméricas y guiones.
                </span>
              </div>
              <div>
                <label style={labelStyle}>Tipo de Arquitectura</label>
                <select
                  value={projectType}
                  onChange={(e) => setProjectType(e.target.value)}
                  style={{ ...inputStyle, cursor: "pointer" }}
                >
                  <option value="compose">Docker Compose (Microservicios Web + API)</option>
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
              <div style={{ fontSize: "0.9rem", color: "#bfdbfe" }}>
                • Frontend: <code>https://{projectName || "<proyecto>"}.sammcore.local</code>
              </div>
              {projectType === "compose" && (
                <div style={{ fontSize: "0.9rem", color: "#bfdbfe" }}>
                  • API Backend: <code>https://{projectName || "<proyecto>"}-api.sammcore.local</code>
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

            {/* Configuración de Imágenes según el Tipo */}
            {projectType === "compose" ? (
              <div style={{ display: "grid", gridTemplateColumns: "2fr 1fr", gap: "16px", marginBottom: "16px" }}>
                <div>
                  <label style={labelStyle}>Imagen Docker Web (Frontend)</label>
                  <input
                    type="text"
                    value={webImage}
                    onChange={(e) => setWebImage(e.target.value)}
                    style={inputStyle}
                  />
                </div>
                <div>
                  <label style={labelStyle}>Puerto Web</label>
                  <input
                    type="number"
                    value={webPort}
                    onChange={(e) => setWebPort(Number(e.target.value))}
                    style={inputStyle}
                  />
                </div>
                <div>
                  <label style={labelStyle}>Imagen Docker API (Backend)</label>
                  <input
                    type="text"
                    value={apiImage}
                    onChange={(e) => setApiImage(e.target.value)}
                    style={inputStyle}
                  />
                </div>
                <div>
                  <label style={labelStyle}>Puerto API</label>
                  <input
                    type="number"
                    value={apiPort}
                    onChange={(e) => setApiPort(Number(e.target.value))}
                    style={inputStyle}
                  />
                </div>
              </div>
            ) : projectType === "dockerfile" ? (
              <div style={{ display: "grid", gridTemplateColumns: "2fr 1fr", gap: "16px", marginBottom: "16px" }}>
                <div>
                  <label style={labelStyle}>Imagen Docker App</label>
                  <input
                    type="text"
                    value={appImage}
                    onChange={(e) => setAppImage(e.target.value)}
                    style={inputStyle}
                  />
                </div>
                <div>
                  <label style={labelStyle}>Puerto App</label>
                  <input
                    type="number"
                    value={appPort}
                    onChange={(e) => setAppPort(Number(e.target.value))}
                    style={inputStyle}
                  />
                </div>
              </div>
            ) : null}

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
              {loadingDeploy ? "⏳ Desplegando en el Clúster SAMMCORE..." : "🚀 Confirmar y Desplegar en SAMMCORE"}
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
            El deployer creará el Namespace, aprovisionará la base de datos en PostgreSQL, aplicará los Secrets y expondrá los subdominios Ingress.
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
