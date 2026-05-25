package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/FREEZONEX/Tier0-cli/cmd"
	"github.com/FREEZONEX/Tier0-cli/internal/highrisk"
)

func main() {
	if err := cmd.Execute(); err != nil {
		var hre *highrisk.HighRiskError
		if errors.As(err, &hre) {
			highrisk.WriteEnvelope(os.Stderr, hre)
			os.Exit(10)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
