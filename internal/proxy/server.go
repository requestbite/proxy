package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// Server handles HTTP proxy requests
type Server struct {
	port             int
	httpClient       *HTTPClient
	server           *http.Server
	logger           *log.Logger
	blockedHostnames []string // Configurable list of hostnames to block (prevents loops)
	version          string   // Version for health endpoint
	enableLocalFiles bool     // Enable local file serving via /file endpoint
}

// NewServer creates a new proxy server instance
func NewServer(port int, version string, enableLocalFiles bool, blacklistFile string) (*Server, error) {
	logger := log.New(log.Writer(), "[PROXY] ", log.LstdFlags)

	// CONFIGURABLE: List of hostnames to block to prevent loops
	// Add/remove hostnames as needed for your deployment
	blockedHostnames := []string{
		"p.requestbite.com",
		"dev.p.requestbite.com",
	}

	// Load additional hostnames from blacklist file if provided
	if blacklistFile != "" {
		additionalHosts, err := loadBlacklistFile(blacklistFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load blacklist file: %v", err)
		}
		blockedHostnames = append(blockedHostnames, additionalHosts...)
		logger.Printf("Loaded %d hostname(s) from blacklist file: %s", len(additionalHosts), blacklistFile)
	}

	return &Server{
		port:             port,
		httpClient:       NewHTTPClient(version),
		logger:           logger,
		blockedHostnames: blockedHostnames,
		version:          version,
		enableLocalFiles: enableLocalFiles,
	}, nil
}

// loadBlacklistFile reads a blacklist file and returns a list of hostnames
// Format: one hostname per line, optionally with description after colon
// Example:
//   p.requestbite.com: Production proxy
//   127.0.0.1: Localhost
//   # This is a comment
func loadBlacklistFile(filename string) ([]string, error) {
	// Read file
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var hostnames []string
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		// Trim whitespace
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Extract hostname (everything before colon, or entire line if no colon)
		hostname := line
		if idx := strings.Index(line, ":"); idx != -1 {
			hostname = strings.TrimSpace(line[:idx])
		}

		// Skip if hostname is empty after extraction
		if hostname == "" {
			continue
		}

		hostnames = append(hostnames, hostname)
	}

	return hostnames, nil
}

// Start starts the HTTP server
func (s *Server) Start() error {
	router := mux.NewRouter()

	// CORS middleware
	router.Use(s.corsMiddleware)

	// Request logging middleware
	router.Use(s.loggingMiddleware)

	// Root endpoint
	router.HandleFunc("/", s.handleRoot).Methods("GET", "OPTIONS")

	// API endpoints
	router.HandleFunc("/proxy/request", s.handleJSONRequest).Methods("POST", "OPTIONS")
	router.HandleFunc("/proxy/form", s.handleFormRequest).Methods("POST", "OPTIONS")
	router.HandleFunc("/file", s.handleFileRequest).Methods("POST", "OPTIONS")
	router.HandleFunc("/dir", s.handleDirectoryRequest).Methods("POST", "OPTIONS")

	// Health check endpoint
	router.HandleFunc("/health", s.handleHealthCheck).Methods("GET", "OPTIONS")

	// Custom 404 handler
	router.NotFoundHandler = http.HandlerFunc(s.handleNotFound)

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: router,
	}

	return s.server.ListenAndServe()
}

// Stop stops the HTTP server gracefully
func (s *Server) Stop(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

// isLoopbackRequest checks if a request URL would create a loop back to this proxy
func (s *Server) isLoopbackRequest(targetURL string) bool {
	// Parse the target URL
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return false // Invalid URL, let validation handle it
	}

	// Allow /health endpoint on any hostname (required for proxy health checks)
	if parsedURL.Path == "/health" {
		return false
	}

	// Extract hostname (ignore port)
	targetHost := parsedURL.Hostname()

	// Check if target hostname is in our blocked list
	return s.isBlockedHostname(targetHost)
}

// isBlockedHostname checks if a hostname is in the blocked list
func (s *Server) isBlockedHostname(hostname string) bool {
	// Check against the configurable blocked hostnames list
	for _, blockedHost := range s.blockedHostnames {
		if strings.EqualFold(hostname, blockedHost) {
			return true
		}
	}
	return false
}

// isProxyUserAgent checks if the incoming request has the proxy's User-Agent
// This prevents infinite loops where the proxy calls itself
func (s *Server) isProxyUserAgent(r *http.Request) bool {
	userAgent := r.Header.Get("User-Agent")
	if userAgent == "" {
		return false
	}

	// Case-insensitive check for "rb-slingshot" substring
	// This catches: "rb-slingshot/0.1.0 (https://requestbite.com/slingshot)"
	return strings.Contains(strings.ToLower(userAgent), "rb-slingshot")
}

// detectLoop checks for potential infinite loops using multiple strategies:
// 1. User-Agent detection (prevents any proxy instance from calling another)
// 2. Hostname blocking (prevents targeting known production domains)
func (s *Server) detectLoop(r *http.Request, targetURL string) bool {
	// Strategy 1: Check incoming User-Agent header
	if s.isProxyUserAgent(r) {
		s.logger.Printf("BLOCKED loop: rb-slingshot User-Agent detected from %s targeting %s",
			r.RemoteAddr, targetURL)
		return true
	}

	// Strategy 2: Check target URL hostname
	if s.isLoopbackRequest(targetURL) {
		s.logger.Printf("BLOCKED loop: hostname blocking prevented request to: %s", targetURL)
		return true
	}

	return false
}

// handleJSONRequest handles /proxy/request endpoint
func (s *Server) handleJSONRequest(w http.ResponseWriter, r *http.Request) {
	// Handle OPTIONS for CORS preflight
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Parse request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeErrorResponse(w, "request_format_error", "Failed to read request body", err.Error())
		return
	}

	var req ProxyRequest
	if err := json.Unmarshal(body, &req); err != nil {
		s.writeErrorResponse(w, "request_format_error", "Invalid JSON", fmt.Sprintf("Failed to parse JSON request: %v", err))
		return
	}

	// Validate required fields
	if req.Method == "" {
		s.writeErrorResponse(w, "request_format_error", "Missing Method", "HTTP method is required")
		return
	}

	if req.URL == "" {
		s.writeErrorResponse(w, "request_format_error", "Missing URL", "URL is required")
		return
	}

	// Set default timeout if not provided
	if req.Timeout == 0 {
		req.Timeout = 60 // default 60 seconds
	}

	// Substitute path parameters if provided
	if req.PathParams != nil {
		req.URL = s.httpClient.SubstitutePathParams(req.URL, req.PathParams)
	}

	// Check for self-loop AFTER path parameter substitution
	if s.detectLoop(r, req.URL) {
		s.writeLoopErrorResponse(w, "Request could create an infinite loop to this proxy server")
		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(req.Timeout)*time.Second)
	defer cancel()

	// Log the request
	s.logger.Printf("%s %s", req.Method, req.URL)

	// Check if streaming is requested
	if req.Streaming {
		s.logger.Printf("Streaming mode enabled for request")
		// Execute the streaming request
		if err := s.httpClient.ExecuteStreamingRequest(ctx, &req, w); err != nil {
			s.logger.Printf("Streaming request failed: %v", err)
			// Check for specific error types
			if strings.Contains(err.Error(), "streaming timeout") {
				s.writeErrorResponse(w, StreamingTimeoutError.Type, StreamingTimeoutError.Title, err.Error())
			} else {
				// If streaming fails, try to write an error response if headers haven't been sent
				s.writeErrorResponse(w, "unknown_error", "Streaming Request Failed", err.Error())
			}
		}
		return
	}

	// Execute the standard request
	response, err := s.httpClient.ExecuteRequest(ctx, &req)
	if err != nil {
		s.logger.Printf("Request failed: %v", err)
		s.writeErrorResponse(w, "unknown_error", "Request Failed", err.Error())
		return
	}

	// Handle pass-through mode
	if req.PassThrough && response.Success {
		// Remove the application/json content-type that was set earlier
		w.Header().Del("Content-Type")

		// Set content-type header to match the proxied response
		if response.ContentType != "" {
			w.Header().Set("Content-Type", response.ContentType)
		}

		// Write raw response body directly
		if _, err := w.Write(response.RawResponseBody); err != nil {
			s.logger.Printf("Failed to write pass-through response: %v", err)
		}
		return
	}

	// Normal mode - write JSON response (Content-Type already set to application/json)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Printf("Failed to encode response: %v", err)
	}
}

// handleFormRequest handles /proxy/form endpoint
func (s *Server) handleFormRequest(w http.ResponseWriter, r *http.Request) {
	// Handle OPTIONS for CORS preflight
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Parse query parameters
	query := r.URL.Query()
	formReq := &FormProxyRequest{
		URL:         query.Get("url"),
		Method:      query.Get("method"),
		ContentType: query.Get("contentType"),
		Headers:     query.Get("headers"),
		PathParams:  query.Get("path_params"),
	}

	// Parse timeout
	if timeoutStr := query.Get("timeout"); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil {
			formReq.Timeout = timeout
		}
	}

	// Parse followRedirects
	if followRedirectsStr := query.Get("followRedirects"); followRedirectsStr != "" {
		if followRedirects, err := strconv.ParseBool(followRedirectsStr); err == nil {
			formReq.FollowRedirects = &followRedirects
		}
	}

	// Validate required fields
	if formReq.URL == "" {
		s.writeErrorResponse(w, "request_format_error", "Missing URL", "URL is required")
		return
	}

	// Check for self-loop before processing
	if s.detectLoop(r, formReq.URL) {
		s.writeLoopErrorResponse(w, "Request could create an infinite loop to this proxy server")
		return
	}

	// Default method to POST
	if formReq.Method == "" {
		formReq.Method = "POST"
	}

	// Set default timeout
	if formReq.Timeout == 0 {
		formReq.Timeout = 60
	}

	// For multipart/form-data, pass the raw body directly to preserve structure
	var formData map[string]string
	var rawBody []byte

	if strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
		// For multipart, read raw body to preserve boundaries and files
		var err error
		rawBody, err = io.ReadAll(r.Body)
		if err != nil {
			s.writeErrorResponse(w, "request_format_error", "Failed to read request body", fmt.Sprintf("Error reading body: %v", err))
			return
		}
		formReq.RawBody = rawBody
		formReq.ContentType = r.Header.Get("Content-Type") // Preserve exact content-type with boundary
	} else {
		// For URL-encoded forms, parse normally
		if err := r.ParseForm(); err != nil {
			s.writeErrorResponse(w, "request_format_error", "Invalid form data", fmt.Sprintf("Failed to parse form data: %v", err))
			return
		}

		// Convert form values to map (preserve multiple values)
		formData = make(map[string]string)
		for key, values := range r.PostForm {
			if len(values) > 0 {
				// Join multiple values with comma (standard behavior)
				formData[key] = strings.Join(values, ",")
			}
		}
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(formReq.Timeout)*time.Second)
	defer cancel()

	// Log the request
	s.logger.Printf("%s %s (form)", formReq.Method, formReq.URL)

	// Execute the request
	response, err := s.httpClient.ExecuteFormRequest(ctx, formReq, formData)
	if err != nil {
		s.logger.Printf("Form request failed: %v", err)
		s.writeErrorResponse(w, "unknown_error", "Request Failed", err.Error())
		return
	}

	// Write response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Printf("Failed to encode response: %v", err)
	}
}

// handleRoot handles the root endpoint with ASCII art
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	// Handle OPTIONS for CORS preflight
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	userAgent := r.Header.Get("User-Agent")
	useColors := strings.Contains(userAgent, "rb-slingshot")

	welcomeMsg := s.generateWelcomeMessage(useColors)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, welcomeMsg)
}

// generateWelcomeMessage creates the welcome ASCII art with optional color codes
func (s *Server) generateWelcomeMessage(useColors bool) string {
	var asciiArt string

	if useColors {
		// Colored version
		asciiArt = "\033[38;2;185;197;1m┏\033[39m\033[38;2;188;194;1m━\033[39m\033[38;2;192;190;1m┓\033[39m\033[38;2;196;186;1m╻\033[39m\033[38;2;199;182;1m \033[39m\033[38;2;203;179;1m \033[39m\033[38;2;206;175;2m╻\033[39m\033[38;2;209;171;2m┏\033[39m\033[38;2;213;167;3m┓\033[39m\033[38;2;216;163;4m╻\033[39m\033[38;2;219;159;5m┏\033[39m\033[38;2;222;154;7m━\033[39m\033[38;2;224;150;8m╸\033[39m\033[38;2;227;146;9m┏\033[39m\033[38;2;230;142;11m━\033[39m\033[38;2;232;138;13m┓\033[39m\033[38;2;234;133;15m╻\033[39m\033[38;2;237;129;17m \033[39m\033[38;2;239;125;19m╻\033[39m\033[38;2;241;121;21m┏\033[39m\033[38;2;243;116;23m━\033[39m\033[38;2;244;112;26m┓\033[39m\033[38;2;246;108;29m╺\033[39m\033[38;2;247;104;31m┳\033[39m\033[38;2;249;100;34m╸\033[39m\033[38;2;250;96;37m \033[39m\033[38;2;251;92;40m \033[39m\033[38;2;252;88;43m \033[39m\033[38;2;253;84;46m┏\033[39m\033[38;2;253;80;50m━\033[39m\033[38;2;254;76;53m┓\033[39m\033[38;2;254;72;56m┏\033[39m\033[38;2;254;68;60m━\033[39m\033[38;2;254;64;64m┓\033[39m\033[38;2;254;61;67m┏\033[39m\033[38;2;254;57;71m━\033[39m\033[38;2;254;54;75m┓\033[39m\033[38;2;253;50;79m╻\033[39m\033[38;2;253;47;83m \033[39m\033[38;2;252;44;87m╻\033[39m\033[38;2;251;41;91m╻\033[39m\033[38;2;250;38;95m \033[39m\033[38;2;249;35;99m╻\033[39m\033[38;2;248;32;103m \033[39m\033[38;2;246;29;107m \033[39m\033[38;2;245;26;111m \033[39m\033[38;2;243;24;116m┏\033[39m\033[38;2;241;22;120m━\033[39m\033[38;2;239;19;124m┓\033[39m\033[38;2;237;17;128m┏\033[39m\033[38;2;235;15;133m━\033[39m\033[38;2;233;13;137m┓\033[39m\033[38;2;230;11;141m╻\033[39m\033[38;2;228;10;145m\033[39m\n" +
			"\033[38;2;196;186;1m┗\033[39m\033[38;2;199;182;1m━\033[39m\033[38;2;203;179;1m┓\033[39m\033[38;2;206;175;2m┃\033[39m\033[38;2;209;171;2m \033[39m\033[38;2;213;167;3m \033[39m\033[38;2;216;163;4m┃\033[39m\033[38;2;219;159;5m┃\033[39m\033[38;2;222;154;7m┗\033[39m\033[38;2;224;150;8m┫\033[39m\033[38;2;227;146;9m┃\033[39m\033[38;2;230;142;11m╺\033[39m\033[38;2;232;138;13m┓\033[39m\033[38;2;234;133;15m┗\033[39m\033[38;2;237;129;17m━\033[39m\033[38;2;239;125;19m┓\033[39m\033[38;2;241;121;21m┣\033[39m\033[38;2;243;116;23m━\033[39m\033[38;2;244;112;26m┫\033[39m\033[38;2;246;108;29m┃\033[39m\033[38;2;247;104;31m \033[39m\033[38;2;249;100;34m┃\033[39m\033[38;2;250;96;37m \033[39m\033[38;2;251;92;40m┃\033[39m\033[38;2;252;88;43m \033[39m\033[38;2;253;84;46m \033[39m\033[38;2;253;80;50m \033[39m\033[38;2;254;76;53m \033[39m\033[38;2;254;72;56m┣\033[39m\033[38;2;254;68;60m━\033[39m\033[38;2;254;64;64m┛\033[39m\033[38;2;254;61;67m┣\033[39m\033[38;2;254;57;71m┳\033[39m\033[38;2;254;54;75m┛\033[39m\033[38;2;253;50;79m┃\033[39m\033[38;2;253;47;83m \033[39m\033[38;2;252;44;87m┃\033[39m\033[38;2;251;41;91m┏\033[39m\033[38;2;250;38;95m╋\033[39m\033[38;2;249;35;99m┛\033[39m\033[38;2;248;32;103m┗\033[39m\033[38;2;246;29;107m┳\033[39m\033[38;2;245;26;111m┛\033[39m\033[38;2;243;24;116m \033[39m\033[38;2;241;22;120m \033[39m\033[38;2;239;19;124m \033[39m\033[38;2;237;17;128m┣\033[39m\033[38;2;235;15;133m━\033[39m\033[38;2;233;13;137m┫\033[39m\033[38;2;230;11;141m┣\033[39m\033[38;2;228;10;145m━\033[39m\033[38;2;225;8;149m┛\033[39m\033[38;2;222;7;154m┃\033[39m\033[38;2;219;6;158m\033[39m\n" +
			"\033[38;2;206;175;2m┗\033[39m\033[38;2;209;171;2m━\033[39m\033[38;2;213;167;3m┛\033[39m\033[38;2;216;163;4m┗\033[39m\033[38;2;219;159;5m━\033[39m\033[38;2;222;154;7m╸\033[39m\033[38;2;224;150;8m╹\033[39m\033[38;2;227;146;9m╹\033[39m\033[38;2;230;142;11m \033[39m\033[38;2;232;138;13m╹\033[39m\033[38;2;234;133;15m┗\033[39m\033[38;2;237;129;17m━\033[39m\033[38;2;239;125;19m┛\033[39m\033[38;2;241;121;21m┗\033[39m\033[38;2;243;116;23m━\033[39m\033[38;2;244;112;26m┛\033[39m\033[38;2;246;108;29m╹\033[39m\033[38;2;247;104;31m \033[39m\033[38;2;249;100;34m╹\033[39m\033[38;2;250;96;37m┗\033[39m\033[38;2;251;92;40m━\033[39m\033[38;2;252;88;43m┛\033[39m\033[38;2;253;84;46m \033[39m\033[38;2;253;80;50m╹\033[39m\033[38;2;254;76;53m \033[39m\033[38;2;254;72;56m \033[39m\033[38;2;254;68;60m \033[39m\033[38;2;254;64;64m \033[39m\033[38;2;254;61;67m╹\033[39m\033[38;2;254;57;71m \033[39m\033[38;2;254;54;75m \033[39m\033[38;2;253;50;79m╹\033[39m\033[38;2;253;47;83m┗\033[39m\033[38;2;252;44;87m╸\033[39m\033[38;2;251;41;91m┗\033[39m\033[38;2;250;38;95m━\033[39m\033[38;2;249;35;99m┛\033[39m\033[38;2;248;32;103m╹\033[39m\033[38;2;246;29;107m \033[39m\033[38;2;245;26;111m╹\033[39m\033[38;2;243;24;116m \033[39m\033[38;2;241;22;120m╹\033[39m\033[38;2;239;19;124m \033[39m\033[38;2;237;17;128m \033[39m\033[38;2;235;15;133m \033[39m\033[38;2;233;13;137m \033[39m\033[38;2;230;11;141m╹\033[39m\033[38;2;228;10;145m \033[39m\033[38;2;225;8;149m╹\033[39m\033[38;2;222;7;154m╹\033[39m\033[38;2;219;6;158m \033[39m\033[38;2;216;4;162m \033[39m\033[38;2;213;3;166m╹\033[39m\033[38;2;210;3;170m\033[39m\n" +
			"======================================================"
	} else {
		// Black and white version
		asciiArt = `┏━┓╻  ╻┏┓╻┏━╸┏━┓╻ ╻┏━┓╺┳╸   ┏━┓┏━┓┏━┓╻ ╻╻ ╻   ┏━┓┏━┓╻
┗━┓┃  ┃┃┗┫┃╺┓┗━┓┣━┫┃ ┃ ┃    ┣━┛┣┳┛┃ ┃┏╋┛┗┳┛   ┣━┫┣━┛┃
┗━┛┗━╸╹╹ ╹┗━┛┗━┛╹ ╹┗━┛ ╹    ╹  ╹┗╸┗━┛╹ ╹ ╹    ╹ ╹╹  ╹
======================================================`
	}

	desc := "The Slingshot Proxy is a powerful HTTP proxy server that enables you to\n" +
		"make HTTP requests through a proxy, bypassing CORS restrictions and providing\n" +
		"advanced features like streaming, form data handling, and local file serving.\n\n" +
		"Learn more about the project at:\n" +
		" - https://github.com/requestbite/slingshot-proxy\n\n" +
		"Endpoints:\n" +
		" - POST /proxy/request - Make HTTP requests via JSON\n" +
		" - POST /proxy/form    - Make HTTP requests via form data\n" +
		" - GET  /health        - Health check endpoint"

	if s.enableLocalFiles {
		desc += "\n - POST /file          - Serve local files (enabled)\n" +
			" - POST /dir           - List directory contents (enabled)"
	}

	return fmt.Sprintf("Welcome to version %s of:\n\n%s\n\n%s\n", s.version, asciiArt, desc)
}

// handleHealthCheck handles the health check endpoint
func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	// Handle OPTIONS for CORS preflight
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	healthResponse := map[string]interface{}{
		"status":     "ok",
		"version":    s.version,
		"user-agent": fmt.Sprintf("rb-slingshot/%s (https://requestbite.com/slingshot)", s.version),
	}

	json.NewEncoder(w).Encode(healthResponse)
}

// handleNotFound handles requests to undefined endpoints
func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := &ProxyResponse{
		Success:      false,
		ErrorType:    EndpointNotFoundError.Type,
		ErrorTitle:   EndpointNotFoundError.Title,
		ErrorMessage: fmt.Sprintf("Endpoint not found: %s", r.URL.Path),
		Cancelled:    false,
	}

	w.WriteHeader(http.StatusNotFound) // HTTP 404 status
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Printf("Failed to encode not found response: %v", err)
	}
}

// corsMiddleware adds CORS headers
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		w.Header().Set("Access-Control-Expose-Headers", "X-Slingshot-Streaming")
		w.Header().Set("Access-Control-Max-Age", "86400")

		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs incoming requests
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer wrapper to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		// Log the request
		s.logger.Printf("%s %s %d %v", r.Method, r.URL.Path, wrapped.statusCode, time.Since(start))
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// Flush implements http.Flusher interface for streaming support
func (w *responseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// writeErrorResponse writes a standardized error response
func (s *Server) writeErrorResponse(w http.ResponseWriter, errorType, errorTitle, errorMessage string) {
	response := &ProxyResponse{
		Success:      false,
		ErrorType:    errorType,
		ErrorTitle:   errorTitle,
		ErrorMessage: errorMessage,
		Cancelled:    false,
	}

	w.WriteHeader(http.StatusOK) // Keep 200 status for API consistency
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Printf("Failed to encode error response: %v", err)
	}
}

// writeLoopErrorResponse writes an error response for loop detection with HTTP 508 status
func (s *Server) writeLoopErrorResponse(w http.ResponseWriter, errorMessage string) {
	response := &ProxyResponse{
		Success:      false,
		ErrorType:    LoopDetectedError.Type,
		ErrorTitle:   LoopDetectedError.Title,
		ErrorMessage: errorMessage,
		Cancelled:    false,
	}

	w.WriteHeader(http.StatusLoopDetected) // HTTP 508 status for loops
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Printf("Failed to encode loop error response: %v", err)
	}
}

// handleFileRequest handles /file endpoint for local file serving
func (s *Server) handleFileRequest(w http.ResponseWriter, r *http.Request) {
	// Handle OPTIONS for CORS preflight
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Check if feature is enabled
	if !s.enableLocalFiles {
		w.WriteHeader(http.StatusNotFound)
		s.logger.Printf("File endpoint accessed but feature is disabled")
		return
	}

	// Parse request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		s.writeErrorResponse(w, "request_format_error", "Failed to read request body", err.Error())
		return
	}

	var req FileRequest
	if err := json.Unmarshal(body, &req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		s.writeErrorResponse(w, "request_format_error", "Invalid JSON", fmt.Sprintf("Failed to parse JSON request: %v", err))
		return
	}

	// Validate required fields
	if req.Path == "" {
		w.Header().Set("Content-Type", "application/json")
		s.writeErrorResponse(w, "request_format_error", "Missing path", "File path is required")
		return
	}

	// Clean and validate the path
	cleanPath := filepath.Clean(req.Path)

	// Security check: Ensure path is absolute
	if !filepath.IsAbs(cleanPath) {
		w.Header().Set("Content-Type", "application/json")
		s.writeErrorResponse(w, FileAccessError.Type, FileAccessError.Title, "Path must be absolute")
		return
	}

	s.logger.Printf("File request: %s", cleanPath)

	// Check if file exists and is accessible
	fileInfo, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			s.writeErrorResponse(w, FileNotFoundError.Type, FileNotFoundError.Title, fmt.Sprintf("File not found: %s", cleanPath))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		s.writeErrorResponse(w, FileAccessError.Type, FileAccessError.Title, fmt.Sprintf("Cannot access file: %v", err))
		return
	}

	// Check if it's a directory
	if fileInfo.IsDir() {
		w.Header().Set("Content-Type", "application/json")
		s.writeErrorResponse(w, FileAccessError.Type, FileAccessError.Title, "Path is a directory, not a file")
		return
	}

	// Read the file
	fileData, err := os.ReadFile(cleanPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		s.writeErrorResponse(w, FileAccessError.Type, FileAccessError.Title, fmt.Sprintf("Failed to read file: %v", err))
		return
	}

	// Detect MIME type
	mimeType := s.detectMimeType(cleanPath, fileData)

	// Set the appropriate Content-Type header
	w.Header().Set("Content-Type", mimeType)

	// Write the file content directly (pass-through mode)
	if _, err := w.Write(fileData); err != nil {
		s.logger.Printf("Failed to write file response: %v", err)
	}

	s.logger.Printf("Served file: %s (%d bytes, %s)", cleanPath, len(fileData), mimeType)
}

// detectMimeType detects the MIME type of a file based on extension and content
func (s *Server) detectMimeType(filePath string, data []byte) string {
	// First try to detect by file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	mimeType := mime.TypeByExtension(ext)

	if mimeType != "" {
		return mimeType
	}

	// If extension-based detection fails, use content-based detection
	mimeType = http.DetectContentType(data)

	// Return the detected MIME type, or default to application/octet-stream
	if mimeType == "" {
		return "application/octet-stream"
	}

	return mimeType
}

// getDefaultRoot returns the user's home directory, or falls back to platform root
func (s *Server) getDefaultRoot() string {
	// Try to get the current user's home directory
	if currentUser, err := user.Current(); err == nil {
		if currentUser.HomeDir != "" {
			return currentUser.HomeDir
		}
	}

	// Fall back to platform-specific root directory
	if runtime.GOOS == "windows" {
		return "C:\\"
	}
	return "/"
}

// getParentDirectory returns the parent directory path, or nil if at root
func (s *Server) getParentDirectory(currentPath string) *string {
	// Clean the path first
	cleanPath := filepath.Clean(currentPath)

	// Get the parent directory
	parentPath := filepath.Dir(cleanPath)

	// Check if we're at the root (parent == current means we can't go up)
	// This handles Unix root "/" and Windows drive roots like "C:\"
	if parentPath == cleanPath {
		return nil
	}

	return &parentPath
}

// sortDirectoryEntries sorts directory entries (directories first, then alphabetically)
func sortDirectoryEntries(entries []DirectoryEntry) {
	sort.Slice(entries, func(i, j int) bool {
		// Directories come before files
		if entries[i].Type != entries[j].Type {
			return entries[i].Type == "directory"
		}
		// Within same type, sort alphabetically (case-insensitive)
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
}

// handleDirectoryRequest handles /dir endpoint for directory listing
func (s *Server) handleDirectoryRequest(w http.ResponseWriter, r *http.Request) {
	// Handle OPTIONS for CORS preflight
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Check if feature is enabled
	if !s.enableLocalFiles {
		w.WriteHeader(http.StatusNotFound)
		s.logger.Printf("Directory endpoint accessed but feature is disabled")
		return
	}

	// Parse request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		s.writeErrorResponse(w, "request_format_error", "Failed to read request body", err.Error())
		return
	}

	var req DirectoryRequest
	if err := json.Unmarshal(body, &req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		s.writeErrorResponse(w, "request_format_error", "Invalid JSON", fmt.Sprintf("Failed to parse JSON request: %v", err))
		return
	}

	// Determine whether to show hidden files (defaults to false)
	showHidden := false
	if req.ShowHiddenFiles != nil {
		showHidden = *req.ShowHiddenFiles
	}

	// Determine target path
	var targetPath string
	if req.Path == nil {
		// Use platform-specific root
		targetPath = s.getDefaultRoot()
	} else {
		targetPath = *req.Path
	}

	// Clean the path
	cleanPath := filepath.Clean(targetPath)

	// Security check: Ensure path is absolute (unless it's the root)
	if !filepath.IsAbs(cleanPath) {
		w.Header().Set("Content-Type", "application/json")
		s.writeErrorResponse(w, FileAccessError.Type, FileAccessError.Title, "Path must be absolute")
		return
	}

	s.logger.Printf("Directory request: %s", cleanPath)

	// Check if path exists
	fileInfo, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			s.writeErrorResponse(w, FileNotFoundError.Type, FileNotFoundError.Title, fmt.Sprintf("Directory not found: %s", cleanPath))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		s.writeErrorResponse(w, FileAccessError.Type, FileAccessError.Title, fmt.Sprintf("Cannot access path: %v", err))
		return
	}

	// Check if it's a directory
	if !fileInfo.IsDir() {
		w.Header().Set("Content-Type", "application/json")
		s.writeErrorResponse(w, FileAccessError.Type, FileAccessError.Title, "Path is a file, not a directory")
		return
	}

	// Read directory contents
	entries, err := os.ReadDir(cleanPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		s.writeErrorResponse(w, FileAccessError.Type, FileAccessError.Title, fmt.Sprintf("Failed to read directory: %v", err))
		return
	}

	// Build response array
	var dirEntries []DirectoryEntry
	for _, entry := range entries {
		entryName := entry.Name()

		// Filter hidden files if showHidden is false
		if !showHidden && strings.HasPrefix(entryName, ".") {
			continue
		}

		// Build full path for this entry
		entryPath := filepath.Join(cleanPath, entryName)

		// Use Lstat to detect symlinks (doesn't follow them)
		lstatInfo, err := os.Lstat(entryPath)
		if err != nil {
			// Log but skip entries we can't access
			s.logger.Printf("Warning: Cannot lstat entry %s: %v", entryPath, err)
			continue
		}

		var dirEntry DirectoryEntry
		dirEntry.Name = entryName

		// Check if it's a symlink
		if lstatInfo.Mode()&os.ModeSymlink != 0 {
			// It's a symlink - follow it to determine target type
			statInfo, err := os.Stat(entryPath)
			if err != nil {
				// Broken symlink or permission denied
				// Default to "file" type and mark as symlink
				s.logger.Printf("Warning: Cannot follow symlink %s: %v", entryPath, err)
				dirEntry.Type = "file"
				isSymlink := true
				dirEntry.IsSymlink = &isSymlink
			} else {
				// Successfully followed symlink
				if statInfo.IsDir() {
					dirEntry.Type = "directory"
				} else {
					dirEntry.Type = "file"
				}
				isSymlink := true
				dirEntry.IsSymlink = &isSymlink
			}
		} else {
			// Not a symlink - use standard type detection
			if lstatInfo.IsDir() {
				dirEntry.Type = "directory"
			} else {
				dirEntry.Type = "file"
			}
			// Don't set IsSymlink field for non-symlinks (omitempty will exclude it)
		}

		dirEntries = append(dirEntries, dirEntry)
	}

	// Sort entries (directories first, then alphabetically)
	sortDirectoryEntries(dirEntries)

	// Get parent directory
	parentDir := s.getParentDirectory(cleanPath)

	// Build response object
	response := DirectoryResponse{
		ParentDir: parentDir,
		Dir:       dirEntries,
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Printf("Failed to encode directory response: %v", err)
	}

	s.logger.Printf("Listed directory: %s (%d entries)", cleanPath, len(dirEntries))
}
