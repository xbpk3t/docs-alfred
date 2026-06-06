package cmd

import "os"

func writeOutput(s string) error {
	_, err := os.Stdout.WriteString(s + "\n")

	return err
}
