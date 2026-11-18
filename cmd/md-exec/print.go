package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"

	"github.com/michaelishri/md-exec/internal/render"
)

// cmdPrint renders the document to the terminal — a reader's view of the
// runbook (SPEC.md §4.8 observation, on a stream).
func cmdPrint(args []string) int {
	width := 0
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--width" || a == "-width":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "md-exec print: --width requires a value")
				return 2
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n <= 0 {
				fmt.Fprintf(os.Stderr, "md-exec print: invalid width %q\n", args[i])
				return 2
			}
			width = n
		case strings.HasPrefix(a, "--width="):
			n, err := strconv.Atoi(strings.TrimPrefix(a, "--width="))
			if err != nil || n <= 0 {
				fmt.Fprintf(os.Stderr, "md-exec print: invalid width %q\n", a)
				return 2
			}
			width = n
		case a == "-h" || a == "--help":
			fmt.Println("usage: md-exec print DOC [--width N]")
			return 0
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) != 1 {
		fmt.Fprintln(os.Stderr, "usage: md-exec print DOC [--width N]")
		return 2
	}

	res, code := loadDoc(positional[0])
	if res == nil {
		return code
	}
	res.Sort()
	printDiagnostics(res)
	if res.HasErrors() {
		return 2
	}

	tty := term.IsTerminal(int(os.Stdout.Fd()))
	if width == 0 {
		if tty {
			if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
				width = w
			}
		}
		if width <= 0 {
			width = 80
		}
	}
	color := tty && lipgloss.ColorProfile() != termenv.Ascii

	out, err := render.Render(res.Doc, width, color)
	if err != nil {
		fmt.Fprintf(os.Stderr, "md-exec: %v\n", err)
		return 2
	}
	for _, l := range out.Lines {
		fmt.Println(l)
	}
	return 0
}
