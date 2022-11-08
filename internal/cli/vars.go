// Copyright (c) 2022. Alvin Baena.
// SPDX-License-Identifier: MIT

package cli

var (
	// create, query, serve
	inputFile string
	// root
	verbose bool
	// root
	profile bool
	// root
	pprofPort uint16
	// create, download
	outFile string
	// create
	probability uint64
	// create
	indexGranularity uint64
	// query
	interactive bool
	// query
	hashed bool
	// download
	threads int
	// create, download
	overwrite bool
	// serve
	selfTLS bool
	// serve
	tlsCert string
	// serve
	tlsKey string
	// serve
	port uint16
)
