// Copyright (c) 2022. Alvin Baena.
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"net/http"
)

var (
	rootCmd = &cobra.Command{
		Use:   "pwdcheck [COMMAND] [OPTIONS]",
		Short: "Check a password against the Pwned Passwords dumps",
		Long: "Create and check passwords against the Pwned Passwords (haveibeenpwned.com) password dumps. " +
			"This command also helps you create a new GCS (Golomb Coded Set) file for a \"smaller\" file",
	}
)

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Print more information on the processing")
	rootCmd.PersistentFlags().BoolVar(&profile, "profile", false, "Enable the profiling server (pprof) when running commands")
	rootCmd.PersistentFlags().Uint16Var(&pprofPort, "profile-port", 6060, "The port to use for the pprof server. Only used if the profile flag is set")
}

func Execute() error {
	return rootCmd.Execute()
}

func applyCliSettings(verbose bool, profile bool, pprofPort uint16) {
	if verbose {
		log.Warn().Msgf("verbosity up")
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	if profile {
		log.Info().Msgf("profiling is enabled for this session. Server will listen on port %d", pprofPort)
		go func() {
			if err := http.ListenAndServe(fmt.Sprintf(":%d", pprofPort), nil); err != nil {
				log.Error().Err(err).Msgf("error starting profiling server on port %d", pprofPort)
				return
			}
		}()
	}
}
