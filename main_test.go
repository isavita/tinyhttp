package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
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

func runHttpGetAndCaptureOutput(t *testing.T, r *os.File, w *os.File, url string) string {
	err := HttpGet(url)
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

	response := runHttpGetAndCaptureOutput(t, r, w, "http://127.0.0.1:8080/")
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

	r, w, cleanup := runTestServer(t, mux, "127.0.0.1:8081") // Different port
	defer cleanup()

	response := runHttpGetAndCaptureOutput(t, r, w, "http://127.0.0.1:8081/") // Different port
	expectedResponse := "Hello, world!"
	if response != expectedResponse {
		t.Fatalf("Expected response to be %q, got %q", expectedResponse, response)
	}
}
