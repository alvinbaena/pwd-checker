// Copyright (c) 2022. Alvin Baena.
// SPDX-License-Identifier: MIT

package main

import (
	"github.com/alvinbaena/pwd-checker/cmd"
	"github.com/rs/zerolog"
)

func main() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	_ = cmd.Execute()
}
