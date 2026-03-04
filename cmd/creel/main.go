package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("creel server starting")
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// TODO: load config, initialize stores, start gRPC server
	return nil
}
