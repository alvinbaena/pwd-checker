package cli

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"os"
	"pwd-checker/internal/hibp"
	"pwd-checker/internal/util"
)

var (
	downloadCmd = &cobra.Command{
		Use:   "download",
		Short: "Download the haveibeenpwned hashes (SHA1) to a file",
		RunE: func(cmd *cobra.Command, args []string) error {
			return downloadCommand()
		},
	}
)

//goland:noinspection GoUnhandledErrorResult
func init() {
	downloadCmd.Flags().StringVarP(&outFile, "out-file", "o", "./pwned-sha1.txt", "Output file path")
	downloadCmd.Flags().BoolVar(&overwrite, "overwrite", false, "Overwrite any existing files while writing the results.")
	downloadCmd.Flags().IntVarP(&threads, "threads", "t", 0, "Number of threads to use for the download. If omitted or less than 2, defaults to eight times the number of processors on the machine.")

	rootCmd.AddCommand(downloadCmd)
}

func downloadCommand() error {
	util.ApplyCliSettings(verbose, profile, pprofPort)

	if !overwrite {
		_, err := os.Stat(outFile)
		if err == nil {
			return fmt.Errorf("file exists and overwrite flag is not set")
		}
	}

	file, err := os.Create(outFile)
	if err != nil {
		return err
	}

	defer func(file *os.File) {
		if err = file.Close(); err != nil {
			log.Error().Err(err).Msg("Error closing Pwned Passwords file")
		}
	}(file)

	d := hibp.NewDownloader(file, threads)
	if err = d.ProcessRanges(); err != nil {
		return err
	}

	return nil
}
