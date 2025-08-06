package main

import (
	"context"
	"fmt"
	"os"

	"smartorders/user-svc/internal/app"
	"smartorders/user-svc/internal/cli/keys"
)

func main() {
	args := os.Args[1:]

	if len(args) > 0 && args[0] == "keys" {
		if err := keys.Run(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}

	if err := app.Run(context.Background(), args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
