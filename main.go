package main

import (
	"fmt"
	"net"
	"os"
	"strings"
)

func HttpGet(url string) error {
	host, port, path := parseURL(url)
	conn, err := net.Dial("tcp", host+":"+port)
	if err != nil {
		return err
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
		os.Stdout.Write(buf[:n])
	}
	return nil
}

func parseURL(url string) (string, string, string) {
	parts := strings.Split(url, "/")
	hostPart := parts[2]
	hostParts := strings.Split(hostPart, ":")
	host := hostParts[0]
	var port string
	if len(hostParts) > 1 {
		port = hostParts[1]
	} else {
		port = "80"
	}
	path := "/" + strings.Join(parts[3:], "/")
	return host, port, path
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: go run tinyhttp.go <url>")
		return
	}
	url := os.Args[1]
	HttpGet(url)
}
