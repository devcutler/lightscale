package main

import (
	"fmt"
	"os"

	"github.com/devcutler/lightscale/cli/cli"
)

func main() {
	root := cli.New()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "lightscale:", err)
		os.Exit(1)
	}
}
