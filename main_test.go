package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func runTestServer(t *testing.T, mux *http.ServeMux, addr string) (*os.File, *os.File, func()) {
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	go server.ListenAndServe()
	time.Sleep(5 * time.Millisecond)

	// Redirect stdout to a buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cleanup := func() {
		w.Close()
		os.Stdout = oldStdout
		server.Close()
	}

	return r, w, cleanup
}

func runHttpGetAndCaptureOutput(t *testing.T, r *os.File, w *os.File, url string, output io.Writer, flags *HttpFlags) string {
	err := HttpGet(url, output, flags)
	if err != nil {
		t.Fatalf("HttpGet failed: %v", err)
	}
	w.Close() // Close the write end of the pipe

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestHttpGetNonChunkedResponse(t *testing.T) {
	content := "Hello, world!"
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(content)))
		fmt.Fprint(w, content)
	})

	r, w, cleanup := runTestServer(t, mux, "127.0.0.1:8080")
	defer cleanup()

	flags := &HttpFlags{
		ShowHeaders:     false,
		ShowOnlyHeaders: false,
	}
	response := runHttpGetAndCaptureOutput(t, r, w, "http://127.0.0.1:8080/", os.Stdout, flags)
	if response != content {
		t.Fatalf("Expected response to end with %q, got %q", content, response)
	}
}

func TestHttpGetChunkedResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		chunks := []string{"Hello", ", ", "world", "!"}
		for _, chunk := range chunks {
			fmt.Fprint(w, chunk)
		}
	})

	r, w, cleanup := runTestServer(t, mux, "127.0.0.1:8080") // Different port
	defer cleanup()

	flags := &HttpFlags{
		ShowHeaders:     false,
		ShowOnlyHeaders: false,
	}
	response := runHttpGetAndCaptureOutput(t, r, w, "http://127.0.0.1:8080/", os.Stdout, flags) // Different port
	expectedResponse := "Hello, world!"
	if response != expectedResponse {
		t.Fatalf("Expected response to be %q, got %q", expectedResponse, response)
	}
}

func TestHttpGetWithHeaders(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Hello, world!")
	})

	r, w, cleanup := runTestServer(t, mux, "127.0.0.1:8080")
	defer cleanup()

	flags := &HttpFlags{
		ShowHeaders:     true,
		ShowOnlyHeaders: false,
	}
	response := runHttpGetAndCaptureOutput(t, r, w, "http://127.0.0.1:8080/", os.Stdout, flags)
	if !strings.Contains(response, "HTTP/1.1") {
		t.Fatalf("Expected response to contain headers, got %q", response)
	}
	if !strings.Contains(response, "Hello, world!") {
		t.Fatalf("Expected response to contain body, got %q", response)
	}
}

func TestHttpGetHeadersOnly(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Hello, world!")
	})

	r, w, cleanup := runTestServer(t, mux, "127.0.0.1:8080")
	defer cleanup()

	flags := &HttpFlags{
		ShowHeaders:     false,
		ShowOnlyHeaders: true,
	}
	response := runHttpGetAndCaptureOutput(t, r, w, "http://127.0.0.1:8080/", os.Stdout, flags)
	if !strings.Contains(response, "HTTP/1.1") || strings.Contains(response, "Hello, world!") {
		t.Fatalf("Expected response to contain headers only, got %q", response)
	}
}

func TestHttpGetAcceptHeader(t *testing.T) {
	mux := http.NewServeMux()

	// Handle requests to the root path
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Check the Accept header to determine the response format
		if r.Header.Get("Accept") == "text/plain" {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, "Hello, world!")
		} else if r.Header.Get("Accept") == "application/xml" {
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, "<greeting>Hello, world!</greeting>")
		} else {
			fmt.Println(r.Header.Get("Accept"))
		}
	})

	// Start the test server
	r, w, cleanup := runTestServer(t, mux, "127.0.0.1:8080")
	defer cleanup()

	// Run the client with the Accept header set to text/plain
	flags := &HttpFlags{
		CustomHeaders: []string{"Accept: text/plain"},
	}
	response := runHttpGetAndCaptureOutput(t, r, w, "http://127.0.0.1:8080/", os.Stdout, flags)

	fmt.Println(response)
	// Verify the text/plain response
	expectedResponse := "Hello, world!"
	if response != expectedResponse {
		t.Fatalf("Expected response to be %q for text/plain, got %q", expectedResponse, response)
	}

	// Reset the pipes to capture stdout again
	r, w, _ = os.Pipe()
	os.Stdout = w

	// Run the client with the Accept header set to application/xml
	flags = &HttpFlags{
		CustomHeaders: []string{"Accept: application/xml"},
	}
	response = runHttpGetAndCaptureOutput(t, r, w, "http://127.0.0.1:8080/", os.Stdout, flags)

	// Verify the application/xml response
	expectedResponse = "<greeting>Hello, world!</greeting>"
	if response != expectedResponse {
		t.Fatalf("Expected response to be %q for application/xml, got %q", expectedResponse, response)
	}
}

func TestHttpGetWithMultipleHeaders(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Simple authentication check
		authHeader := r.Header.Get("Authorization")
		customAuthHeader := r.Header.Get("Custom-Auth")
		if authHeader == "Basic XXXXX" && customAuthHeader == "SecretToken" {
			fmt.Fprint(w, "Authentication successful!")
		} else {
			http.Error(w, "Forbidden", http.StatusForbidden)
		}
	})

	r, w, cleanup := runTestServer(t, mux, "127.0.0.1:8080")
	defer cleanup()

	// Set up the custom headers
	flags := &HttpFlags{
		CustomHeaders: []string{
			"Authorization: Basic XXXXX",
			"Custom-Auth: SecretToken",
		},
	}

	response := runHttpGetAndCaptureOutput(t, r, w, "http://127.0.0.1:8080/", os.Stdout, flags)
	expectedResponse := "Authentication successful!"
	if response != expectedResponse {
		t.Fatalf("Expected response to be %q, got %q", expectedResponse, response)
	}
}

func TestHttpGetWithOutputToFile(t *testing.T) {
	content := "Hello, file output!"
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, content)
	})

	// Start the test server
	r, w, cleanup := runTestServer(t, mux, "127.0.0.1:8080")
	defer cleanup()

	// Generate a unique file name using a timestamp
	timeStamp := time.Now().UnixNano()
	fileName := fmt.Sprintf("tmp_output_%d.txt", timeStamp)
	defer os.Remove(fileName)
	file, err := CreateOutputFile(fileName)
	if err != nil {
		t.Fatalf("Failed to create output file: %v", err)
	}
	flags := &HttpFlags{
		OutputFile: fileName,
	}

	response := runHttpGetAndCaptureOutput(t, r, w, "http://127.0.0.1:8080/", file, flags)

	// Check the response is empty
	if response != "" {
		t.Fatalf("Expected response to be empty, got %q", response)
	}
	// Check the contents of the output file
	data, err := os.ReadFile(fileName)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}
	if string(data) != content {
		t.Fatalf("Expected output file to contain %q, got %q", content, string(data))
	}
}

func TestHttpGetHeadersOnlyOutputToFile(t *testing.T) {
	content := "Hello, world!"
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, content)
	})

	// Start the test server
	r, w, cleanup := runTestServer(t, mux, "127.0.0.1:8084")
	defer cleanup()

	// Generate a unique file name using a timestamp
	timeStamp := time.Now().UnixNano()
	fileName := fmt.Sprintf("tmp_output_%d.txt", timeStamp)
	defer os.Remove(fileName)
	file, err := CreateOutputFile(fileName)
	if err != nil {
		t.Fatalf("Failed to create output file: %v", err)
	}

	flags := &HttpFlags{
		ShowOnlyHeaders: true,
		OutputFile:      fileName,
	}

	response := runHttpGetAndCaptureOutput(t, r, w, "http://127.0.0.1:8084/", file, flags) // Execute the HTTP GET request

	// Check that the response is empty
	if response != "" {
		t.Fatalf("Expected response to be empty, got %q", response)
	}

	// Check the contents of the output file
	data, err := os.ReadFile(fileName)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}
	if !strings.Contains(string(data), "HTTP/1.1") || strings.Contains(string(data), content) {
		t.Fatalf("Expected output file to contain headers only, got %q", string(data))
	}
}
