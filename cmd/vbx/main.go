package main

import (
	"fmt"
	"os"
	"strings"

	"vbx/pkg/transpiler"
)

const Version = "0.1.0-alpha"

func printUsage() {
	usage := `Visual Basic X (VBX) Compiler

Usage:
  vbx run <filepath.vbx>                    Transpile and execute a .vbx script
  vbx build <filepath.vbx> [options]        Compile .vbx script into a standalone executable
  vbx version                               Display version information

Build Options:
  -o <output>                               Specify output binary path
  --keep-c                                  Keep temporary intermediate .c file`
	fmt.Println(usage)
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

	case "build":
		var filePath string
		var outputPath string
		var keepC bool

		args := os.Args[2:]
		for i := 0; i < len(args); i++ {
			arg := args[i]
			if arg == "-o" || arg == "--output" {
				if i+1 >= len(args) {
					fmt.Fprintln(os.Stderr, "Error: missing output path for -o flag")
					os.Exit(1)
				}
				outputPath = args[i+1]
				i++
			} else if strings.HasPrefix(arg, "-o=") {
				outputPath = strings.TrimPrefix(arg, "-o=")
			} else if strings.HasPrefix(arg, "--output=") {
				outputPath = strings.TrimPrefix(arg, "--output=")
			} else if arg == "--keep-c" || arg == "-keep-c" {
				keepC = true
			} else if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "Error: unknown flag %q\n", arg)
				printUsage()
				os.Exit(1)
			} else {
				if filePath != "" {
					fmt.Fprintf(os.Stderr, "Error: multiple input files specified (%q, %q)\n", filePath, arg)
					os.Exit(1)
				}
				filePath = arg
			}
		}

		if filePath == "" {
			fmt.Fprintln(os.Stderr, "Error: missing input .vbx file path")
			printUsage()
			os.Exit(1)
		}

		builtPath, err := transpiler.Build(filePath, outputPath, keepC)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("[VBX] Successfully built: %s\n", builtPath)

	case "version", "-v", "--version":
		fmt.Printf("vbx version %s\n", Version)

	case "help", "-h", "--help":
		printUsage()

	default:
		fmt.Fprintf(os.Stderr, "Error: unknown command %q\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}
