package main

import (
	_ "github.com/joho/godotenv/autoload"

	"github.com/xbpk3t/docs-alfred/workflow/pwgen/cmd"
)

func main() {
	cmd.Execute()
}
