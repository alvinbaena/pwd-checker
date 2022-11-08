// Copyright (c) 2022. Alvin Baena.
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/gin-contrib/logger"
	"github.com/gin-gonic/gin"
	"github.com/likexian/selfca"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"net/http"
	"os"
	"os/signal"
	"pwd-checker/internal/api"
	"pwd-checker/internal/util"
	"syscall"
	"time"
)

var (
	serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "Serve the API for querying the Pwned Password GCS database",
		RunE: func(cmd *cobra.Command, args []string) error {
			return serveCommand()
		},
	}
)

//goland:noinspection GoUnhandledErrorResult
func init() {
	serveCmd.Flags().StringVarP(&inputFile, "in-file", "i", "", "Pwned Passwords GCS input file (required)")
	serveCmd.MarkFlagRequired("in-file")
	serveCmd.Flags().BoolVar(&selfTLS, "self-tls", false,
		"If the server should use a self-signed certificate when starting. The certificate is renewed on each server restart")
	serveCmd.Flags().StringVar(&tlsCert, "tls-cert", "", "Path to the PEM encoded TLS certificate to be used by the server")
	serveCmd.Flags().StringVar(&tlsCert, "tls-key", "", "Path to the PEM encoded TLS private key to be used by the server")
	serveCmd.Flags().Uint16VarP(&port, "port", "p", 3100, "Port to be used by the server")

	rootCmd.AddCommand(serveCmd)
}

func serveCommand() error {
	util.ApplyCliSettings(verbose, profile, pprofPort)
	if !verbose {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(logger.SetLogger(logger.WithLogger(func(c *gin.Context, z zerolog.Logger) zerolog.Logger {
		return zerolog.New(gin.DefaultWriter).With().Timestamp().Logger()
	})))

	v1 := router.Group("/v1")

	pwned := v1.Group("/check")
	if err := api.RegisterQueryApi(pwned, inputFile); err != nil {
		return fmt.Errorf("error initializing API: %s", err)
	}

	srvAddr := fmt.Sprintf(":%d", port)
	srv := &http.Server{
		Addr:    srvAddr,
		Handler: router,
	}

	go func() {
		log.Info().Msgf("starting TLS Server on address: %s", srvAddr)
		if tlsCert != "" && tlsKey != "" {
			// service connections with tls certs
			if err := srv.ListenAndServeTLS(tlsCert, tlsKey); err != nil && err != http.ErrServerClosed {
				log.Fatal().Err(err).Msg("error starting server")
			}
		} else if selfTLS {
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
			log.Fatal().Msg("server requires TLS configuration to start. " +
				"Please use either the --self-tls flag or set a certificate with the --tls-cert and --tls-key flags")
		}
	}()

	gracefulShutdown(srv)
	return nil
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
