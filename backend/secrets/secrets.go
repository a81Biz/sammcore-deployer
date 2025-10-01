package secrets

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

func GetGithubToken() string {
	// Cargar .env si existe
	_ = godotenv.Load()

	// Buscar variable en entorno
	token := os.Getenv("GITHUB_TOKEN")
	if token != "" {
		return strings.TrimSpace(token)
	}

	log.Println("[Secrets] ⚠️ No se encontró GITHUB_TOKEN")
	return ""
}
