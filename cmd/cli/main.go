package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/version"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	validate := flag.Bool("validate-config", false, "load and validate configuration from the environment, then exit")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("%s %s\n", version.Name, version.Version)
		os.Exit(0)
	}

	if *validate {
		if _, err := config.Load(); err != nil {
			fmt.Fprintf(os.Stderr, "config invalid: %v\n", err)
			os.Exit(2)
		}
		fmt.Fprintln(os.Stdout, "config ok")
		os.Exit(0)
	}

	fmt.Fprintln(os.Stderr, "usage: cli -validate-config | -version")
	os.Exit(1)
}
