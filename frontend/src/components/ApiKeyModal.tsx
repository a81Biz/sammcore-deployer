import React, { useState, useEffect } from "react";
import { getApiKey, setApiKey } from "../services/api";

interface ApiKeyModalProps {
  isOpen: boolean;
  onClose: () => void;
  reason?: string | null;
}

export default function ApiKeyModal({ isOpen, onClose, reason }: ApiKeyModalProps) {
  const [keyInput, setKeyInput] = useState("");
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    if (isOpen) {
      setKeyInput(getApiKey());
      setSaved(false);
    }
  }, [isOpen]);

  if (!isOpen) return null;

  const handleSave = (e: React.FormEvent) => {
    e.preventDefault();
    setApiKey(keyInput);
    setSaved(true);
    setTimeout(() => {
      onClose();
    }, 600);
  };

  const handleClear = () => {
    setApiKey("");
    setKeyInput("");
    setSaved(true);
    setTimeout(() => {
      onClose();
    }, 600);
  };

  return (
    <div
      style={{
        position: "fixed",
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        backgroundColor: "rgba(0, 0, 0, 0.65)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        zIndex: 9999,
      }}
    >
      <div
        style={{
          background: "#1e1e24",
          color: "#fff",
          padding: "24px",
          borderRadius: "8px",
          width: "100%",
          maxWidth: "440px",
          boxShadow: "0 8px 24px rgba(0, 0, 0, 0.5)",
          border: "1px solid #333",
        }}
      >
        <h3 style={{ margin: "0 0 12px 0", fontSize: "1.2rem", display: "flex", alignItems: "center", gap: "8px" }}>
          🔑 Clave de API (DEPLOYER_API_KEY)
        </h3>

        {reason && (
          <div
            style={{
              background: "#ff525222",
              border: "1px solid #ff5252",
              color: "#ff8080",
              padding: "8px 12px",
              borderRadius: "4px",
              fontSize: "0.85rem",
              marginBottom: "12px",
            }}
          >
            {reason}
          </div>
        )}

        <p style={{ fontSize: "0.85rem", color: "#aaa", margin: "0 0 16px 0" }}>
          Esta clave se almacena localmente en el navegador y se envía en la cabecera <code>Authorization: Bearer</code> de cada petición al backend.
        </p>

        <form onSubmit={handleSave}>
          <input
            type="password"
            placeholder="Ingrese el token DEPLOYER_API_KEY..."
            value={keyInput}
            onChange={(e) => setKeyInput(e.target.value)}
            style={{
              width: "100%",
              boxSizing: "border-box",
              padding: "10px 12px",
              borderRadius: "4px",
              border: "1px solid #444",
              background: "#121216",
              color: "#fff",
              fontSize: "0.95rem",
              marginBottom: "16px",
            }}
            autoFocus
          />

          {saved && (
            <div style={{ color: "#4caf50", fontSize: "0.85rem", marginBottom: "12px" }}>
              ✓ Guardado correctamente
            </div>
          )}

          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
            <button
              type="button"
              onClick={handleClear}
              style={{
                background: "transparent",
                border: "1px solid #555",
                color: "#ccc",
                padding: "6px 12px",
                borderRadius: "4px",
                cursor: "pointer",
                fontSize: "0.85rem",
              }}
            >
              Borrar Clave
            </button>

            <div style={{ display: "flex", gap: "8px" }}>
              <button
                type="button"
                onClick={onClose}
                style={{
                  background: "#333",
                  border: "none",
                  color: "#ddd",
                  padding: "8px 14px",
                  borderRadius: "4px",
                  cursor: "pointer",
                  fontSize: "0.85rem",
                }}
              >
                Cancelar
              </button>
              <button
                type="submit"
                style={{
                  background: "#1976d2",
                  border: "none",
                  color: "#fff",
                  padding: "8px 16px",
                  borderRadius: "4px",
                  cursor: "pointer",
                  fontWeight: "bold",
                  fontSize: "0.85rem",
                }}
              >
                Guardar
              </button>
            </div>
          </div>
        </form>
      </div>
    </div>
  );
}
