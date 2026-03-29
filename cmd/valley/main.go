package main

import (
	"errors"
	"fmt"
	"os"
)

func main() {
	if err := loadDotEnv(".env"); err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "warning: failed to load .env: %v\n", err)
	}
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
