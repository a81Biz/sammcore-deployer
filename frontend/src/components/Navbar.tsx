import { useState, useEffect } from "react";
import { Link } from "react-router-dom";
import { getApiKey } from "../services/api";
import ApiKeyModal from "./ApiKeyModal";

export default function Navbar() {
  const [modalOpen, setModalOpen] = useState(false);
  const [authReason, setAuthReason] = useState<string | null>(null);
  const [hasKey, setHasKey] = useState(false);

  useEffect(() => {
    setHasKey(Boolean(getApiKey()));

    const handleAuthRequired = (e: Event) => {
      const customEvent = e as CustomEvent<{ message?: string }>;
      setAuthReason(customEvent.detail?.message || "Autenticación requerida.");
      setModalOpen(true);
    };

    window.addEventListener("deployer:auth-required", handleAuthRequired);
    return () => {
      window.removeEventListener("deployer:auth-required", handleAuthRequired);
    };
  }, []);

  const handleOpenModal = () => {
    setAuthReason(null);
    setModalOpen(true);
  };

  const handleCloseModal = () => {
    setModalOpen(false);
    setAuthReason(null);
    setHasKey(Boolean(getApiKey()));
  };

  return (
    <>
      <nav
        style={{
          padding: "12px 24px",
          background: "#16161a",
          color: "#fff",
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          borderBottom: "1px solid #282830",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: "20px" }}>
          <strong style={{ fontSize: "1.05rem", letterSpacing: "0.5px" }}>SAMMCORE Deployer</strong>
          <Link to="/" style={{ color: "#aaa", textDecoration: "none", fontSize: "0.9rem" }}>
            Registrar Proyecto
          </Link>
          <Link to="/estado" style={{ color: "#aaa", textDecoration: "none", fontSize: "0.9rem" }}>
            Estado de Proyectos
          </Link>
        </div>
        <div>
          <button
            onClick={handleOpenModal}
            style={{
              background: "#222228",
              color: "#eee",
              border: "1px solid #3d3d48",
              padding: "6px 12px",
              cursor: "pointer",
              borderRadius: "4px",
              fontSize: "0.85rem",
              display: "flex",
              alignItems: "center",
              gap: "6px",
            }}
          >
            <span>🔑 Clave API</span>
            <span
              style={{
                width: "8px",
                height: "8px",
                borderRadius: "50%",
                background: hasKey ? "#4caf50" : "#ff9800",
                display: "inline-block",
              }}
              title={hasKey ? "Clave API configurada" : "Sin clave API configurada"}
            />
          </button>
        </div>
      </nav>

      <ApiKeyModal isOpen={modalOpen} onClose={handleCloseModal} reason={authReason} />
    </>
  );
}
