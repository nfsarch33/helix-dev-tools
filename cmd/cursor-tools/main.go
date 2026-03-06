package main

import (
	"os"

	"github.com/nfsarch33/cursor-tools/internal/cli"
)

var version = "dev"

func main() {
	cli.SetVersion(version)
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
