package main

import (
	"bufio"
	"io"
	"os"
	"strings"
	"testing"
)

func TestMenuShowsVersionAndUpdateHint(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previousOutput, previousInput := os.Stdout, input
	defer func() {
		os.Stdout, input = previousOutput, previousInput
		reader.Close()
		writer.Close()
	}()
	os.Stdout = writer
	input = bufio.NewReader(strings.NewReader("0\n"))
	menuErr := menu()
	writer.Close()
	os.Stdout, input = previousOutput, previousInput
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if menuErr != nil {
		t.Fatal(menuErr)
	}
	for _, expected := range []string{
		"ChatGPT Update " + version,
		"Tip: 1 updates ChatGPT; 4 updates this utility.",
		"4) Update the updater",
		"0) Exit",
	} {
		if !strings.Contains(string(output), expected) {
			t.Errorf("menu missing %q; got %q", expected, output)
		}
	}
}
