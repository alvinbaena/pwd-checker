// Copyright (c) 2022. Alvin Baena.
// SPDX-License-Identifier: MIT

package cmd

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"github.com/alvinbaena/pwd-checker/gcs"
	"github.com/manifoldco/promptui"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"os"
	"regexp"
	"strings"
)

var (
	queryCmd = &cobra.Command{
		Use:   "query",
		Short: "Query the Pwned Passwords GCS database file",
		Args: func(cmd *cobra.Command, args []string) error {
			if !interactive {
				if err := cobra.MinimumNArgs(1)(cmd, args); err != nil {
					return err
				}
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if interactive {
				// Dummy string
				return queryCommand("")
			} else {
				return queryCommand(args[0])
			}
		},
	}
)

//goland:noinspection GoUnhandledErrorResult
func init() {
	queryCmd.Flags().StringVarP(&inputFile, "in-file", "i", "", "Pwned Passwords GCS input file (required)")
	queryCmd.MarkFlagRequired("in-file")
	queryCmd.Flags().BoolVarP(&interactive, "interactive", "n", false, "Interactive mode.")
	queryCmd.Flags().BoolVarP(&hashed, "hashed", "s", false, "If the supplied password will be a Hexadecimal SHA1 hash or a plain text string.")

	rootCmd.AddCommand(queryCmd)
}

func queryCommand(password string) (err error) {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	applyCliSettings(verbose, profile, pprofPort)

	searcher := gcs.NewReader(inputFile)
	if err = searcher.Initialize(); err != nil {
		return
	}

	var hash uint64
	if interactive {
		var label string
		if hashed {
			label = "SHA1 Hex hash"
		} else {
			label = "Password"
		}

		prompt := promptui.Prompt{
			Label: label,
			Validate: func(input string) error {
				if len(input) == 0 {
					return fmt.Errorf("please enter a valid password")
				}

				if hashed {
					match, _ := regexp.MatchString("^[a-fA-F\\d]{40}$", password)
					if !match {
						return fmt.Errorf("input is not a valid SHA1 Hexadecimal hash")
					}
				}
				return nil
			},
		}

		if !hashed {
			prompt.Mask = '*'
		} else {
			log.Info().Msgf("flag 'hashed' is set. Please use SHA1 Hashed passwords.")
		}

		log.Info().Msgf("running interactive session. ^C to exit")
		if err = runInteractiveSession(prompt, searcher); err != nil {
			if err.Error() == "^C" || err.Error() == "^D" {
				log.Info().Msgf("Goodbye")
			} else {
				log.Error().Err(err).Msgf("error during interactive session")
			}
			// No return to avoid the default cobra error message
			return nil
		}
	} else {
		hash, err = processPassword(password)
		if err != nil {
			return
		}

		return queryDatabase(hash, searcher)
	}

	return
}

func runInteractiveSession(prompt promptui.Prompt, searcher *gcs.Reader) error {
	for {
		result, err := prompt.Run()
		if err != nil {
			return err
		}

		hash, err := processPassword(result)
		if err != nil {
			log.Error().Err(err).Msg("error processing input")
		}

		if err = queryDatabase(hash, searcher); err != nil {
			log.Error().Err(err).Msg("error during query")
		}
	}
}

func queryDatabase(hash uint64, searcher *gcs.Reader) error {
	exists, err := searcher.Exists(hash)
	if err != nil {
		return err
	}

	if exists {
		log.Info().Msgf("password is present")
	} else {
		log.Info().Msgf("password is not present")
	}

	return nil
}

func processPassword(password string) (uint64, error) {
	if hashed {
		if match, _ := regexp.MatchString("^[a-fA-F\\d]{40}$", password); !match {
			return 0, fmt.Errorf("input is not a valid SHA1 Hexadecimal hash")
		}

		// The hash must be uppercase
		return gcs.U64FromHex([]byte(strings.ToUpper(password))[0:16]), nil
	} else {
		h := sha1.New()
		h.Write([]byte(password))
		buf := h.Sum(nil)
		val := binary.BigEndian.Uint64(buf)
		return val, nil
	}
}
