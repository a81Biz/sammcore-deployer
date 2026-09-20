package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"sammcore-deployer/core"
	"sammcore-deployer/services"
	"sammcore-deployer/storage"

	"github.com/gorilla/mux"
)

type APIError struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	Code   string `json:"code,omitempty"`
}

func writeJSONError(w http.ResponseWriter, statusCode int, message string, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(APIError{
		Status: "error",
		Error:  message,
		Code:   code,
	})
}

var activeDeployManager *services.DeployManager

func SetDeployManager(dm *services.DeployManager) {
	activeDeployManager = dm
}

func getDeployManager() *services.DeployManager {
	if activeDeployManager != nil {
		return activeDeployManager
	}
	kubeClient, err := services.GetKubeClient()
	if err == nil && kubeClient != nil {
		activeDeployManager = services.NewDeployManager(kubeClient)
	}
	return activeDeployManager
}

func getAllowedOrigins() map[string]bool {
	raw := os.Getenv("ALLOWED_ORIGINS")
	if raw == "" {
		raw = "https://deployer.sammcore.local,http://localhost:5173"
	}
	origins := make(map[string]bool)
	for _, o := range strings.Split(raw, ",") {
		clean := strings.TrimSpace(o)
		if clean != "" {
			origins[clean] = true
		}
	}
	return origins
}

func enableCORS(next http.Handler) http.Handler {
	origins := getAllowedOrigins()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")

		origin := r.Header.Get("Origin")
		if origin != "" {
			if origins[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			} else {
				if r.Method == "OPTIONS" {
					w.WriteHeader(http.StatusForbidden)
					return
				}
			}
		}

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedKey := os.Getenv("DEPLOYER_API_KEY")
		allowInsecure := os.Getenv("ALLOW_INSECURE_DEV") == "true"

		if expectedKey == "" && allowInsecure {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			writeJSONError(w, http.StatusUnauthorized, "Cabecera Authorization inválida. Debe tener formato 'Bearer <DEPLOYER_API_KEY>'", "UNAUTHORIZED_HEADER")
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		token = strings.TrimSpace(token)

		if subtle.ConstantTimeCompare([]byte(token), []byte(expectedKey)) != 1 {
			writeJSONError(w, http.StatusUnauthorized, "No autorizado: clave de API inválida", "INVALID_API_KEY")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func listProjectsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	projects, err := storage.LoadProjects()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error(), "INTERNAL_ERROR")
		return
	}
	_ = json.NewEncoder(w).Encode(projects)
}

func getProjectHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]
	p, err := storage.GetProject(id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error(), "PROJECT_NOT_FOUND")
		return
	}
	_ = json.NewEncoder(w).Encode(p)
}

func deleteProjectHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]
	deleteDB := r.URL.Query().Get("delete_db") == "true"

	p, err := storage.GetProject(id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Proyecto no encontrado", "PROJECT_NOT_FOUND")
		return
	}

	dm := getDeployManager()
	if dm != nil {
		_ = dm.DeleteProjectDeployment(r.Context(), p.Name, deleteDB)
	}

	if err := storage.DeleteProject(id); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error(), "INTERNAL_ERROR")
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": fmt.Sprintf("Proyecto %s eliminado exitosamente (delete_db=%v)", p.Name, deleteDB),
	})
}

func logsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]

	p, err := storage.GetProject(id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Proyecto no encontrado", "PROJECT_NOT_FOUND")
		return
	}

	dm := getDeployManager()
	if dm == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"id":   id,
			"logs": "Cliente Kubernetes no disponible en este entorno",
		})
		return
	}

	logs, err := dm.GetPodLogs(r.Context(), p.Namespace, 100)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error(), "LOGS_ERROR")
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]string{
		"id":   id,
		"logs": logs,
	})
}

func redeployHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]

	p, err := storage.GetProject(id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Proyecto no encontrado", "PROJECT_NOT_FOUND")
		return
	}

	p.Status = storage.StatusDeploying
	p.UpdatedAt = time.Now()
	_ = storage.AddOrUpdateProject(*p)

	dm := getDeployManager()
	if dm != nil {
		manifestParams := services.ProjectManifestParams{
			ProjectName:      p.Name,
			Namespace:        p.Namespace,
			Type:             p.Type,
			Domain:           p.Domain,
			APIDomain:        p.APIDomain,
			RequiresDatabase: p.RequiresDatabase,
			WebPort:          80,
			APIPort:          8000,
			AppPort:          8080,
			WebImage:         fmt.Sprintf("ghcr.io/a81biz/%s-web:latest", p.Name),
			APIImage:         fmt.Sprintf("ghcr.io/a81biz/%s-api:latest", p.Name),
			AppImage:         fmt.Sprintf("ghcr.io/a81biz/%s:latest", p.Name),
			StaticImage:      "nginx:alpine",
		}
		go func() {
			_ = dm.ExecuteDeploy(context.Background(), *p, manifestParams)
		}()
	}

	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "accepted",
		"message": "Re-despliegue iniciado correctamente",
		"project": p,
	})
}

func deployHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req services.DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "JSON inválido en el cuerpo de la petición", "INVALID_JSON")
		return
	}

	repo := strings.TrimSpace(req.Repo)
	if repo == "" {
		writeJSONError(w, http.StatusBadRequest, "El campo repo es obligatorio", "MISSING_REPO")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		analyzed := core.Analyze(core.AnalyzeRequest{Repo: repo, Branch: req.Branch})
		if analyzed.Status == "ok" {
			name = analyzed.Name
		} else {
			name = "app-" + strings.ToLower(core.DeriveDeterministicID(repo))
		}
	}

	pType := req.Type
	if pType == "" {
		pType = "compose"
	}

	branch := req.Branch
	if branch == "" {
		branch = "main"
	}

	reqDB := false
	if req.RequiresDatabase != nil {
		reqDB = *req.RequiresDatabase
	} else if pType == "compose" {
		reqDB = true
	}

	projectID := strings.ToLower(core.DeriveDeterministicID(repo))
	domain := fmt.Sprintf("%s.sammcore.local", name)
	var apiDomain string
	if pType == "compose" {
		apiDomain = fmt.Sprintf("%s-api.sammcore.local", name)
	}

	proj := storage.Project{
		ID:               projectID,
		Name:             name,
		Repo:             repo,
		Branch:           branch,
		Type:             pType,
		Namespace:        name,
		Domain:           domain,
		APIDomain:        apiDomain,
		RequiresDatabase: reqDB,
		Status:           storage.StatusProvisioning,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := storage.AddOrUpdateProject(proj); err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Error guardando proyecto: %v", err), "STORE_ERROR")
		return
	}

	dm := getDeployManager()
	if dm != nil {
		manifestParams := services.ProjectManifestParams{
			ProjectName:      name,
			Namespace:        name,
			Type:             pType,
			Domain:           domain,
			APIDomain:        apiDomain,
			RequiresDatabase: reqDB,
			WebImage:         req.WebImage,
			WebPort:          req.WebPort,
			APIImage:         req.APIImage,
			APIPort:          req.APIPort,
			AppImage:         req.AppImage,
			AppPort:          req.AppPort,
			StaticImage:      req.StaticImage,
		}
		if manifestParams.WebPort == 0 {
			manifestParams.WebPort = 80
		}
		if manifestParams.APIPort == 0 {
			manifestParams.APIPort = 8000
		}
		if manifestParams.AppPort == 0 {
			manifestParams.AppPort = 8080
		}
		if manifestParams.WebImage == "" {
			manifestParams.WebImage = fmt.Sprintf("ghcr.io/a81biz/%s-web:latest", name)
		}
		if manifestParams.APIImage == "" {
			manifestParams.APIImage = fmt.Sprintf("ghcr.io/a81biz/%s-api:latest", name)
		}
		if manifestParams.AppImage == "" {
			manifestParams.AppImage = fmt.Sprintf("ghcr.io/a81biz/%s:latest", name)
		}
		if manifestParams.StaticImage == "" {
			manifestParams.StaticImage = "nginx:alpine"
		}

		go func() {
			_ = dm.ExecuteDeploy(context.Background(), proj, manifestParams)
		}()
	}

	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "accepted",
		"message": "Despliegue iniciado correctamente",
		"project": proj,
	})
}

func analyzeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req core.AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "JSON inválido en el cuerpo de la petición", "INVALID_JSON")
		return
	}
	resp := core.Analyze(req)
	if resp.Status == "error" {
		w.WriteHeader(http.StatusBadRequest)
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func NewRouter() http.Handler {
	r := mux.NewRouter()

	// Rutas Públicas de Diagnóstico y Métricas
	r.Handle("/metrics", promhttp.Handler())
	r.HandleFunc("/health", healthHandler).Methods("GET")
	r.HandleFunc("/api/health", healthHandler).Methods("GET")

	// Subrouter protegido bajo /api
	apiRouter := r.PathPrefix("/api").Subrouter()
	apiRouter.Use(authMiddleware)

	apiRouter.HandleFunc("/analyzeRepo", analyzeHandler).Methods("POST")
	apiRouter.HandleFunc("/deploy", deployHandler).Methods("POST")
	apiRouter.HandleFunc("/projects", listProjectsHandler).Methods("GET")
	apiRouter.HandleFunc("/projects/{id}", getProjectHandler).Methods("GET")
	apiRouter.HandleFunc("/projects/{id}", deleteProjectHandler).Methods("DELETE")
	apiRouter.HandleFunc("/projects/{id}/logs", logsHandler).Methods("GET")
	apiRouter.HandleFunc("/projects/{id}/redeploy", redeployHandler).Methods("POST")

	return enableCORS(r)
}
