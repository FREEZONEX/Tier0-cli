package main

import (
	"os"

	"github.com/FREEZONEX/Tier0-cli/cmd/tier0"
)

func main() {
	if err := tier0.Execute(); err != nil {
		os.Exit(1)
	}
}
