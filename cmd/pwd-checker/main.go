package main

import (
	"github.com/rs/zerolog/log"
	"net/http"
	_ "net/http/pprof"
	"pwd-checker/internal/config"
	"pwd-checker/internal/pwned"
)

func main() {
	// we need a webserver to get the pprof webserver
	go func() {
		err := http.ListenAndServe("localhost:6060", nil)
		if err != nil {
			return
		}
	}()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Error loading configuration")
	}

	// Load the Pwned Pwd database
	err = pwned.LoadPwnedPasswordsZinc("D:/Work/pwned-passwords-sha1-ordered-by-hash-v8.txt", cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Error loading Pwned Passwords database")
	}
}
