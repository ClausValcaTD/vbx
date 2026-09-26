package main

import (
	"fmt"
	"os"

	"vbx/pkg/transpiler"
)

func printUsage() {
	fmt.Println("Usage: vbx run <filepath.vbx>")
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	subcommand := os.Args[1]
	switch subcommand {
	case "run":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Error: missing input .vbx file path")
			printUsage()
			os.Exit(1)
		}
		filePath := os.Args[2]
		if err := transpiler.BuildAndRun(filePath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown command %q\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}
