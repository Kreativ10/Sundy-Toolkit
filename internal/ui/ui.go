package ui

import (
	"bufio"
	"fmt"
	"io"
	"regexp"

	"github.com/mattn/go-runewidth"
	"golang.org/x/term"
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
	renderBox(os.Stdout, title, lines, terminalWidth())
}

func terminalWidth() int {
	if width, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && width > 0 {
		return width
	}
	if width, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && width > 0 {
		return width
	}
	return 80
}

func renderBox(w io.Writer, title string, lines []string, columns int) {
	columns = max(8, columns)
	width := max(50, displayWidth(title)+2)
	for _, line := range lines {
		for _, part := range strings.Split(line, "\n") {
			width = max(width, displayWidth(part))
		}
	}
	width = min(width, columns-4)
	title = runewidth.Truncate(stripANSI(title), width-2, "…")
	fmt.Fprintln(w, C(Orange, "╭─ ")+C(Bold, title)+C(Orange, " "+strings.Repeat("─", width-displayWidth(title)-1)+"╮"))
	for _, line := range lines {
		for _, part := range strings.Split(line, "\n") {
			part = strings.ReplaceAll(part, "\t", "    ")
			if displayWidth(part) > width {
				// Wrap plain text so an ANSI sequence cannot be split across lines.
				part = runewidth.Wrap(stripANSI(part), width)
			}
			for _, row := range strings.Split(part, "\n") {
				fmt.Fprintln(w, C(Orange, "│")+" "+row+strings.Repeat(" ", max(0, width-displayWidth(row)))+" "+C(Orange, "│"))
			}
		}
	}
	fmt.Fprintln(w, C(Orange, "╰"+strings.Repeat("─", width+2)+"╯"))
}

// Reuse the reader: creating one per prompt discards buffered answers from pipes.
var inputFile *os.File
var inputReader *bufio.Reader

func Input() *bufio.Reader {
	if inputReader == nil || inputFile != os.Stdin {
		inputFile = os.Stdin
		inputReader = bufio.NewReader(os.Stdin)
	}
	return inputReader
}

func readLine() (string, error) {
	line, err := Input().ReadString('\n')
	if err == io.EOF && len(line) > 0 {
		err = nil
	}
	return line, err
}

func Menu(title string, items []string) (int, error) {
	Header(title)
	for i, item := range items {
		fmt.Printf("  %s %s\n", C(Orange, fmt.Sprintf("[%d]", i+1)), item)
	}
	fmt.Printf("\n%s ", C(Orange2, "Select:"))
	line, err := readLine()
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
	line, err := readLine()
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
	line, err := readLine()
	if err != nil {
		return false
	}
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
	line, err := readLine()
	if err != nil {
		return nil, err
	}
	line = strings.TrimSpace(strings.ToLower(line))
	if line == "" && !defaultAll {
		return nil, nil
	}
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

func KeyValue(k, v string) string {
	return C(Gray, k) + strings.Repeat(" ", max(1, 19-displayWidth(k))) + v
}

var ansiRx = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

func stripANSI(s string) string { return ansiRx.ReplaceAllString(s, "") }
func displayWidth(s string) int { return runewidth.StringWidth(stripANSI(s)) }
