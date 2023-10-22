package main

import (
	"bytes"
	"fmt"
	"testing"
)

func BenchmarkFmtFprint(b *testing.B) {
	var output bytes.Buffer
	headers := "Some sample headers for the benchmark test\n" +
		"Header1: Value1\nHeader2: Value2\nHeader3: Value3\n"
	for i := 0; i < b.N; i++ {
		output.Reset() // Reset the buffer to ensure consistent testing
		fmt.Fprint(&output, headers)
	}
}

func BenchmarkOutputWrite(b *testing.B) {
	var output bytes.Buffer
	headers := "Some sample headers for the benchmark test\n" +
		"Header1: Value1\nHeader2: Value2\nHeader3: Value3\n"
	for i := 0; i < b.N; i++ {
		output.Reset() // Reset the buffer to ensure consistent testing
		output.Write([]byte(headers))
	}
}
