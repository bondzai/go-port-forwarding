package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/bondzai/portforward/internal/model"
	"gopkg.in/yaml.v2"
)

func StartHTTPServer(config *model.Config, sigCh chan os.Signal) {
	http.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			getConfigHandler(w, r, config)
		case http.MethodPut:
			updateConfigHandler(w, r, config)
		default:
			http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/shutdown", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Initiating graceful shutdown")
		close(model.ShutdownCh)
	})

	fmt.Println("HTTP server listening on :8080")
	http.ListenAndServe(":8080", nil)
}

func getConfigHandler(w http.ResponseWriter, r *http.Request, config *model.Config) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

func updateConfigHandler(w http.ResponseWriter, r *http.Request, config *model.Config) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1048576))
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	var updateConfig model.Config
	err = json.Unmarshal(body, &updateConfig)
	if err != nil {
		http.Error(w, "Error unmarshalling request body", http.StatusBadRequest)
		return
	}

	config.Mappings = updateConfig.Mappings
	saveConfig(*config, model.ConfigFile)

	fmt.Fprintln(w, "Configuration updated successfully")
}

func saveConfig(config model.Config, filename string) {
	data, err := yaml.Marshal(&config)
	if err != nil {
		fmt.Println("Error marshalling configuration:", err)
		return
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		fmt.Println("Error writing configuration file:", err)
	}
}
