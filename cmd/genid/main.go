package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Albe83/id-service/internal/idgen"
)

func main() {
	count := flag.Int("n", 1, "number of IDs to generate (1-1000)")
	flag.Parse()

	g := idgen.NewGenerator(nil)

	if *count == 1 {
		id, err := g.NextID()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(idgen.String(id))
		return
	}

	ids, err := g.NextIDs(*count)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, id := range ids {
		fmt.Println(idgen.String(id))
	}
}
