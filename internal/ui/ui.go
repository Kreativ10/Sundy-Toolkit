package ui

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	Reset   = "\x1b[0m"
	Bold    = "\x1b[1m"
	Dim     = "\x1b[2m"
	Orange  = "\x1b[38;5;208m"
	Orange2 = "\x1b[38;5;214m"
	Green   = "\x1b[38;5;82m"
	Yellow  = "\x1b[38;5;220m"
	Red     = "\x1b[38;5;196m"
	Gray    = "\x1b[38;5;245m"
	White   = "\x1b[38;5;255m"
)

var noColor = os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb"

func C(code, s string) string {
	if noColor {
		return s
	}
	return code + s + Reset
}

func Logo() string {
	art := `
   ███████╗██╗   ██╗███╗   ██╗██████╗ ██╗   ██╗
   ██╔════╝██║   ██║████╗  ██║██╔══██╗╚██╗ ██╔╝
   ███████╗██║   ██║██╔██╗ ██║██║  ██║ ╚████╔╝
   ╚════██║██║   ██║██║╚██╗██║██║  ██║  ╚██╔╝
   ███████║╚██████╔╝██║ ╚████║██████╔╝   ██║
   ╚══════╝ ╚═════╝ ╚═╝  ╚═══╝╚═════╝    ╚═╝

   ████████╗ ██████╗  ██████╗ ██╗     ██╗  ██╗██╗████████╗
   ╚══██╔══╝██╔═══██╗██╔═══██╗██║     ██║ ██╔╝██║╚══██╔══╝
      ██║   ██║   ██║██║   ██║██║     █████╔╝ ██║   ██║
      ██║   ██║   ██║██║   ██║██║     ██╔═██╗ ██║   ██║
      ██║   ╚██████╔╝╚██████╔╝███████╗██║  ██╗██║   ██║
      ╚═╝    ╚═════╝  ╚═════╝ ╚══════╝╚═╝  ╚═╝╚═╝   ╚═╝`
	return C(Orange, art) + "\n"
}

func Header(title string) { fmt.Printf("\n%s %s\n", C(Orange, "◆"), C(Bold, title)) }
func Info(msg string)     { fmt.Printf("%s %s\n", C(Orange2, "●"), msg) }
func Success(msg string)  { fmt.Printf("%s %s\n", C(Green, "✓"), msg) }
func Warn(msg string)     { fmt.Printf("%s %s\n", C(Yellow, "!"), msg) }
func Error(msg string)    { fmt.Printf("%s %s\n", C(Red, "✗"), msg) }
func Muted(msg string)    { fmt.Println(C(Gray, msg)) }

func Box(title string, lines []string) {
	width := len(title) + 4
	for _, l := range lines {
		if len(stripANSI(l))+4 > width {
			width = len(stripANSI(l)) + 4
		}
	}
	if width < 52 {
		width = 52
	}
	fmt.Println(C(Orange, "╭─ ") + C(Bold, title) + C(Orange, strings.Repeat("─", max(1, width-len(title)-4))+"╮"))
	for _, l := range lines {
		fmt.Printf("%s %-*s %s\n", C(Orange, "│"), width-2, l, C(Orange, "│"))
	}
	fmt.Println(C(Orange, "╰"+strings.Repeat("─", width)+"╯"))
}

func Menu(title string, items []string) (int, error) {
	Header(title)
	for i, item := range items {
		fmt.Printf("  %s %s\n", C(Orange, fmt.Sprintf("[%d]", i+1)), item)
	}
	fmt.Printf("\n%s ", C(Orange2, "Select:"))
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || n < 1 || n > len(items) {
		return 0, fmt.Errorf("invalid selection")
	}
	return n - 1, nil
}

func Prompt(label, def string) (string, error) {
	if def != "" {
		fmt.Printf("%s %s [%s]: ", C(Orange2, "›"), label, C(Gray, def))
	} else {
		fmt.Printf("%s %s: ", C(Orange2, "›"), label)
	}
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def, nil
	}
	return line, nil
}

func Confirm(label string, def bool) bool {
	suffix := "[y/N]"
	if def {
		suffix = "[Y/n]"
	}
	fmt.Printf("%s %s %s: ", C(Orange2, "?"), label, suffix)
	r := bufio.NewReader(os.Stdin)
	line, _ := r.ReadString('\n')
	line = strings.ToLower(strings.TrimSpace(line))
	if line == "" {
		return def
	}
	return line == "y" || line == "yes"
}

func SelectMany(title string, items []string, defaultAll bool) ([]int, error) {
	Header(title)
	for i, item := range items {
		mark := " "
		if defaultAll {
			mark = "x"
		}
		fmt.Printf("  [%s] %d. %s\n", mark, i+1, item)
	}
	Muted("Enter comma-separated numbers, 'all', or press Enter to use the defaults.")
	fmt.Printf("%s ", C(Orange2, "Select:"))
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimSpace(strings.ToLower(line))
	if line == "" && defaultAll {
		out := make([]int, len(items))
		for i := range items {
			out[i] = i
		}
		return out, nil
	}
	if line == "all" {
		out := make([]int, len(items))
		for i := range items {
			out[i] = i
		}
		return out, nil
	}
	var out []int
	seen := map[int]bool{}
	for _, p := range strings.Split(line, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n < 1 || n > len(items) {
			return nil, fmt.Errorf("invalid selection %q", p)
		}
		n--
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out, nil
}

func KeyValue(k, v string) string { return fmt.Sprintf("%-18s %s", C(Gray, k), v) }

func stripANSI(s string) string {
	out := make([]rune, 0, len(s))
	esc := false
	for _, r := range s {
		if r == '\x1b' {
			esc = true
			continue
		}
		if esc {
			if r == 'm' {
				esc = false
			}
			continue
		}
		out = append(out, r)
	}
	return string(out)
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
