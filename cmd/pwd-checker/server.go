package main

import (
	"context"
	"fmt"
	"github.com/gin-contrib/logger"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"net/http"
	"os"
	"os/signal"
	"pwd-checker/internal/api"
	"syscall"
	"time"
)

func main() {
	cfg, err := api.LoadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("Error loading configuration")
	}

	file, err := os.Open(cfg.GcsFile)
	if err != nil {
		log.Fatal().Err(err).Msg("error initializing Server")
	}

	if !cfg.Debug {
		gin.SetMode(gin.ReleaseMode)
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(logger.SetLogger(logger.WithLogger(func(c *gin.Context, z zerolog.Logger) zerolog.Logger {
		return zerolog.New(gin.DefaultWriter)
	})))

	v1 := router.Group("/v1")

	pwned := v1.Group("/check")
	if err = api.RegisterQueryApi(pwned, file); err != nil {
		log.Fatal().Err(err).Msg("error initializing Query API")
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Port),
		Handler: router,
	}

	go func() {
		// service connections
		if err = srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Msgf("listen: %s\n", err)
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server with
	// a timeout.
	quit := make(chan os.Signal)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall. SIGKILL but can't be a catch, so don't need to add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("Shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if err = srv.Shutdown(ctx); err != nil {
		log.Warn().Err(err).Msg("Server Shutdown.")
	}
	// catching ctx.Done(). timeout of 5 seconds.
	select {
	case <-ctx.Done():
		if err = file.Close(); err != nil {
			log.Error().Err(err).Msg("Error closing file")
		}
	}
	log.Info().Msg("Server exiting...")
}
