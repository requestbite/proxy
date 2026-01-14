package main

import (
	"fmt"
	"log"
	"os"

	"github.com/requestbite/proxy-go/internal/proxy"
	flag "github.com/spf13/pflag"
)

const (
	DefaultPort = 7331
)

var (
	Version   = "dev"       // Injected by build system from git tag
	BuildTime = "unknown"   // Injected by build system
	GitCommit = "unknown"   // Injected by build system
)

func main() {
	// Command line flags
	var (
		port             = flag.IntP("port", "p", DefaultPort, "Port to listen on")
		enableLocalFiles = flag.Bool("enable-local-files", false, "Enable local file and directory serving")
		blacklistFile    = flag.String("enable-blacklist", "", "Enable hostname blacklist from file (one hostname per line)")
		enableLogging    = flag.BoolP("logging", "l", false, "Enable verbose logging")
		showVersion      = flag.BoolP("version", "v", false, "Show version information")
		showHelp         = flag.BoolP("help", "h", false, "Show help information")
	)
	flag.Parse()

	// Show version
	if *showVersion {
		fmt.Printf("RequestBite Slingshot Proxy v%s\n", Version)
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
		fmt.Printf("RequestBite Slingshot Proxy v%s\n\n", Version)
		fmt.Println("Usage:")
		fmt.Printf("  %s [options]\n\n", os.Args[0])
		fmt.Println("Options:")
		flag.PrintDefaults()
		os.Exit(0)
	}

	// Start the proxy server
	server, err := proxy.NewServer(*port, Version, *enableLocalFiles, *blacklistFile, *enableLogging)
	if err != nil {
		log.Fatalf("Failed to create proxy server: %v", err)
	}

	fmt.Printf("RequestBite Slingshot Proxy v%s listening on port %d\n", Version, *port)
	if *enableLocalFiles {
		fmt.Println("\033[33mWarning:\033[0m Local file and dir serving enabled via /file and /dir endpoints")
	}
	if *blacklistFile != "" {
		fmt.Printf("\033[33mInfo:\033[0m Hostname blacklist enabled from file: %s\n", *blacklistFile)
	}
	fmt.Println("Press Ctrl+C to stop")

	if err := server.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
