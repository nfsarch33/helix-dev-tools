package main

import (
	"os"

	"github.com/nfsarch33/cursor-tools/internal/cli"
)

var version = "dev"
var setVersion = cli.SetVersion
var executeCLI = cli.Execute
var exitMain = os.Exit

func main() {
	setVersion(version)
	if err := executeCLI(); err != nil {
		exitMain(1)
	}
}
