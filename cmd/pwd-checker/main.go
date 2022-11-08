// Copyright (c) 2022. Alvin Baena.
// SPDX-License-Identifier: MIT

package main

import (
	"github.com/rs/zerolog"
	"pwd-checker/internal/cli"
)

func main() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	_ = cli.Execute()
}
