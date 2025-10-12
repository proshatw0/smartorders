package main

import (
	"context"
	"fmt"
	"os"

	"smartorders/user-svc/internal/app"
	"smartorders/user-svc/internal/cli/keys"
	"smartorders/user-svc/internal/cli/migrations"
)

func main() {
	args := os.Args[1:]

	if len(args) > 0 {
		switch args[0] {
		case "keys":
			if err := keys.Run(args[1:]); err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}
			return
		case "migrate":
			if err := migrations.Run(args[1:]); err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}
			return
		}
	}

	if err := app.Run(context.Background(), args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
