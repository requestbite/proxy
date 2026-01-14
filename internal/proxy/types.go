package proxy

import (
	"fmt"
	"time"
)

// ProxyRequest represents the JSON request structure matching the Lua API
type ProxyRequest struct {
	Method          string            `json:"method"`
	URL             string            `json:"url"`
	Headers         []string          `json:"headers"`
	Body            string            `json:"body,omitempty"`
	Timeout         int               `json:"timeout,omitempty"`
	FollowRedirects *bool             `json:"followRedirects,omitempty"`
	PathParams      map[string]string `json:"path_params,omitempty"`
	PassThrough     bool              `json:"passThrough,omitempty"`
	Streaming       bool              `json:"streaming,omitempty"`
}

// FormProxyRequest represents form data request parameters
type FormProxyRequest struct {
	URL             string `json:"url"`
	Method          string `json:"method"`
	Timeout         int    `json:"timeout,omitempty"`
	FollowRedirects *bool  `json:"followRedirects,omitempty"`
	ContentType     string `json:"contentType,omitempty"`
	Headers         string `json:"headers,omitempty"`
	PathParams      string `json:"path_params,omitempty"`
	RawBody         []byte `json:"-"` // For multipart data, exclude from JSON
}

// FileRequest represents a local file request
type FileRequest struct {
	Path string `json:"path"`
}

// DirectoryRequest represents a directory listing request
type DirectoryRequest struct {
	Path            *string `json:"path"`            // Pointer to allow null detection
	ShowHiddenFiles *bool   `json:"showHiddenFiles"` // Defaults to false if not provided
}

// DirectoryEntry represents a file or directory entry
type DirectoryEntry struct {
	Name      string `json:"name"`
	Type      string `json:"type"`                // "file" or "directory"
	IsSymlink *bool  `json:"isSymlink,omitempty"` // Only present if entry is a symlink
	SizeBytes *int64 `json:"sizeBytes,omitempty"` // File size in bytes (only for files)
	SizeHuman *string `json:"sizeHuman,omitempty"` // Human-readable size (only for files)
}

// DirectoryResponse represents the response for directory listing
type DirectoryResponse struct {
	ParentDir  *string          `json:"parentDir"`  // Absolute path to parent directory, or null if at root
	CurrentDir string           `json:"currentDir"` // Absolute path to the currently listed directory
	Dir        []DirectoryEntry `json:"dir"`        // Array of directory entries
}

// ExecRequest represents a process execution request
type ExecRequest struct {
	Command       string            `json:"command"`              // Required
	Args          []string          `json:"args,omitempty"`       // Optional
	Timeout       int               `json:"timeout,omitempty"`    // Optional, default 10s, max 20s
	WorkingDir    string            `json:"workingDir,omitempty"` // Optional
	Env           map[string]string `json:"env,omitempty"`        // Optional
	CombineOutput bool              `json:"combineOutput,omitempty"` // Optional, default false
}

// ExecResponse represents the response from process execution
type ExecResponse struct {
	Success        bool   `json:"success"`
	ExitCode       int    `json:"exitCode,omitempty"`
	Stdout         string `json:"stdout,omitempty"`        // Only if not combined
	Stderr         string `json:"stderr,omitempty"`        // Only if not combined
	CombinedOutput string `json:"combinedOutput,omitempty"` // Only if combined
	ExecutionTime  string `json:"executionTime,omitempty"`

	// Error fields (when success = false)
	ErrorType    string `json:"errorType,omitempty"`
	ErrorTitle   string `json:"errorTitle,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// ProxyResponse represents the response structure matching the Lua API
type ProxyResponse struct {
	Success         bool              `json:"success"`
	ResponseStatus  int               `json:"response_status,omitempty"`
	ResponseHeaders map[string]string `json:"response_headers,omitempty"`
	ResponseData    string            `json:"response_data,omitempty"`
	ResponseSize    string            `json:"response_size,omitempty"`
	ResponseTime    string            `json:"response_time,omitempty"`
	ContentType     string            `json:"content_type,omitempty"`
	IsBinary        bool              `json:"is_binary,omitempty"`
	Cancelled       bool              `json:"cancelled,omitempty"`

	// Error fields (when success = false)
	ErrorType    string `json:"error_type,omitempty"`
	ErrorTitle   string `json:"error_title,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`

	// Internal fields for pass-through mode
	RawResponseBody []byte `json:"-"`
	PassThrough     bool   `json:"-"`
}

// StreamingResponse represents the initial metadata response for streaming requests
// This excludes response_data, response_size, and response_time which are not available during streaming
type StreamingResponse struct {
	Success         bool              `json:"success"`
	ResponseStatus  int               `json:"response_status,omitempty"`
	ResponseHeaders map[string]string `json:"response_headers,omitempty"`
	ContentType     string            `json:"content_type,omitempty"`
	IsBinary        bool              `json:"is_binary,omitempty"`
	Cancelled       bool              `json:"cancelled,omitempty"`

	// Error fields (when success = false)
	ErrorType    string `json:"error_type,omitempty"`
	ErrorTitle   string `json:"error_title,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// ProxyError represents different types of proxy errors
type ProxyError struct {
	Type    string
	Title   string
	Message string
}

func (e *ProxyError) Error() string {
	return e.Message
}

// Predefined error types matching Lua implementation
var (
	URLValidationError = &ProxyError{
		Type:  "url_validation_error",
		Title: "Invalid URL",
	}
	TimeoutError = &ProxyError{
		Type:  "timeout",
		Title: "Request Timed Out",
	}
	ConnectionError = &ProxyError{
		Type:  "connection_error",
		Title: "Connection Failed",
	}
	RedirectNotFollowedError = &ProxyError{
		Type:  "redirect_not_followed",
		Title: "Redirect Not Followed",
	}
	LoopDetectedError = &ProxyError{
		Type:  "loop_detected",
		Title: "Loop Detected",
	}
	StreamingTimeoutError = &ProxyError{
		Type:  "request_timeout",
		Title: "Streaming Request Timeout",
	}
	FileNotFoundError = &ProxyError{
		Type:  "file_not_found",
		Title: "File Not Found",
	}
	FileAccessError = &ProxyError{
		Type:  "file_access_error",
		Title: "File Access Error",
	}
	FeatureDisabledError = &ProxyError{
		Type:  "feature_disabled",
		Title: "Feature Disabled",
	}
	EndpointNotFoundError = &ProxyError{
		Type:  "endpoint_not_found",
		Title: "Endpoint Not Found",
	}
	ExecTimeoutError = &ProxyError{
		Type:  "exec_timeout",
		Title: "Execution Timeout",
	}
	ExecFailedError = &ProxyError{
		Type:  "exec_failed",
		Title: "Execution Failed",
	}
	LocalhostOnlyError = &ProxyError{
		Type:  "localhost_only",
		Title: "Localhost Only",
	}
)

// RequestMetrics holds timing and size information
type RequestMetrics struct {
	StartTime    time.Time
	EndTime      time.Time
	ResponseSize int64
}

// GetDuration returns the total request duration in milliseconds
func (m *RequestMetrics) GetDuration() float64 {
	return float64(m.EndTime.Sub(m.StartTime).Nanoseconds()) / 1000000
}

// FormatDuration returns formatted duration string
func (m *RequestMetrics) FormatDuration() string {
	return fmt.Sprintf("%.2f ms", m.GetDuration())
}

// FormatSize returns formatted size string
func (m *RequestMetrics) FormatSize() string {
	size := m.ResponseSize
	if size >= 1024*1024 {
		return fmt.Sprintf("%.2f MB", float64(size)/(1024*1024))
	} else if size >= 1024 {
		return fmt.Sprintf("%.2f KB", float64(size)/1024)
	}
	return fmt.Sprintf("%d B", size)
}

// FormatFileSize returns a human-readable file size string
// Formats as kb, MB, or GB (rounded to nearest whole number)
func FormatFileSize(bytes int64) string {
	const (
		kb = 1024
		mb = 1024 * 1024
		gb = 1024 * 1024 * 1024
	)

	switch {
	case bytes >= gb:
		return fmt.Sprintf("%d GB", (bytes+gb/2)/gb) // Round to nearest GB
	case bytes >= mb:
		return fmt.Sprintf("%d MB", (bytes+mb/2)/mb) // Round to nearest MB
	case bytes >= kb:
		return fmt.Sprintf("%d kb", (bytes+kb/2)/kb) // Round to nearest kb
	default:
		return fmt.Sprintf("%d kb", 0) // Less than 1kb rounds to 0 kb
	}
}
