// Copyright (c) 2022. Alvin Baena.
// SPDX-License-Identifier: MIT

package cli

import (
	"github.com/spf13/cobra"
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
