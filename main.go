/*
 * Copyright (c) 2017-2018 Miguel Ángel Ortuño.
 * See the COPYING file for more information.
 */

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ortuman/jackal/config"
	"github.com/ortuman/jackal/version"
)

func main() {
	var configFile string
	var showVersion bool
	var showUsage bool

	flag.BoolVar(&showUsage, "help", false, "show application usage")
	flag.BoolVar(&showVersion, "version", false, "show application version")
	flag.StringVar(&configFile, "config", "/etc/jackal/jackal.yaml", "configuration path file")
	flag.Parse()

	// print usage
	if showUsage {
		flag.Usage()
		os.Exit(-1)
	}

	// print version
	if showVersion {
		fmt.Printf("jackal version: %v", version.ApplicationVersion)
		os.Exit(-1)
	}

	// load configuration
	if err := config.Load(configFile); err != nil {
		fmt.Fprintf(os.Stderr, "jackal: %v", err)
		os.Exit(-1)
	}
	if len(config.DefaultConfig.Servers) > 0 {
		fmt.Fprint(os.Stderr, "jackal: couldn't find a server configuration")
		os.Exit(-1)
	}

	// initialize logger subsystem
}
