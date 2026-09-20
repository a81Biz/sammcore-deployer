# Fase 3 — Frontend básico y “Estado de proyectos” (con persistencia simple)

## Objetivo

Crear una UI mínima que consuma el API de la Fase 2 y muestre el **estado de los proyectos** analizados.
Requisitos cumplidos:

* Formulario **Registrar Proyecto** (repo + branch) que llama al backend (`POST /analyzeRepo`) y muestra la respuesta.
* Pantalla **Estado de Proyectos** con **tabla dinámica** (repo, branch, tipo, estado) y **botones**: `Logs`, `Redeploy`, `Eliminar`.
* Persistencia ligera del historial en backend (archivo `history.json`).
* Hooks listos para Fase 4 (integración real con K3s) sin sobre-desarrollar esta fase.

---

## Cambios de backend

### 1) Persistencia mínima (`backend/storage/store.go`)

* **Estructura `Project`**: `ID`, `Repo`, `Branch`, `Type`, `Status`, `Timestamp`.
* **Archivo**: `history.json` en el directorio de ejecución del backend.
* **Operaciones**: `LoadProjects()`, `SaveProjects()`, `AddProject()`, `DeleteProject()` con `sync.Mutex` para evitar condiciones de carrera.

### 2) Hook desde el core

* Tras un análisis **exitoso** en `core/Analyze`, se persiste una entrada en `history.json`:

  * `Status = "analizado"`
  * `Type ∈ {compose, dockerfile, unknown}`

### 3) Endpoints nuevos (dummy, listos para Fase 4)

* `GET /history` → devuelve el arreglo de proyectos persistidos.
* `DELETE /history/{id}` → elimina una entrada del historial.
* `GET /logs/{id}` → devuelve un texto **mock** (“aquí aparecerían los logs”).
* `POST /redeploy/{id}` → devuelve texto **mock** (“redeploy iniciado”).
* **CORS habilitado** para permitir el frontend en `5173`:

  * Middleware que agrega `Access-Control-Allow-Origin: *`, maneja `OPTIONS`, y permite los headers básicos.

### 4) API existente

* `GET /health`
* `POST /analyzeRepo` (reanálisis bajo demanda)

---

## Cambios de frontend

### Estructura (Vite + React + TS)

```
frontend/
├── .env.example             # VITE_API_BASE=http://localhost:8080
├── src/
│   ├── vite-env.d.ts        # referencia a tipos de Vite (import.meta.env)
│   ├── services/api.ts      # wrapper fetch -> API backend
│   ├── pages/
│   │   ├── RegistrarProyecto.tsx  # form repo/branch -> POST /analyzeRepo
│   │   └── EstadoProyectos.tsx    # tabla dinámica -> /history + acciones
│   └── components/Navbar.tsx
```

### Pantallas

* **RegistrarProyecto**: envía `repo` y `branch`, muestra el JSON devuelto (incluye `type` y `evidence`).
* **EstadoProyectos**:

  * `useEffect` → `GET /history` y pinta la tabla.
  * Botones:

    * **Logs** → `GET /logs/{id}` → `alert` con mock.
    * **Redeploy** → `POST /redeploy/{id}` → `alert` con mock.
    * **Eliminar** → `DELETE /history/{id}` → recarga tabla.

---

## Errores encontrados y cómo se solucionaron

1. **CORS (Failed to fetch / preflight)**

   * **Síntoma**: consola del navegador → “No `Access-Control-Allow-Origin` header…”
   * **Causa**: el backend no respondía a preflight `OPTIONS` con los headers CORS.
   * **Fix**: middleware `enableCORS` que:

     * añade `Access-Control-Allow-Origin: *`
     * permite métodos y headers básicos
     * responde `200` a `OPTIONS` sin pasar al handler.

2. **TypeScript: “La propiedad ‘env’ no existe en ‘ImportMeta’”**

   * **Causa**: faltaba declaración de tipos de Vite.
   * **Fix**: archivo `src/vite-env.d.ts` con `/// <reference types="vite/client" />`
     (opcional: tipado explícito de `VITE_API_BASE`).

3. **Imports del backend no resueltos (module path)**

   * **Síntoma**: `package sammcore-deployer/api is not in std`
   * **Causa**: el `module` del `go.mod` no coincidía con los imports.
   * **Fix**: en `backend/go.mod` → `module sammcore-deployer` y usar imports internos:

     * `sammcore-deployer/api`, `sammcore-deployer/cli`, `sammcore-deployer/core`, `sammcore-deployer/services`, `sammcore-deployer/secrets`.

4. **Token de GitHub no detectado**

   * **Síntoma**: `[Secrets] ⚠️ No se encontró GITHUB_TOKEN`.
   * **Causa**: no había variable o `.env`.
   * **Fix**: `secrets.go` carga `.env` (en desarrollo) y/o lee `os.Getenv("GITHUB_TOKEN")`.
     En producción/K3s, se inyecta como **Secret** de Kubernetes (env var).
     Frontend **no** maneja el token.

---

## Cómo correr (Fase 3)

### Backend

```bash
cd backend
go mod tidy
go run .
# “🚀 sammcore-deployer API escuchando en http://localhost:8080”
```

### Frontend

```bash
cd frontend
cp .env.example .env    # si no lo has hecho
npm install
npm run dev
# abre http://localhost:5173
```

---

## Contrato de API (resumen)

* `POST /analyzeRepo`

  * Body: `{ "repo": string, "branch": string }`
  * Respuesta OK:

    ```json
    {
      "status": "ok",
      "workdir": ".../tmp/sammcore-deployer-XXXX",
      "type": "compose|dockerfile|unknown",
      "evidence": ["..."]
    }
    ```
  * Side effect: guarda entrada en `history.json`.

* `GET /history`

  * Respuesta:

    ```json
    [
      { "id":"ab12cd34", "repo":"...", "branch":"...", "type":"compose", "status":"analizado", "timestamp":"..." }
    ]
    ```

* `DELETE /history/{id}` → `{"status":"deleted"}`

* `GET /logs/{id}` → `{"id":"...","logs":"📜 Aquí aparecerían los logs de despliegue (mock)."}`

* `POST /redeploy/{id}` → `{"id":"...","status":"♻️ Redeploy iniciado (mock)."}`

* `GET /health` → `{"status":"ok"}`

---

## Qué quedó listo para Fase 4 (K3s)

* **Hooks**/endpoints (`/redeploy`, `/logs`) para conectar con el clúster.
* **Persistencia** del estado de proyectos para alimentar la UI.
* **Autenticación GitHub** consolidada en backend (secrets/env), sin exponer al frontend.

---

## Checklist de verificación

* [x] Registrar proyecto (repo público y privado) devuelve JSON y registra historial.
* [x] Estado de Proyectos lista dinámicamente desde `GET /history`.
* [x] Acciones `Logs / Redeploy / Eliminar` responden (mocks) y la UI refresca.
* [x] CORS habilitado para `http://localhost:5173`.
* [x] `import.meta.env` tipado con `vite-env.d.ts`.
* [x] Token GitHub gestionado solo en backend con `secrets.go`.
