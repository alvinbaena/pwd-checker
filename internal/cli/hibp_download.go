package cli

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"pwd-checker/internal/util"
	"pwd-checker/pkg/hibp"
)

var (
	downloadCmd = &cobra.Command{
		Use:   "download",
		Short: "Download the latest haveibeenpwned hashes (SHA1) to a file",
		RunE: func(cmd *cobra.Command, args []string) error {
			return downloadCommand()
		},
	}
)

//goland:noinspection GoUnhandledErrorResult
func init() {
	downloadCmd.Flags().StringVarP(&outFile, "out-file", "o", "./pwned-sha1.txt", "Output file path. Can be absolute or relative.")
	downloadCmd.Flags().BoolVar(&overwrite, "overwrite", false, "Overwrite any existing files while writing the results.")
	downloadCmd.Flags().IntVarP(&threads, "threads", "t", 0, "Number of threads to use for the download. If omitted or less than 2, defaults to eight times the number of logical processors of the machine.")

	rootCmd.AddCommand(downloadCmd)
}

func downloadCommand() error {
	util.ApplyCliSettings(verbose, profile, pprofPort)

	abs, err := filepath.Abs(outFile)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get absolute path of file")
	}

	if !overwrite {
		_, err := os.Stat(abs)
		if err == nil {
			log.Fatal().Msgf("file %s exists and overwrite flag is not set", abs)
		}
	}

	file, err := os.Create(abs)
	if err != nil {
		return err
	}

	defer func(file *os.File) {
		if err = file.Close(); err != nil {
			log.Error().Err(err).Msg("error closing Pwned Passwords file")
		}
	}(file)

	d := hibp.NewDownloader(file, threads)
	if err = d.ProcessRanges(); err != nil {
		return err
	}

	return nil
}
