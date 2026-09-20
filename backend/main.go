package main

import (
	"log"
	"net/http"
	"os"

	"sammcore-deployer/api"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Validación de seguridad: no permitir arrancar sin DEPLOYER_API_KEY
	// a menos que se declare explícitamente ALLOW_INSECURE_DEV=true
	apiKey := os.Getenv("DEPLOYER_API_KEY")
	allowInsecure := os.Getenv("ALLOW_INSECURE_DEV") == "true"
	if apiKey == "" && !allowInsecure {
		log.Fatalf("FATAL: DEPLOYER_API_KEY no está configurada en el entorno. Para arrancar en modo inseguro de desarrollo local, defina ALLOW_INSECURE_DEV=true.")
	}

	r := api.NewRouter()

	log.Printf("Iniciando SAMMCORE-Deployer API en el puerto %s...", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Error al iniciar el servidor: %v", err)
	}
}
