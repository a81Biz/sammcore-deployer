import { Link } from "react-router-dom";

export default function Navbar() {
  return (
    <nav style={{ padding: "10px", background: "#222", color: "#fff" }}>
      <Link to="/" style={{ marginRight: "10px", color: "#fff" }}>Registrar Proyecto</Link>
      <Link to="/estado" style={{ color: "#fff" }}>Estado de Proyectos</Link>
    </nav>
  );
}
