package main

import (
	"fmt"
	"os"

	"github.com/iamfurkann/osint-engine/internal/cli"
)

// Build zamanında ayarlanır: go build -ldflags "-X main.version=v0.1.0"
var version = "v0.1.0-dev"

func main() {
	rootCmd := cli.NewRootCmd(version)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Hata: %v\n", err)
		os.Exit(1)
	}
}
