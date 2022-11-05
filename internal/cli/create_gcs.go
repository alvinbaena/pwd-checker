package cli

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"pwd-checker/internal/util"
	"pwd-checker/pkg/gcs"
)

var (
	createCmd = &cobra.Command{
		Use:   "create",
		Short: "Create a GCS database from a Pwned Passwords file (SHA1)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return createCommand()
		},
	}
)

//goland:noinspection GoUnhandledErrorResult
func init() {
	createCmd.Flags().Uint64VarP(&probability, "false-positive-rate", "p", 16777216, "False positive rate for queries, 1-in-p.")
	createCmd.Flags().Uint64VarP(&indexGranularity, "index-granularity", "g", 1024, "Entries per index point (16 bytes each).")
	createCmd.Flags().StringVarP(&inputFile, "in-file", "i", "", "Pwned passwords input file path (required)")
	createCmd.MarkFlagRequired("in-file")
	createCmd.Flags().StringVarP(&outFile, "out-file", "o", fmt.Sprintf("./pwned.gcs"), "GCS file output path")
	createCmd.Flags().BoolVar(&overwrite, "overwrite", false, "Overwrite any existing files while writing the results.")

	rootCmd.AddCommand(createCmd)
}

func createCommand() error {
	util.ApplyCliSettings(verbose, profile, pprofPort)

	file, err := os.Open(inputFile)
	if err != nil {
		return err
	}

	defer func(file *os.File) {
		if err = file.Close(); err != nil {
			log.Error().Err(err).Msg("error closing Pwned Passwords file")
		}
	}(file)

	abs, err := filepath.Abs(outFile)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get absolute path of file")
	}

	if !overwrite {
		_, err = os.Stat(abs)
		if !os.IsNotExist(err) {
			log.Fatal().Msgf("file %s exists and overwrite flag is not set", outFile)
		}
	}

	out, err := os.Create(abs)
	if err != nil {
		return err
	}

	defer func(out *os.File) {
		if err = out.Close(); err != nil {
			log.Error().Err(err).Msg("error closing GCS file")
		}
	}(out)

	builder := gcs.NewBuilder(file, out, probability, indexGranularity)
	if err = builder.Process(); err != nil {
		return err
	}

	return nil
}
