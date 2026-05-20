package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/FREEZONEX/Tier0-cli/cmd/tier0"
	"github.com/FREEZONEX/Tier0-cli/internal/highrisk"
)

func main() {
	if err := tier0.Execute(); err != nil {
		var hre *highrisk.HighRiskError
		if errors.As(err, &hre) {
			highrisk.WriteEnvelope(os.Stderr, hre)
			os.Exit(10)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
