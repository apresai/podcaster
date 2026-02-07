package main

import (
	"os"

	"github.com/apresai/podcaster/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
