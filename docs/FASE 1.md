# 📘 Fase 1 — CLI mínimo de `sammcore-deployer`

## 🎯 Objetivo

Construir un **CLI inicial en Go** que permita:

1. Clonar un repositorio GitHub.
2. Detectar si contiene `docker-compose.yml`, `Dockerfile` o solo código estático.
3. Mostrar el resultado en **JSON por defecto** (con opción a texto plano).

Este CLI forma la base del backend del `sammcore-deployer` y será extendido en fases posteriores para integrarse con el frontend y Kubernetes.

---

## 📂 Estructura del repositorio

```
sammcore-deployer/
├── backend/
│   ├── handlers/
│   ├── services/
│   │   ├── repo_manager.go
│   │   └── repo_manager_test.go
│   ├── main.go
│   └── go.mod
├── .github/
│   └── workflows/
│       └── test.yml
└── README.md
```

* `backend/main.go`: CLI principal.
* `services/repo_manager.go`: Lógica de clonado y detección de proyecto.
* `services/repo_manager_test.go`: Pruebas unitarias.
* `.github/workflows/test.yml`: GitHub Actions para ejecutar `go test`.

---

## 🧪 Batería de pruebas local

### Preparar entorno

```bash
cd sammcore-deployer/backend
go mod tidy
```

### Ejecuciones básicas

* Proyecto con **Compose**:

```bash
go run . -repo https://github.com/docker/awesome-compose.git -branch master
```

* Proyecto con **Dockerfile**:

```bash
go run . -repo https://github.com/nginxinc/docker-nginx.git
```

* Proyecto **estático**:

```bash
go run . -repo https://github.com/twbs/bootstrap.git
```

* Proyecto **inválido**:

```bash
go run . -repo https://github.com/noexiste/404.git
```

### Salida JSON esperada (ejemplo Compose)

```json
{
  "status": "ok",
  "workdir": "/tmp/sammcore-deployer-12345",
  "type": "compose",
  "evidence": [
    "docker-compose.yml"
  ]
}
```

### Ejecutar pruebas unitarias

```bash
go test ./services -v
```

---

## ⚠️ Error encontrado en la Fase 1

Durante la primera prueba con:

```bash
go run . -repo https://github.com/docker/awesome-compose.git
```

obtuvimos:

```json
{
  "status": "error",
  "error": "error al clonar: no se pudo clonar repo: couldn't find remote ref \"refs/heads/main\""
}
```

📌 **Causa:**
El repositorio `awesome-compose` usa la rama **`master`** como predeterminada, no `main`.
Nuestro código inicial asumía siempre `main`, provocando el error.

---

## ✅ Solución implementada

Se modificó la función `Clone()` en `repo_manager.go` para añadir una **estrategia de fallback**:

1. Intentar clonar la rama solicitada (`main` por defecto).
2. Si falla, intentar con `master`.
3. Si aún falla, clonar sin especificar rama (Git usará la default del repo).

### Código del fix

```go
_, err := git.PlainClone(target, false, &git.CloneOptions{
    URL:           r.RepoURL,
    Progress:      progressWriter(r.Verbose),
    Depth:         1,
    SingleBranch:  true,
    ReferenceName: plumbing.NewBranchReferenceName(r.Branch),
})
if err == nil {
    r.Workdir = target
    return nil
}

// fallback a "master"
_, errMaster := git.PlainClone(target, false, &git.CloneOptions{
    URL:           r.RepoURL,
    Progress:      progressWriter(r.Verbose),
    Depth:         1,
    SingleBranch:  true,
    ReferenceName: plumbing.NewBranchReferenceName("master"),
})
if errMaster == nil {
    r.Workdir = target
    return nil
}

// fallback sin rama
_, errDefault := git.PlainClone(target, false, &git.CloneOptions{
    URL:          r.RepoURL,
    Progress:     progressWriter(r.Verbose),
    Depth:        1,
    SingleBranch: true,
})
if errDefault == nil {
    r.Workdir = target
    return nil
}

return fmt.Errorf("no se pudo clonar repo (branch=%s): %v | master: %v | default: %v",
    r.Branch, err, errMaster, errDefault)
```

Con esto, el CLI ahora funciona correctamente sin importar si el repo usa `main`, `master` o cualquier rama por defecto.

---

## 📌 Estado de la Fase 1

* [x] CLI mínimo en Go.
* [x] Salida JSON por defecto (texto opcional).
* [x] Detección de proyecto: Compose, Dockerfile, Static, Unknown.
* [x] Pruebas unitarias locales (`go test`).
* [x] Integración con GitHub Actions (CI).
* [x] Corrección de error de rama `main/master`.

