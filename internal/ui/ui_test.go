package ui

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestBoxBorders(t *testing.T) {
	for _, width := range []int{20, 40, 80, 120} {
		for _, colored := range []bool{false, true} {
			t.Run(strings.Repeat("w", width), func(t *testing.T) {
				original := noColor
				noColor = !colored
				defer func() { noColor = original }()
				var out bytes.Buffer
				renderBox(&out, "Система 主机", []string{KeyValue("Хост", "тест"), C(Orange, "✓ Готово"), "e\u0301 / 日本語", strings.Repeat("очень длинная строка ", 10), "first\nsecond", "tab\there"}, width)
				lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
				expected := displayWidth(lines[0])
				for _, line := range lines {
					if got := displayWidth(line); got != expected || got > width {
						t.Fatalf("border mismatch: got %d want %d (terminal %d): %q", got, expected, width, line)
					}
				}
			})
		}
	}
}

func TestPromptsPreservePipedInputAndRejectEOFConfirmation(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "input")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	file.WriteString("2\nsurvival\n\ny\n")
	file.Seek(0, 0)
	old := os.Stdin
	os.Stdin = file
	defer func() { os.Stdin = old }()
	n, err := Menu("Test", []string{"first", "second"})
	if err != nil || n != 1 {
		t.Fatalf("menu %d %v", n, err)
	}
	name, err := Prompt("Name", "")
	if err != nil || name != "survival" {
		t.Fatalf("lost buffered answer: %q %v", name, err)
	}
	selected, err := SelectMany("Empty", []string{"one"}, false)
	if err != nil || len(selected) != 0 {
		t.Fatalf("empty selection: %v %v", selected, err)
	}
	if !Confirm("Accept", false) {
		t.Fatal("lost confirmation")
	}
	if Confirm("EOF", true) {
		t.Fatal("EOF approved a destructive default")
	}
}
