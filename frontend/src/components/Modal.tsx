import React from "react";

export interface ModalProps {
  isOpen: boolean;
  title: string;
  onClose: () => void;
  onConfirm?: () => void;
  confirmText?: string;
  cancelText?: string;
  variant?: "danger" | "primary" | "info";
  children: React.ReactNode;
  width?: string;
}

export default function Modal({
  isOpen,
  title,
  onClose,
  onConfirm,
  confirmText = "Aceptar",
  cancelText = "Cancelar",
  variant = "primary",
  children,
  width = "520px",
}: ModalProps) {
  if (!isOpen) return null;

  const getConfirmBg = () => {
    switch (variant) {
      case "danger":
        return "#ef4444";
      case "info":
        return "#6366f1";
      default:
        return "#3b82f6";
    }
  };

  return (
    <div
      style={{
        position: "fixed",
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        backgroundColor: "rgba(0, 0, 0, 0.75)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        zIndex: 10000,
        backdropFilter: "blur(4px)",
      }}
      onClick={onClose}
    >
      <div
        style={{
          background: "#1c1c24",
          color: "#e0e0e8",
          padding: "24px",
          borderRadius: "10px",
          width: "90%",
          maxWidth: width,
          boxShadow: "0 10px 30px rgba(0, 0, 0, 0.6)",
          border: "1px solid #2d2d3a",
          fontFamily: "system-ui, sans-serif",
          maxHeight: "85vh",
          display: "flex",
          flexDirection: "column",
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            marginBottom: "16px",
            borderBottom: "1px solid #2d2d3a",
            paddingBottom: "12px",
          }}
        >
          <h3 style={{ margin: 0, fontSize: "1.2rem", color: "#ffffff", display: "flex", alignItems: "center", gap: "8px" }}>
            {title}
          </h3>
          <button
            onClick={onClose}
            style={{
              background: "transparent",
              border: "none",
              color: "#8a8a9e",
              fontSize: "1.3rem",
              cursor: "pointer",
              lineHeight: 1,
            }}
          >
            ✕
          </button>
        </div>

        {/* Body */}
        <div style={{ overflowY: "auto", flex: 1, marginBottom: "20px", fontSize: "0.95rem", lineHeight: 1.5 }}>
          {children}
        </div>

        {/* Footer */}
        <div style={{ display: "flex", justifyContent: "flex-end", gap: "10px", borderTop: "1px solid #2d2d3a", paddingTop: "14px" }}>
          {onConfirm ? (
            <>
              <button
                type="button"
                onClick={onClose}
                style={{
                  padding: "8px 16px",
                  background: "#2a2a38",
                  color: "#cbd5e1",
                  border: "1px solid #3d3d4d",
                  borderRadius: "6px",
                  cursor: "pointer",
                  fontWeight: 600,
                  fontSize: "0.9rem",
                }}
              >
                {cancelText}
              </button>
              <button
                type="button"
                onClick={onConfirm}
                style={{
                  padding: "8px 18px",
                  background: getConfirmBg(),
                  color: "#fff",
                  border: "none",
                  borderRadius: "6px",
                  cursor: "pointer",
                  fontWeight: 600,
                  fontSize: "0.9rem",
                  boxShadow: "0 2px 8px rgba(0,0,0,0.3)",
                }}
              >
                {confirmText}
              </button>
            </>
          ) : (
            <button
              type="button"
              onClick={onClose}
              style={{
                padding: "8px 20px",
                background: "#3b82f6",
                color: "#fff",
                border: "none",
                borderRadius: "6px",
                cursor: "pointer",
                fontWeight: 600,
                fontSize: "0.9rem",
              }}
            >
              Cerrar
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
