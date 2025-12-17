package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/requestbite/proxy-go/internal/proxy"
)

const (
	DefaultPort = 8080
)

var (
	Version   = "dev"       // Injected by build system from git tag
	BuildTime = "unknown"   // Injected by build system
	GitCommit = "unknown"   // Injected by build system
)

func main() {
	// Command line flags
	var (
		port             = flag.Int("port", DefaultPort, "Port to listen on")
		enableLocalFiles = flag.Bool("enable-local-files", false, "Enable local file and directory serving")
		showVersion      = flag.Bool("version", false, "Show version information")
		showHelp         = flag.Bool("help", false, "Show help information")
	)
	flag.Parse()

	// Show version
	if *showVersion {
		fmt.Printf("RequestBite Slingshot Proxy (Go) v%s\n", Version)
		if BuildTime != "unknown" {
			fmt.Printf("Built: %s\n", BuildTime)
		}
		if GitCommit != "unknown" {
			fmt.Printf("Commit: %s\n", GitCommit)
		}
		os.Exit(0)
	}

	// Show help
	if *showHelp {
		fmt.Printf("RequestBite Slingshot Proxy (Go) v%s\n\n", Version)
		fmt.Println("Usage:")
		fmt.Printf("  %s [options]\n\n", os.Args[0])
		fmt.Println("Options:")
		flag.PrintDefaults()
		os.Exit(0)
	}

	// Start the proxy server
	server, err := proxy.NewServer(*port, Version, *enableLocalFiles)
	if err != nil {
		log.Fatalf("Failed to create proxy server: %v", err)
	}

	fmt.Printf("RequestBite Slingshot Proxy listening on port %d\n", *port)
	if *enableLocalFiles {
		fmt.Println("\033[33mWarning:\033[0m Local file and dir serving enabled via /file and /dir endpoints")
	}
	fmt.Println("Press Ctrl+C to stop")

	if err := server.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
