package main

import (
	"fmt"

	"github.com/xbpk3t/docs-alfred/internal/docs/wiki"
	ghindex "github.com/xbpk3t/docs-alfred/internal/gh/index"
)

func main() {
	candidates, err := ghindex.LocalTopicCatalog(ghindex.LocalGHConfig{})
	if err != nil {
		fmt.Println("ERROR:", err)
		return
	}
	// Since FormatTopicCandidatesGrouped is unexported, call via the public wrapper
	fmt.Println(wiki.FormatTopicCandidatesGrouped(candidates))
}
