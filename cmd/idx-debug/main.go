package main

import (
	"encoding/gob"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"idx/internal/core/domain"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s <index_file>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Converts a binary gob index file to readable JSON for inspection.\n")
	}

	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}

	indexPath := flag.Arg(0)

	// Register types for gob decoding
	gob.Register(&domain.InvertedIndex{})
	gob.Register(&domain.TermStats{})
	gob.Register(&domain.DocTermStats{})
	gob.Register(&domain.DocStats{})

	// Open and decode the binary index
	f, err := os.Open(indexPath) //nolint:gosec
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = f.Close() }()

	var index domain.InvertedIndex
	decoder := gob.NewDecoder(f)
	if err := decoder.Decode(&index); err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding index: %v\n", err)
		os.Exit(1)
	}

	// Convert to JSON and print
	data, err := json.MarshalIndent(&index, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error converting to JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(data))
}
