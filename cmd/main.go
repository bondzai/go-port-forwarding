package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/bondzai/portforward/internal/api"
	"github.com/bondzai/portforward/internal/model"
	"github.com/bondzai/portforward/internal/portals"
)

func main() {
	config, err := portals.ReadConfig(model.ConfigFile)
	if err != nil {
		fmt.Println("Error reading configuration:", err)
		return
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		close(model.ShutdownCh)
	}()

	var wg sync.WaitGroup

	for _, mapping := range config.Mappings {
		wg.Add(1)
		go portals.StartPortForwarding(mapping, sigCh, &wg)
	}

	model.Once.Do(func() {
		go api.StartHTTPServer(&config, sigCh)
	})

	wg.Wait()
}
