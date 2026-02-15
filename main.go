package main

import (
	"auth/auth"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/rs/zerolog"
)

var (
	log zerolog.Logger
)

func main() {
	if err := auth.ReadConfiguration(); err != nil {
		fmt.Println("failed to load configuration")
	}

	log = auth.GetLogger()
	log.Debug().Msgf("config loaded successfully: %v", auth.AppConfig)

	authServer := auth.NewAuthServer()
	authServer.Start()
	var wg sync.WaitGroup

	wg.Go(func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
		sig := <-quit
		log.Debug().
			Str("signal", sig.String()).
			Msg("Received shutdown signal")
		authServer.Shutdown()
		log.Info().Msg("service stopped gracefully")
	})

	wg.Wait()
}
