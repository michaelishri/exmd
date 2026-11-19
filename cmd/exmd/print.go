package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"golang.org/x/term"

	"github.com/michaelishri/exmd/internal/render"
	"github.com/michaelishri/exmd/internal/theme"
)

const printUsage = "usage: exmd print DOC [--width N] [--theme auto|dark|notty]"

// parsePrintArgs parses print's flags, accepting both dash forms for --width
// and --theme via the shared flag parser.
func parsePrintArgs(args []string) (themeName string, width int, positional []string, err error) {
	args, help := extractHelp(args)
	if help {
		return "", 0, nil, errHelp
	}
	themeValues, args, err := parseFlag(args, "theme", "theme")
	if err != nil {
		return "", 0, nil, err
	}
	widthValues, positional, err := parseFlag(args, "width", "width")
	if err != nil {
		return "", 0, nil, err
	}
	if len(themeValues) > 0 {
		themeName = themeValues[len(themeValues)-1]
	}
	if len(widthValues) > 0 {
		raw := widthValues[len(widthValues)-1]
		width, err = strconv.Atoi(raw)
		if err != nil || width <= 0 {
			return "", 0, nil, fmt.Errorf("invalid width %q", raw)
		}
	}
	return themeName, width, positional, nil
}

// cmdPrint renders the document to the terminal — a reader's view of the
// runbook (SPEC.md §4.8 observation, on a stream). --theme selects the
// palette.
func cmdPrint(args []string) int {
	themeName, width, positional, err := parsePrintArgs(args)
	if errors.Is(err, errHelp) {
		fmt.Println(printUsage)
		return 0
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "exmd print: %v\n", err)
		return 2
	}
	if len(positional) != 1 {
		fmt.Fprintln(os.Stderr, printUsage)
		return 2
	}

	th, err := resolveTheme(themeName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "exmd print: %v\n", err)
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
			width = render.DefaultWidth
		}
	}
	// Forcing the notty theme suppresses color even on a capable terminal.
	color := tty && colorSupported() && th.Name != theme.Notty.Name

	out := render.Render(res.Doc, width, color)
	for _, l := range out.Lines {
		fmt.Println(l)
	}
	return 0
}
