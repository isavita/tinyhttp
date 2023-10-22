package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/pflag"
)

type HttpFlags struct {
	ShowHeaders     bool
	ShowOnlyHeaders bool
	CustomHeaders   []string
	OutputFile      string
}

func parseFlags() *HttpFlags {
	showHeaders := pflag.Bool("i", false, "Show response headers")
	showOnlyHeaders := pflag.Bool("I", false, "Show only response headers")
	var customHeaders []string
	pflag.StringSliceVar(&customHeaders, "H", nil, "Custom headers to include in the request")
	outputFile := pflag.String("o", "", "Output to file instead of stdout")

	pflag.Parse()

	return &HttpFlags{
		ShowHeaders:     *showHeaders,
		ShowOnlyHeaders: *showOnlyHeaders,
		CustomHeaders:   customHeaders,
		OutputFile:      *outputFile,
	}
}

func CreateOutputFile(fileName string) (*os.File, error) {
	if fileName == "" {
		return nil, fmt.Errorf("file name is empty")
	}
	file, err := os.Create(fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %w", err)
	}
	return file, nil
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

func readHeaders(reader *bufio.Reader) (string, bool, error) {
	var headers strings.Builder
	var isChunked bool
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", false, err
		}
		headers.WriteString(line)
		if line == "\r\n" {
			break // End of headers
		}
		if strings.HasPrefix(line, "Transfer-Encoding: chunked") {
			isChunked = true
		}
	}
	return headers.String(), isChunked, nil
}

func readNonChunkedResponse(reader *bufio.Reader, output io.Writer) {
	buf := make([]byte, 4096)
	for {
		n, err := reader.Read(buf)
		if err != nil {
			if err != io.EOF {
				fmt.Fprintln(os.Stderr, "read error:", err)
			}
			break
		}
		if _, err := output.Write(buf[:n]); err != nil {
			fmt.Fprintln(os.Stderr, "write error:", err)
		}
	}
}

// Reads a line from the given reader.
func readLine(reader io.Reader) (string, error) {
	bufReader, ok := reader.(*bufio.Reader)
	if !ok {
		bufReader = bufio.NewReader(reader)
	}
	line, err := bufReader.ReadString('\n')
	return strings.TrimRight(line, "\r\n"), err
}

func trimSpace(line string) string {
	return strings.TrimSpace(line)
}

func parseIntHex(hexString string) (int64, error) {
	return strconv.ParseInt(hexString, 16, 64)
}

// Reads bytes into a buffer from a given reader.
func readBytes(reader io.Reader, buf []byte) error {
	_, err := io.ReadFull(reader, buf)
	return err
}

// Writes data to stdout.
func write(data []byte, output io.Writer) error {
	_, err := output.Write(data)
	return err
}

// Discards a specified number of bytes from a reader.
func discardBytes(reader io.Reader, n int) error {
	_, err := io.CopyN(io.Discard, reader, int64(n))
	return err
}

func readChunkedResponse(reader io.Reader, output io.Writer) error {

	for {

		// Read chunk size line
		sizeLine, err := readLine(reader)
		if err != nil {
			return err
		}

		// Trim whitespace
		sizeLine = trimSpace(sizeLine)

		// Check for empty line
		if sizeLine == "" {
			break
		}

		// Parse chunk size
		size, err := parseIntHex(sizeLine)
		if err != nil {
			return err
		}

		// End of response
		if size == 0 {
			break
		}

		// Read chunk data
		chunkData := make([]byte, size)
		if err := readBytes(reader, chunkData); err != nil {
			return err
		}

		// Write chunk data
		if err := write(chunkData, output); err != nil {
			return err
		}

		// Discard trailing CRLF
		if err := discardBytes(reader, 2); err != nil {
			return err
		}
	}

	return nil
}

func HttpGet(url string, output io.Writer, flags *HttpFlags) error {
	host, port, path := parseURL(url)
	conn, err := net.Dial("tcp", host+":"+port)
	if err != nil {
		return err
	}
	defer conn.Close()

	requestLine := fmt.Sprintf("GET %s HTTP/1.1\r\n", path)
	requestHeaders := fmt.Sprintf("Host: %s\r\n", host)
	for _, header := range flags.CustomHeaders {
		requestHeaders += fmt.Sprintf(header + "\r\n")
	}
	requestHeaders += "Connection: close\r\n\r\n"
	request := requestLine + requestHeaders
	conn.Write([]byte(request))

	reader := bufio.NewReader(conn)

	// Read headers
	headers, isChunked, err := readHeaders(reader)
	if err != nil {
		return err
	}

	if flags.ShowHeaders || flags.ShowOnlyHeaders {
		output.Write([]byte(headers)) // Print headers
		if flags.ShowOnlyHeaders {
			return nil // Return early if only headers should be shown
		}
	}

	if isChunked {
		readChunkedResponse(reader, output)
	} else {
		readNonChunkedResponse(reader, output)
	}

	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run tinyhttp.go <url>")
		return
	}
	var url string
	if len(os.Args) > 2 {
		url = os.Args[len(os.Args)-1]
	} else {
		url = os.Args[1]
	}
	flags := parseFlags()

	var output io.Writer
	output = os.Stdout // Default to stdout

	if flags.OutputFile != "" {
		file, err := CreateOutputFile(flags.OutputFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		defer file.Close()
		output = file
	}

	HttpGet(url, output, flags)
}
