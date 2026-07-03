package main

import (
	"fmt"
	"os"

	"github.com/josefdolezal/crew/internal/cli"
)

func main() {
	if err := cli.Root().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "crew:", err)
		os.Exit(1)
	}
}
