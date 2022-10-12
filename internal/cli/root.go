package cli

import (
	"github.com/spf13/cobra"
)

var (
	inputFile string
	verbose   bool

	rootCmd = &cobra.Command{
		Use:   "pwdcheck [COMMAND] [OPTIONS]",
		Short: "Check a password against the Pwned Passwords dumps",
		Long: "Create and check passwords against the Pwned Passwords (haveibeenpwned.com) password dumps. " +
			"This command also helps you create a new GCS (Golomb Coded Set) file for a \"smaller\" file",
	}
)

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "If it should print more info")
}

func Execute() error {
	return rootCmd.Execute()
}
