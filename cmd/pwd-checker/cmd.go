package main

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"pwd-checker/internal/cli"
)

func main() {
	// we need a webserver to see the pprof
	go func() {
		if err := http.ListenAndServe("localhost:6060", nil); err != nil {
			return
		}
	}()

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	_ = cli.Execute()
}
