package api

import (
	"encoding/json"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"sammcore-deployer/core"
	"sammcore-deployer/storage"

	"github.com/gorilla/mux"
)

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Si es una preflight request, respondemos sin pasar al siguiente handler
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func historyHandler(w http.ResponseWriter, r *http.Request) {
	projects, _ := storage.LoadProjects()
	json.NewEncoder(w).Encode(projects)
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	storage.DeleteProject(id)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

func logsHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	json.NewEncoder(w).Encode(map[string]string{
		"id":   id,
		"logs": "📜 Aquí aparecerían los logs de despliegue (mock).",
	})
}

func redeployHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	json.NewEncoder(w).Encode(map[string]string{
		"id":     id,
		"status": "♻️ Redeploy iniciado (mock).",
	})
}

func NewRouter() http.Handler {
	r := mux.NewRouter()

	// métricas para Prometheus
	r.Handle("/metrics", promhttp.Handler())

	// Healthcheck
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}).Methods("GET")

	// AnalyzeRepo
	r.HandleFunc("/analyzeRepo", func(w http.ResponseWriter, r *http.Request) {
		var req core.AnalyzeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"status":"error","error":"JSON inválido"}`, http.StatusBadRequest)
			return
		}
		resp := core.Analyze(req)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}).Methods("POST")

	r.HandleFunc("/history", historyHandler).Methods("GET")
	r.HandleFunc("/history/{id}", deleteHandler).Methods("DELETE")
	r.HandleFunc("/logs/{id}", logsHandler).Methods("GET")
	r.HandleFunc("/redeploy/{id}", redeployHandler).Methods("POST")

	return enableCORS(r)
}
