package portals

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/bondzai/portforward/internal/model"
	"gopkg.in/yaml.v2"
)

func StartPortForwarding(mapping model.Mapping, sigCh chan os.Signal, wg *sync.WaitGroup) {
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
		case <-model.ShutdownCh:
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

func ReadConfig(configFile string) (model.Config, error) {
	var config model.Config

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

	buffer := model.BufferPool.Get().([]byte)
	defer model.BufferPool.Put(&buffer)

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
