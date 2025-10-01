import { BrowserRouter, Routes, Route } from "react-router-dom";
import Navbar from "./components/Navbar";
import RegistrarProyecto from "./pages/RegistrarProyecto";
import EstadoProyectos from "./pages/EstadoProyectos";

function App() {
  return (
    <BrowserRouter>
      <Navbar />
      <Routes>
        <Route path="/" element={<RegistrarProyecto />} />
        <Route path="/estado" element={<EstadoProyectos />} />
      </Routes>
    </BrowserRouter>
  );
}

export default App;
