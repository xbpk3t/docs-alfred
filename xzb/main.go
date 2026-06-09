package main

import (
	"fmt"
	"os"

	"github.com/xbpk3t/docs-alfred/xzb/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
