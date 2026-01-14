package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

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
		enableExec       = flag.Bool("enable-exec", false, "Enable process execution via /exec endpoint")
		noUpgradeCheck   = flag.Bool("no-upgrade-check", false, "Disable automatic upgrade check")
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

	// Check for updates (unless disabled or running in development)
	if !*noUpgradeCheck && !isRunningInDevelopment() {
		checkForUpdates()
	}

	// Start the proxy server
	server, err := proxy.NewServer(*port, Version, *enableLocalFiles, *blacklistFile, *enableLogging, *enableExec)
	if err != nil {
		log.Fatalf("Failed to create proxy server: %v", err)
	}

	fmt.Printf("RequestBite Slingshot Proxy v%s listening on port %d\n", Version, *port)

	// Show security warnings for enabled features
	if *enableLocalFiles || *enableExec {
		fmt.Println("\033[31m╔═══════════════════════════════════════════════════════════════════════════╗")
		fmt.Println("║                                                                           ║")
		fmt.Println("║                                 WARNING!                                  ║")
		fmt.Println("║                                 ========                                  ║")
		fmt.Println("║                                                                           ║")

		if *enableLocalFiles {
			fmt.Println("║  You have enabled file browsing and the ability to read files via the     ║")
			fmt.Println("║  POST /dir and /file endpoints. This means clients (on localhost) can     ║")
			fmt.Println("║  read any directories and files your user has access to. Use with         ║")
			fmt.Println("║  caution.                                                                 ║")

			if *enableExec {
				fmt.Println("║                                                                           ║")
			}
		}

		if *enableExec {
			fmt.Println("║  You have enabled local execution of processes via the POST /exec         ║")
			fmt.Println("║  endpoint. This means clients (on localhost) can execute any process      ║")
			fmt.Println("║  as your user. Use with extreme caution.                                  ║")
		}

		fmt.Println("║                                                                           ║")
		fmt.Println("╚═══════════════════════════════════════════════════════════════════════════╝\033[0m")
	}

	if *blacklistFile != "" {
		fmt.Printf("\033[33mInfo:\033[0m Hostname blacklist enabled from file: %s\n", *blacklistFile)
	}
	fmt.Println("Press Ctrl+C to stop")

	if err := server.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// isRunningInDevelopment detects if the binary is running in a development environment (e.g., with Air)
func isRunningInDevelopment() bool {
	// Check for Air-specific environment variables
	if os.Getenv("AIR_WATCH") != "" || os.Getenv("AIR_TMP_DIR") != "" {
		return true
	}

	// Check if running from a tmp directory (common with Air)
	execPath, err := os.Executable()
	if err == nil && strings.Contains(execPath, "tmp") {
		return true
	}

	// Check if version is "dev" (unbuilt or development build)
	if Version == "dev" {
		return true
	}

	return false
}

// HealthResponse represents the response from the health endpoint
type HealthResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	UserAgent string `json:"user-agent"`
}

// getRemoteVersion fetches the latest version from the health endpoint with a 2-second timeout
func getRemoteVersion() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", "https://p.requestbite.com/health", nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var healthResp HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
		return "", err
	}

	return healthResp.Version, nil
}

// checkForUpdates checks if a new version is available and prompts the user to install it
func checkForUpdates() {
	remoteVersion, err := getRemoteVersion()
	if err != nil {
		// Silently fail - don't interrupt the user experience if update check fails
		return
	}

	// Compare versions (simple string comparison)
	if remoteVersion == Version || remoteVersion == "" {
		return // No update available or same version
	}

	// Notify user about new version
	fmt.Printf("\n\033[33mThere is a new version of RequestBite Proxy available.\033[0m\n")
	fmt.Printf("You're running v%s and the new version is v%s.\n\n", Version, remoteVersion)

	// Handle platform-specific installation
	if runtime.GOOS == "windows" {
		fmt.Println("See https://github.com/requestbite/proxy/ for installation details.\n")
		return
	}

	// Prompt for installation on Mac/Linux
	fmt.Print("Do you want to install (Y/N): ")
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("\nContinuing with current version...")
		return
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response == "y" || response == "yes" {
		fmt.Println("\nInstalling update...")
		if err := installUpdate(); err != nil {
			fmt.Printf("\033[31mFailed to install update: %v\033[0m\n", err)
			fmt.Println("Please visit https://github.com/requestbite/proxy/ for manual installation.\n")
		} else {
			fmt.Println("\033[32mUpdate installed successfully!\033[0m")
			fmt.Println("Please restart the proxy to use the new version.\n")
			os.Exit(0)
		}
	} else {
		fmt.Println("\nContinuing with current version...")
	}
	fmt.Println()
}

// installUpdate runs the installation script
func installUpdate() error {
	cmd := exec.Command("bash", "-c", "curl -fsSL https://raw.githubusercontent.com/requestbite/proxy/main/install.sh | bash")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
