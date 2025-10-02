package main

import (
	"log"
	"net/http"
	"os"

	"sammcore-deployer/api"
	"sammcore-deployer/cli"
)

func main() {
	// CLI
	if len(os.Args) > 1 && os.Args[1][0] == '-' {
		cli.Run()
		return
	}

	// API
	r := api.NewRouter()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("🚀 sammcore-deployer API escuchando en http://localhost:%s", port)

	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("❌ Error iniciando servidor: %v", err)
	}
}
