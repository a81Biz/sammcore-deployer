package main

import (
	"log"
	"net/http"
	"os"

	"sammcore-deployer/api"
	"sammcore-deployer/cli"
)

func main() {
	// Si hay flags (ej: -repo) → ejecutar como CLI
	if len(os.Args) > 1 && os.Args[1][0] == '-' {
		cli.Run()
		return
	}

	// Sino, levantar API
	r := api.NewRouter()
	log.Println("🚀 sammcore-deployer API escuchando en http://localhost:8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("❌ Error iniciando servidor: %v", err)
	}
}
