// Package main provides the Meanwhile CLI.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	exitSuccess = 0
	exitUsage   = 2
	exitFailure = 1
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(exitUsage)
	}

	switch os.Args[1] {
	case "version":
		printVersion(os.Stdout)
		os.Exit(exitSuccess)
	case "run":
		if err := run(os.Args[2:], os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(exitFailure)
		}
		os.Exit(exitSuccess)
	case "start":
		if err := start(os.Args[2:], os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(exitFailure)
		}
		os.Exit(exitSuccess)
	case "help", "-h", "--help":
		usage(os.Stdout)
		os.Exit(exitSuccess)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage(os.Stderr)
		os.Exit(exitUsage)
	}
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "Meanwhile CLI")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  meanwhile version")
	fmt.Fprintln(w, "  meanwhile run [--config path] [--protocol name] [--topic text] [--output format]")
	fmt.Fprintln(w, "  meanwhile start [--port 3000]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  version   Print build info")
	fmt.Fprintln(w, "  run       Run a session headlessly (scaffold)")
	fmt.Fprintln(w, "  start     Start Studio (scaffold)")
}

func printVersion(w io.Writer) {
	fmt.Fprintf(w, "meanwhile %s\n", version)
	fmt.Fprintf(w, "commit: %s\n", commit)
	fmt.Fprintf(w, "built:  %s\n", date)
}

func run(args []string, w io.Writer) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var (
		configPath string
		protocol   string
		topic      string
		output     string
	)

	fs.StringVar(&configPath, "config", "", "Path to config file")
	fs.StringVar(&protocol, "protocol", "", "Protocol name")
	fs.StringVar(&topic, "topic", "", "Session topic")
	fs.StringVar(&output, "output", "text", "Output format: text|json|yaml")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	output = strings.ToLower(strings.TrimSpace(output))
	if output == "" {
		output = "text"
	}

	if output != "text" && output != "json" && output != "yaml" {
		return fmt.Errorf("unsupported output format: %s", output)
	}

	if configPath == "" && protocol == "" {
		return fmt.Errorf("run requires --config or --protocol")
	}

	fmt.Fprintln(w, "run: scaffold only (no execution yet)")
	if configPath != "" {
		fmt.Fprintf(w, "config: %s\n", configPath)
	}
	if protocol != "" {
		fmt.Fprintf(w, "protocol: %s\n", protocol)
	}
	if topic != "" {
		fmt.Fprintf(w, "topic: %s\n", topic)
	}
	fmt.Fprintf(w, "output: %s\n", output)
	return nil
}

func start(args []string, w io.Writer) error {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	port := fs.Int("port", 3000, "Studio port")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	fmt.Fprintln(w, "studio: scaffold only (not implemented yet)")
	fmt.Fprintf(w, "port: %d\n", *port)
	return nil
}
