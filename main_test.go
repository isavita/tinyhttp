package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestHttpGetLargeResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		largeText := strings.Repeat("a", 4096) + "b"
		fmt.Fprintln(w, largeText)
	})

	server := &http.Server{
		Addr:    "127.0.0.1:8080",
		Handler: mux,
	}
	go server.ListenAndServe()
	defer server.Close()

	// Redirect stdout to a buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := HttpGet("http://127.0.0.1:8080/")
	if err != nil {
		t.Fatalf("HttpGet failed: %v", err)
	}

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	var buf strings.Builder
	io.Copy(&buf, r)
	response := buf.String()
	// Trim the response to remove chunked encoding headers and whitespace
	response = strings.Trim(response, " \t\r\n0")

	if len(response) <= 4096 {
		t.Fatalf("Expected response length to be greater than 4096, got %d", len(response))
	}

	if !strings.HasSuffix(response, "b") {
		t.Fatalf("Expected response to end with 'b\\r\\n', got '%s'", response[len(response)-3:])
	}
}
