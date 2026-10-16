package main

import (
	"fmt"
	"os"

	"github.com/songji-tian/agentdock/internal/cli"
)

func main() {
	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "agentdock:", err)
		os.Exit(1)
	}
}
