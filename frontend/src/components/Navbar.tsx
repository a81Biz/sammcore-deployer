import { Link } from "react-router-dom";
import { getApiKey, setApiKey } from "../services/api";

export default function Navbar() {
  const handleConfigKey = () => {
    const current = getApiKey();
    const newKey = prompt("Ingrese la DEPLOYER_API_KEY para autenticar peticiones:", current);
    if (newKey !== null) {
      setApiKey(newKey);
      alert("Clave API guardada en el navegador.");
    }
  };

  return (
    <nav style={{ padding: "10px 20px", background: "#222", color: "#fff", display: "flex", justifyContent: "space-between", alignItems: "center" }}>
      <div>
        <strong style={{ marginRight: "20px" }}>SAMMCORE Deployer</strong>
        <Link to="/" style={{ marginRight: "15px", color: "#fff", textDecoration: "none" }}>Registrar Proyecto</Link>
        <Link to="/estado" style={{ color: "#fff", textDecoration: "none" }}>Estado de Proyectos</Link>
      </div>
      <div>
        <button onClick={handleConfigKey} style={{ background: "#444", color: "#fff", border: "1px solid #666", padding: "5px 10px", cursor: "pointer", borderRadius: "4px" }}>
          🔑 Clave API
        </button>
      </div>
    </nav>
  );
}
