package main

import (
	"fmt"
	"net"
	"os"
	"strings"
)

func HttpGet(url string) {
	host, path := parseURL(url)
	conn, err := net.Dial("tcp", host+":80")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer conn.Close()

	requestLine := fmt.Sprintf("GET %s HTTP/1.1\r\n", path)
	headers := fmt.Sprintf("Host: %s\r\n", host)
	headers += "Connection: close\r\n\r\n"

	request := requestLine + headers
	conn.Write([]byte(request))

	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			break
		}
		fmt.Print(string(buf[:n]))
	}
}

func parseURL(url string) (string, string) {
	// Assuming the URL is well-formed and has a scheme followed by the host
	parts := strings.Split(url, "/")
	host := parts[2]
	path := "/" + strings.Join(parts[3:], "/")
	return host, path
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: go run tinyhttp.go <url>")
		return
	}
	url := os.Args[1]
	HttpGet(url)
}
