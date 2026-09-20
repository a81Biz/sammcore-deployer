import { Project } from "../services/api";

interface DeploymentProgressProps {
  project: Project;
  onRetry?: () => void;
  onViewLogs?: () => void;
}

export default function DeploymentProgress({ project, onRetry, onViewLogs }: DeploymentProgressProps) {
  const currentStep = project.current_step || (project.status === "running" ? (project.total_steps || 4) : 1);
  const totalSteps = project.total_steps || (project.requires_database ? 4 : 3);
  const percent = project.status === "running" ? 100 : Math.min(Math.round((currentStep / totalSteps) * 100), 95);
  const isFailed = project.status === "failed";
  const isRunning = project.status === "running";

  const getStepTitle = (stepIndex: number): string => {
    if (project.requires_database) {
      switch (stepIndex) {
        case 1:
          return "Base de Datos Supabase";
        case 2:
          return "Compilación con Kaniko";
        case 3:
          return "Manifiestos Kubernetes";
        case 4:
          return "Rollout de Pods";
        default:
          return `Paso ${stepIndex}`;
      }
    } else {
      switch (stepIndex) {
        case 1:
          return "Compilación con Kaniko";
        case 2:
          return "Manifiestos Kubernetes";
        case 3:
          return "Rollout de Pods";
        default:
          return `Paso ${stepIndex}`;
      }
    }
  };

  const steps = Array.from({ length: totalSteps }, (_, i) => i + 1);

  return (
    <div
      style={{
        background: "#16161e",
        border: `1px solid ${isFailed ? "#ef4444" : isRunning ? "#22c55e" : "#3b82f6"}`,
        borderRadius: "8px",
        padding: "18px 20px",
        marginBottom: "18px",
        boxShadow: "0 4px 14px rgba(0,0,0,0.3)",
      }}
    >
      {/* Header */}
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "14px" }}>
        <div>
          <span style={{ fontSize: "0.85rem", color: "#8a8a9e", textTransform: "uppercase", fontWeight: 700, letterSpacing: "0.5px" }}>
            Progreso del Despliegue:
          </span>
          <h4 style={{ margin: "4px 0 0 0", color: "#ffffff", fontSize: "1.1rem" }}>
            {project.name}
            <span style={{ marginLeft: "10px", fontSize: "0.8rem", color: "#a5b4fc", fontWeight: 500 }}>
              ({project.domain})
            </span>
          </h4>
        </div>

        <div style={{ textAlign: "right" }}>
          <span
            style={{
              padding: "4px 12px",
              borderRadius: "12px",
              fontSize: "0.8rem",
              fontWeight: 700,
              background: isFailed ? "#7f1d1d" : isRunning ? "#14532d" : "#1e3a8a",
              color: isFailed ? "#fca5a5" : isRunning ? "#86efac" : "#93c5fd",
            }}
          >
            {isFailed
              ? "FALLIDO"
              : isRunning
              ? "RUNNING (100%)"
              : `PASO ${currentStep}/${totalSteps} (${percent}%)`}
          </span>
        </div>
      </div>

      {/* Stepper visual */}
      <div style={{ display: "flex", gap: "8px", marginBottom: "12px" }}>
        {steps.map((s) => {
          const isDone = s < currentStep || isRunning;
          const isCurrent = s === currentStep && !isRunning && !isFailed;
          const isStepFailed = isFailed && s === currentStep;

          let stepBg = "#22222f";
          let stepColor = "#66667a";
          if (isDone) {
            stepBg = "#15803d";
            stepColor = "#fff";
          } else if (isCurrent) {
            stepBg = "#2563eb";
            stepColor = "#fff";
          } else if (isStepFailed) {
            stepBg = "#dc2626";
            stepColor = "#fff";
          }

          return (
            <div
              key={s}
              style={{
                flex: 1,
                background: "#121218",
                borderRadius: "6px",
                padding: "8px 10px",
                border: `1px solid ${isCurrent ? "#3b82f6" : isStepFailed ? "#ef4444" : "#2d2d3a"}`,
                display: "flex",
                alignItems: "center",
                gap: "8px",
              }}
            >
              <div
                style={{
                  width: "22px",
                  height: "22px",
                  borderRadius: "50%",
                  background: stepBg,
                  color: stepColor,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  fontSize: "0.75rem",
                  fontWeight: 700,
                  flexShrink: 0,
                }}
              >
                {isDone ? "✓" : isStepFailed ? "✕" : s}
              </div>
              <div style={{ overflow: "hidden" }}>
                <div style={{ fontSize: "0.7rem", color: "#8a8a9e" }}>Paso {s}/{totalSteps}</div>
                <div
                  style={{
                    fontSize: "0.8rem",
                    color: isCurrent ? "#93c5fd" : isStepFailed ? "#fca5a5" : "#e0e0e8",
                    fontWeight: 600,
                    whiteSpace: "nowrap",
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                  }}
                >
                  {getStepTitle(s)}
                </div>
              </div>
            </div>
          );
        })}
      </div>

      {/* Barra de progreso */}
      <div style={{ width: "100%", background: "#22222f", borderRadius: "4px", height: "6px", overflow: "hidden", marginBottom: "12px" }}>
        <div
          style={{
            width: `${percent}%`,
            height: "100%",
            background: isFailed ? "#ef4444" : isRunning ? "#22c55e" : "linear-gradient(90deg, #3b82f6, #60a5fa)",
            transition: "width 0.4s ease",
          }}
        />
      </div>

      {/* Descripción en tiempo real */}
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <div style={{ fontSize: "0.88rem", color: isFailed ? "#f87171" : isRunning ? "#4ade80" : "#93c5fd" }}>
          {isFailed ? (
            <span>⚠️ <strong>Fallo en el despliegue:</strong> {project.last_error || "Error indeterminado"}</span>
          ) : (
            <span>⏳ {project.step_description || (isRunning ? "Todos los servicios operativos" : "Procesando tarea en K3s...")}</span>
          )}
        </div>

        <div style={{ display: "flex", gap: "8px" }}>
          {onViewLogs && (
            <button
              onClick={onViewLogs}
              style={{
                padding: "4px 10px",
                background: "#22222e",
                color: "#cbd5e1",
                border: "1px solid #3d3d4d",
                borderRadius: "4px",
                fontSize: "0.8rem",
                cursor: "pointer",
              }}
            >
              📋 Ver Logs
            </button>
          )}
          {isFailed && onRetry && (
            <button
              onClick={onRetry}
              style={{
                padding: "4px 12px",
                background: "#2563eb",
                color: "#fff",
                border: "none",
                borderRadius: "4px",
                fontSize: "0.8rem",
                fontWeight: 600,
                cursor: "pointer",
              }}
            >
              🔄 Reintentar
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
