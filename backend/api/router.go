package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"sammcore-deployer/core"
	"sammcore-deployer/storage"

	"github.com/gorilla/mux"
)

var allowedOrigins = map[string]bool{
	"https://deployer.sammcore.local": true,
	"http://localhost:5173":           true,
	"http://localhost:8080":           true,
}

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else if origin == "" {
			// Peticiones directas o locales sin header Origin
			w.Header().Set("Access-Control-Allow-Origin", "https://deployer.sammcore.local")
		}

		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

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
		// Si no se configuró clave en el entorno (ej. local dev), se permite el paso
		if expectedKey == "" {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		token := strings.TrimPrefix(authHeader, "Bearer ")
		token = strings.TrimSpace(token)

		if token != expectedKey {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"status": "error",
				"error":  "No autorizado: requiere Authorization: Bearer <DEPLOYER_API_KEY>",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func listProjectsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	projects, err := storage.LoadProjects()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(projects)
}

func getProjectHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]
	p, err := storage.GetProject(id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(p)
}

func deleteProjectHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]
	deleteDB := r.URL.Query().Get("delete_db") == "true"

	_ = storage.DeleteProject(id)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "deleted",
		"id":        id,
		"delete_db": deleteDB,
	})
}

func logsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]
	json.NewEncoder(w).Encode(map[string]string{
		"id":   id,
		"logs": "📜 Logs de despliegue (mock para Fase 4).",
	})
}

func redeployHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]
	json.NewEncoder(w).Encode(map[string]string{
		"id":     id,
		"status": "redeploy_initiated",
	})
}

func analyzeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req core.AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "error": "JSON inválido"})
		return
	}
	resp := core.Analyze(req)
	if resp.Status == "error" {
		w.WriteHeader(http.StatusBadRequest)
	}
	json.NewEncoder(w).Encode(resp)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func NewRouter() http.Handler {
	r := mux.NewRouter()

	// Rutas Públicas de Observabilidad
	r.Handle("/metrics", promhttp.Handler())
	r.HandleFunc("/health", healthHandler).Methods("GET")
	r.HandleFunc("/api/health", healthHandler).Methods("GET")

	// Subrouter para API protegida
	apiRouter := r.PathPrefix("/api").Subrouter()
	apiRouter.Use(authMiddleware)

	apiRouter.HandleFunc("/analyzeRepo", analyzeHandler).Methods("POST")
	apiRouter.HandleFunc("/projects", listProjectsHandler).Methods("GET")
	apiRouter.HandleFunc("/projects/{id}", getProjectHandler).Methods("GET")
	apiRouter.HandleFunc("/projects/{id}", deleteProjectHandler).Methods("DELETE")
	apiRouter.HandleFunc("/projects/{id}/logs", logsHandler).Methods("GET")
	apiRouter.HandleFunc("/projects/{id}/redeploy", redeployHandler).Methods("POST")

	// Alias de compatibilidad hacia atrás
	r.HandleFunc("/analyzeRepo", analyzeHandler).Methods("POST")
	r.HandleFunc("/history", listProjectsHandler).Methods("GET")
	r.HandleFunc("/history/{id}", deleteProjectHandler).Methods("DELETE")
	r.HandleFunc("/logs/{id}", logsHandler).Methods("GET")
	r.HandleFunc("/redeploy/{id}", redeployHandler).Methods("POST")

	return enableCORS(r)
}
