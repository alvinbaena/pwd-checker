package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/gin-contrib/logger"
	"github.com/gin-gonic/gin"
	"github.com/likexian/selfca"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/context"
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
		log.Fatal().Err(err).Msg("error loading configuration")
	}

	if !cfg.Debug {
		gin.SetMode(gin.ReleaseMode)
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(logger.SetLogger(logger.WithLogger(func(c *gin.Context, z zerolog.Logger) zerolog.Logger {
		return zerolog.New(gin.DefaultWriter).With().Timestamp().Logger()
	})))

	v1 := router.Group("/v1")

	pwned := v1.Group("/check")
	if err = api.RegisterQueryApi(pwned, cfg.GcsFile); err != nil {
		log.Fatal().Err(err).Msg("error initializing Query API")
	}

	srvAddr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{
		Addr:    srvAddr,
		Handler: router,
	}

	go func() {
		log.Info().Msgf("starting TLS Server on address: %s", srvAddr)
		if cfg.TLSCert != "" && cfg.TLSKey != "" {
			// service connections with tls certs
			if err = srv.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey); err != nil && err != http.ErrServerClosed {
				log.Fatal().Err(err).Msg("error starting server")
			}
		} else if cfg.SelfTLS {
			log.Warn().Msgf("using auto self-signed certificate for TLS. This is not recommended for production. Please consider using your own certificates.")
			caConfig := selfca.Certificate{
				IsCA:      true,
				KeySize:   2048,
				NotBefore: time.Now(),
				// 30 day self-signed cert.
				NotAfter: time.Now().Add(time.Duration(30*24) * time.Hour),
			}

			// generating the certificate
			certificate, key, err := selfca.GenerateCertificate(caConfig)
			if err != nil {
				log.Fatal().Err(err).Msg("error generating auto self-signed certificate")
			}

			pair, err := tls.X509KeyPair(
				pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate}),
				pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}),
			)
			if err != nil {
				log.Fatal().Err(err).Msg("error using auto self-signed certificate")
			}

			srv.TLSConfig = &tls.Config{
				Certificates: []tls.Certificate{pair},
			}

			// service connections with tls config, no need to pass files
			if err = srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				log.Fatal().Err(err).Msg("error starting server")
			}
		} else {
			log.Fatal().Msg("server requires TLS configuration to start.")
		}
	}()

	gracefulShutdown(srv)
}

func gracefulShutdown(srv *http.Server) {
	// Wait for interrupt signal to gracefully shut down the server with
	// a timeout.
	quit := make(chan os.Signal)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall. SIGKILL but can't be a catch, so don't need to add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Warn().Err(err).Msg("server Shutdown.")
	}
	// catching ctx.Done(). timeout of 5 seconds.
	select {
	case <-ctx.Done():
		// Nothing for now
	}
	log.Info().Msg("server exiting...")
}
