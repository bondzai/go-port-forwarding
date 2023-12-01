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
	configFile := "config.yaml" // Default configuration file

	config, err := readConfig(configFile)
	if err != nil {
		fmt.Println("Error reading configuration:", err)
		return
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	for _, mapping := range config.Mappings {
		wg.Add(1)
		go startPortForwarding(mapping, sigCh, &wg)
	}

	wg.Wait()
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

			wg.Add(1)
			go handleConnection(localConn, mapping.Remote, wg)
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

	_, err := io.Copy(dst, src)
	if err != nil {
		fmt.Println("Error copying data:", err)
	}

	if closer, ok := dst.(io.Closer); ok {
		_ = closer.Close()
	}
}
