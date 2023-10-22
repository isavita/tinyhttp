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
)

func runTestServer(t *testing.T, mux *http.ServeMux, addr string) (*os.File, *os.File, func()) {
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	go server.ListenAndServe()

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

func runHttpGetAndCaptureOutput(t *testing.T, r *os.File, w *os.File, url string, flags *HttpFlags) string {
	err := HttpGet(url, flags)
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
	response := runHttpGetAndCaptureOutput(t, r, w, "http://127.0.0.1:8080/", flags)
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
	response := runHttpGetAndCaptureOutput(t, r, w, "http://127.0.0.1:8080/", flags) // Different port
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
	response := runHttpGetAndCaptureOutput(t, r, w, "http://127.0.0.1:8080/", flags)
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
	response := runHttpGetAndCaptureOutput(t, r, w, "http://127.0.0.1:8080/", flags)
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
	r, w, cleanup := runTestServer(t, mux, "127.0.0.1:8084")
	defer cleanup()

	// Run the client with the Accept header set to text/plain
	flags := &HttpFlags{
		CustomHeaders: []string{"Accept: text/plain"},
	}
	response := runHttpGetAndCaptureOutput(t, r, w, "http://127.0.0.1:8084/", flags)

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
	response = runHttpGetAndCaptureOutput(t, r, w, "http://127.0.0.1:8084/", flags)

	// Verify the application/xml response
	expectedResponse = "<greeting>Hello, world!</greeting>"
	if response != expectedResponse {
		t.Fatalf("Expected response to be %q for application/xml, got %q", expectedResponse, response)
	}
}
