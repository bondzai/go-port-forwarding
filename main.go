// main.go
package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"gopkg.in/yaml.v2"
)

// Mapping represents a single remote-to-local port mapping
type Mapping struct {
	Remote string `yaml:"remote"`
	Local  string `yaml:"local"`
}

// Config represents the overall configuration structure
type Config struct {
	Mappings []Mapping `yaml:"mappings"`
}

func main() {
	// Parse command line arguments
	configFile := "config.yaml" // Default configuration file

	// Read configuration from the YAML file
	config, err := readConfig(configFile)
	if err != nil {
		fmt.Println("Error reading configuration:", err)
		return
	}

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	// Start port forwarding for each mapping
	for _, mapping := range config.Mappings {
		wg.Add(1)
		go func(mapping Mapping) {
			defer wg.Done()

			localListener, err := net.Listen("tcp", mapping.Local)
			if err != nil {
				fmt.Printf("Error starting local listener for %s: %s\n", mapping.Remote, err)
				return
			}
			defer localListener.Close()

			fmt.Printf("Port forwarding from %s to %s\n", mapping.Remote, mapping.Local)

			// Accept and handle incoming connections
			for {
				select {
				case <-sigCh:
					fmt.Println("Received signal. Shutting down port forwarding for", mapping.Remote)
					localListener.Close()
					return
				default:
					localConn, err := localListener.Accept()
					if err != nil {
						fmt.Printf("Error accepting local connection for %s: %s\n", mapping.Remote, err)
						continue
					}

					go handleConnection(localConn, mapping.Remote)
				}
			}
		}(mapping)
	}

	// Wait for all port forwarding instances to finish
	wg.Wait()
}

// readConfig reads the configuration from a YAML file
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

// handleConnection handles the forwarding of data between local and remote connections
func handleConnection(localConn net.Conn, remoteAddr string) {
	defer localConn.Close()

	// Connect to the remote address
	remoteConn, err := net.Dial("tcp", remoteAddr)
	if err != nil {
		fmt.Println("Error connecting to remote address:", err)
		return
	}
	defer remoteConn.Close()

	// Use a WaitGroup to wait for both directions to complete
	var wg sync.WaitGroup
	wg.Add(2)

	// Forward data between local and remote connections
	go copyData(localConn, remoteConn, &wg)
	go copyData(remoteConn, localConn, &wg)

	// Wait for both directions to complete
	wg.Wait()
}

// copyData copies data bidirectionally between two connections
func copyData(dst io.Writer, src io.Reader, wg *sync.WaitGroup) {
	defer wg.Done()

	_, err := io.Copy(dst, src)
	if err != nil {
		fmt.Println("Error copying data:", err)
	}

	// Close the destination connection if it implements io.Closer
	if closer, ok := dst.(io.Closer); ok {
		_ = closer.Close()
	}
}
