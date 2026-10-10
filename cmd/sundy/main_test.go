package main

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestUnicodeDiagnosticTruncation(t *testing.T) {
	text := oneLine("  Ошибка\nподключения  ", 8)
	if !utf8.ValidString(text) || !strings.HasSuffix(text, "…") {
		t.Fatalf("invalid truncated text: %q", text)
	}
}

func TestInvalidReadOnlyCommands(t *testing.T) {
	if err := cmdOverview([]string{"--does-not-exist"}); err == nil {
		t.Fatal("accepted invalid overview flag")
	}
	if err := cmdSnapshot([]string{"show"}); err == nil {
		t.Fatal("snapshot show accepted no name")
	}
	if err := cmdService([]string{"start"}); err == nil {
		t.Fatal("service accepted no name")
	}
	if err := cmdRuntime([]string{"invalid"}); err == nil {
		t.Fatal("accepted invalid runtime")
	}
}
