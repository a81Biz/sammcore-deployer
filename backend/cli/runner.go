package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"sammcore-deployer/core"
)

func Run() {
	repoURL := flag.String("repo", "", "URL del repositorio a clonar")
	branch := flag.String("branch", "main", "Rama a clonar (default=main)")
	outputText := flag.Bool("text", false, "Salida en texto (default=JSON)")
	flag.Parse()

	if *repoURL == "" {
		fmt.Fprintln(os.Stderr, "ERROR: debe especificar -repo")
		os.Exit(1)
	}

	req := core.AnalyzeRequest{
		Repo:   *repoURL,
		Branch: *branch,
	}
	resp := core.Analyze(req)

	if *outputText {
		if resp.Status == "error" {
			fmt.Printf("ERROR: %s\n", resp.Error)
		} else {
			fmt.Printf("Proyecto detectado: %s\n", resp.Type)
			fmt.Println("Evidencia:")
			for _, e := range resp.Evidence {
				fmt.Printf(" - %s\n", e)
			}
		}
	} else {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(resp)
	}
}
