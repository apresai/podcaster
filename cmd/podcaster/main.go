package main

import (
	"os"

	"github.com/chad/podcaster/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
