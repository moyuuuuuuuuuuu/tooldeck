package main

import (
	"flag"
	"github.com/moyuuuuuuuuuuu/tooldeck/internal/platform"
	"log"
)

func main() {
	root := flag.String("data", "data", "data directory")
	output := flag.String("output", "", "new JSON snapshot path")
	flag.Parse()
	if *output == "" {
		log.Fatal("--output is required")
	}
	if err := platform.ExportState(*root, *output); err != nil {
		log.Fatal(err)
	}
}
