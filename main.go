package main

import (
	"fmt"
	"os"

	"github.com/FREEZONEX/Tier0-cli/cmd/tier0"
)

func main() {
	if err := tier0.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
