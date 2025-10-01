package api

import (
	"encoding/json"
	"net/http"

	"sammcore-deployer/core"

	"github.com/gorilla/mux"
)

func NewRouter() *mux.Router {
	r := mux.NewRouter()

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

	return r
}
