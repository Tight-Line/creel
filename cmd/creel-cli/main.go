package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// TODO: initialize cobra root command, load config from ~/.creel/config.yaml
	fmt.Println("creel-cli: not yet implemented")
	return nil
}
