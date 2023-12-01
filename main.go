// main.go
package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	// Parse command line arguments
	localAddr := flag.String("local", "localhost:6056", "Local address to bind to")
	remoteAddr := flag.String("remote", "localhost:5050", "Remote address to forward from")
	flag.Parse()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start the local listener
	localListener, err := net.Listen("tcp", *localAddr)
	if err != nil {
		fmt.Println("Error starting local listener:", err)
		return
	}
	defer localListener.Close()

	fmt.Printf("Port forwarding from %s to %s\n", *remoteAddr, *localAddr)

	var wg sync.WaitGroup

	// Accept and handle incoming connections
	for {
		select {
		case <-sigCh:
			fmt.Println("Received signal. Shutting down...")
			localListener.Close()
			wg.Wait()
			return
		default:
			localConn, err := localListener.Accept()
			if err != nil {
				fmt.Println("Error accepting local connection:", err)
				continue
			}

			wg.Add(1)
			go handleConnection(localConn, *remoteAddr, &wg)
		}
	}
}

func handleConnection(localConn net.Conn, remoteAddr string, wg *sync.WaitGroup) {
	defer wg.Done()

	// Connect to the remote address
	remoteConn, err := net.Dial("tcp", remoteAddr)
	if err != nil {
		fmt.Println("Error connecting to remote address:", err)
		localConn.Close()
		return
	}

	// Forward data between local and remote connections
	go copyData(localConn, remoteConn)
	go copyData(remoteConn, localConn)
}

func copyData(dst io.Writer, src io.Reader) {
	_, err := io.Copy(dst, src)
	if err != nil {
		fmt.Println("Error copying data:", err)
	}

	// Close both connections when copying is done
	if closer, ok := dst.(io.Closer); ok {
		closer.Close()
	}
	if closer, ok := src.(io.Closer); ok {
		closer.Close()
	}
}
