// main.go
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"gopkg.in/yaml.v2"
)

type Mapping struct {
	Remote string `yaml:"remote"`
	Local  string `yaml:"local"`
}

type Config struct {
	Mappings []Mapping `yaml:"mappings"`
}

var configFile = "config.yaml"
var shutdownCh = make(chan struct{})

func main() {
	config, err := readConfig(configFile)
	if err != nil {
		fmt.Println("Error reading configuration:", err)
		return
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		close(shutdownCh) // Signal all goroutines to start shutdown
	}()

	var wg sync.WaitGroup

	for _, mapping := range config.Mappings {
		wg.Add(1)
		go startPortForwarding(mapping, sigCh, &wg)
	}
	go startHTTPServer(&config, sigCh)

	wg.Wait()
}

func startHTTPServer(config *Config, sigCh chan os.Signal) {
	http.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			getConfigHandler(w, r, config)
		case http.MethodPost:
			createConfigHandler(w, r, config)
		case http.MethodPut:
			updateConfigHandler(w, r, config)
		case http.MethodDelete:
			deleteConfigHandler(w, r, config)
		default:
			http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/shutdown", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Initiating graceful shutdown")
		close(shutdownCh) // Trigger graceful shutdown
	})

	fmt.Println("HTTP server listening on :8080")
	http.ListenAndServe(":8080", nil)
}

func getConfigHandler(w http.ResponseWriter, r *http.Request, config *Config) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

func createConfigHandler(w http.ResponseWriter, r *http.Request, config *Config) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1048576))
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	var newConfig Config
	err = yaml.Unmarshal(body, &newConfig)
	if err != nil {
		http.Error(w, "Error unmarshalling request body", http.StatusBadRequest)
		return
	}

	*config = newConfig
	saveConfig(*config, configFile)

	fmt.Fprintln(w, "Configuration created successfully")
}

func updateConfigHandler(w http.ResponseWriter, r *http.Request, config *Config) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1048576))
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	var updateConfig Config
	err = yaml.Unmarshal(body, &updateConfig)
	if err != nil {
		http.Error(w, "Error unmarshalling request body", http.StatusBadRequest)
		return
	}

	config.Mappings = updateConfig.Mappings
	saveConfig(*config, configFile)

	fmt.Fprintln(w, "Configuration updated successfully")
}

func deleteConfigHandler(w http.ResponseWriter, r *http.Request, config *Config) {
	config.Mappings = nil
	saveConfig(*config, configFile)

	fmt.Fprintln(w, "Configuration deleted successfully")
}

func saveConfig(config Config, filename string) {
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

func startPortForwarding(mapping Mapping, sigCh chan os.Signal, wg *sync.WaitGroup) {
	defer wg.Done()

	localListener, err := net.Listen("tcp", mapping.Local)
	if err != nil {
		fmt.Printf("Error starting local listener for %s: %s\n", mapping.Remote, err)
		return
	}
	defer localListener.Close()

	fmt.Printf("Port forwarding from %s to %s\n", mapping.Remote, mapping.Local)

	connCh := make(chan net.Conn)
	errCh := make(chan error)

	go func() {
		for {
			conn, err := localListener.Accept()
			if err != nil {
				errCh <- err
				return
			}
			connCh <- conn
		}
	}()

	for {
		select {
		case <-shutdownCh:
			fmt.Println("Shutting down port forwarding for", mapping.Remote)
			return
		case err := <-errCh:
			if !strings.Contains(err.Error(), "use of closed network connection") {
				fmt.Printf("Error accepting local connection for %s: %s\n", mapping.Remote, err)
			}
			return
		case conn := <-connCh:
			wg.Add(1)
			go handleConnection(conn, mapping.Remote, wg)
		}
	}
}

func readConfig(configFile string) (Config, error) {
	var config Config

	file, err := os.Open(configFile)
	if err != nil {
		return config, err
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		return config, err
	}

	return config, nil
}

func handleConnection(localConn net.Conn, remoteAddr string, wg *sync.WaitGroup) {
	defer wg.Done()
	defer localConn.Close()

	remoteConn, err := net.Dial("tcp", remoteAddr)
	if err != nil {
		fmt.Println("Error connecting to remote address:", err)
		return
	}
	defer remoteConn.Close()

	var dataCopyWg sync.WaitGroup
	dataCopyWg.Add(2)

	go copyData(localConn, remoteConn, &dataCopyWg)
	go copyData(remoteConn, localConn, &dataCopyWg)

	dataCopyWg.Wait()
}

func copyData(dst io.Writer, src io.Reader, wg *sync.WaitGroup) {
	defer wg.Done()

	buffer := make([]byte, 4096)

	for {
		n, err := src.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "use of closed network connection" {
				break
			}
			fmt.Println("Error reading data:", err)
			break
		}

		_, err = dst.Write(buffer[:n])
		if err != nil {
			fmt.Println("Error writing data:", err)
		}
	}

	if closer, ok := dst.(io.Closer); ok {
		_ = closer.Close()
	}
}
