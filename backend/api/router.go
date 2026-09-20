package api

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"sammcore-deployer/core"
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
				// Origen no autorizado: no emitir cabeceras CORS y rechazar OPTIONS
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

		// En modo desarrollo inseguro sin clave definida se permite el paso
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

		// Comparación en tiempo constante para mitigar ataques de temporización
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
	// En Hito 4.0 no se simula éxito falso si no se tocan K8s ni DB
	writeJSONError(w, http.StatusNotImplemented, "Eliminación real de proyectos en K8s y PostgreSQL central pendiente de implementar en Hito 4.4", "NOT_IMPLEMENTED")
}

func logsHandler(w http.ResponseWriter, r *http.Request) {
	// Retorna 501 Not Implemented estricto en Hito 4.0
	writeJSONError(w, http.StatusNotImplemented, "📜 Logs de pods en vivo vía K8s API client-go pendiente de implementar en Hito 4.4", "NOT_IMPLEMENTED")
}

func redeployHandler(w http.ResponseWriter, r *http.Request) {
	writeJSONError(w, http.StatusNotImplemented, "Re-despliegue autónomo en K8s pendiente de implementar en Hito 4.4", "NOT_IMPLEMENTED")
}

func deployHandler(w http.ResponseWriter, r *http.Request) {
	writeJSONError(w, http.StatusNotImplemented, "Orquestación de despliegue en K3s (Hito 4.4) en desarrollo", "NOT_IMPLEMENTED")
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

	// Se eliminan por completo los alias no autenticados en la raíz
	return enableCORS(r)
}
