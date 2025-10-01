# 📘 Fase 2: API REST con Autenticación de GitHub

## 🎯 Objetivo

En esta fase se extendió `sammcore-deployer` para que, además del CLI inicial, ofreciera un **API REST** accesible desde `http://localhost:8080`.
Tanto el CLI como el API comparten un **único núcleo de lógica** que:

* Clona repositorios públicos y privados de GitHub.
* Detecta si el proyecto es:

  * **compose**: contiene `docker-compose.yml` o `docker-compose.yaml`.
  * **dockerfile**: contiene un `Dockerfile`.
  * **unknown**: no contiene ninguno de los anteriores (sin contenedor).

---

## 🛠 Cambios principales

### 1. Unificación de CLI y API

Antes, cada modo (CLI y API) ejecutaba el proceso de forma independiente.
Ahora ambos delegan al **mismo componente `core/analyzer.go`**, que centraliza la lógica de:

* Sanitizar parámetros (`repo`, `branch`).
* Crear un `RepoManager`.
* Ejecutar `Clone()` con fallback automático (`main → master → default`).
* Detectar el tipo de proyecto (`DetectProjectType()`).

De esta forma:

* **CLI** → maneja flags (`-repo`, `-branch`, etc.), llama a `core.Analyze()`.
* **API** → expone un endpoint `/analyzeRepo`, recibe JSON, llama a `core.Analyze()`.
* Ambos producen la misma salida en formato JSON.

---

### 2. Soporte para autenticación con GitHub

Para poder clonar repositorios privados se integró soporte de **autenticación con token**.

* Se agregó el paquete `secrets/secrets.go`, que cada vez que arranca la aplicación busca el token de forma segura:

  1. Primero carga un archivo `.env` local (solo en desarrollo).
  2. Luego revisa la variable de entorno `GITHUB_TOKEN`.
  3. Si no encuentra nada, muestra un warning en logs:

     ```
     [Secrets] ⚠️ No se encontró GITHUB_TOKEN
     ```

* En `repo_manager.go`, al inicializar `CloneOptions`, se asigna el token a `Auth`:

  ```go
  opts.Auth = &http.BasicAuth{
      Username: "git",          // requerido pero ignorado
      Password: token,          // tu GITHUB_TOKEN
  }
  ```

De esta manera:

* **En local**: basta con tener un `.env` con `GITHUB_TOKEN=...`.
* **En producción (K3s)**: se define un Secret en Kubernetes que inyecta `GITHUB_TOKEN` como variable de entorno.
* **En GitHub Actions (CI/CD)**: se usa `${{ secrets.GITHUB_TOKEN_CUSTOM }}`.

---

### 3. Ajuste del detector de proyectos

El detector se simplificó para esta fase:

* **Prioridad 1** → si existe `docker-compose.yml|yaml` → tipo `compose`.
* **Prioridad 2** → si existe `Dockerfile` → tipo `dockerfile`.
* **Caso contrario** → tipo `unknown`.

Esto permite distinguir rápidamente:

* Si el repo define un **contenedor de contenedores** (compose).
* Si define un **contenedor único** (dockerfile).
* Si es un proyecto sin contenedor (lenguaje puro, estático, etc.).

---

## 📂 Estructura final del backend

```
backend/
├── main.go            # Punto de entrada, decide CLI o API
├── api/
│   └── router.go      # Rutas REST (health, analyzeRepo)
├── cli/
│   └── runner.go      # CLI flags y salida JSON/texto
├── core/
│   └── analyzer.go    # Núcleo compartido (sanitiza, clona, detecta)
├── secrets/
│   └── secrets.go     # Helper para GITHUB_TOKEN
└── services/
    └── repo_manager.go # Lógica de clone y detección
```

---

## 🔧 Ejemplos de uso

### CLI

```bash
go run . -repo https://github.com/docker/awesome-compose.git -branch master
```

Salida:

```json
{
  "status": "ok",
  "type": "compose",
  "evidence": ["docker-compose.yaml"]
}
```

### API

Levantar servicio:

```bash
go run .
```

Healthcheck:

```bash
curl http://localhost:8080/health
# {"status":"ok"}
```

Analizar repo:

```bash
curl -X POST http://localhost:8080/analyzeRepo \
  -H "Content-Type: application/json" \
  -d '{"repo":"https://github.com/a81Biz/ecv-shortener.git","branch":"main"}'
```

Salida esperada:

```json
{
  "status": "ok",
  "type": "dockerfile",
  "evidence": ["Dockerfile"]
}
```

---

## ✅ Resultado de la Fase 2

* Se construyó un **API REST funcional** junto con el CLI.
* Ambos usan el mismo **núcleo `core.Analyze()`**.
* Se añadió soporte para **repositorios privados** con autenticación.
* Se simplificó la **detección de tipo de proyecto**.
* El sistema ahora está listo para correr en **desarrollo local**, **CI/CD** (GitHub Actions) y **producción en K3s** usando Secrets.
