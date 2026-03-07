package main

import (
	"os"

	"github.com/ronikoz/atlas-recon/internal/cli"
)

func main() {
	os.Exit(cli.Execute(os.Args))
}

// Signed-off-by: ronikoz
