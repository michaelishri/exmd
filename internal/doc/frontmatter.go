package doc

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

var globalNameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// splitFrontmatter detects the frontmatter block of src (SPEC.md §2.1).
//
// It returns the raw YAML between the `---` markers, the 1-based line of the
// opening marker (0 when there is no frontmatter), whether a closing marker
// was found, the 1-based first line after the frontmatter, and a copy of the
// source in which the YAML content is blanked out with spaces. The masked
// copy has identical byte offsets, so Markdown parsing over it yields source
// positions that are valid for the original.
func splitFrontmatter(src []byte) (yamlSrc []byte, openLine int, closed bool, bodyLine int, masked []byte) {
	first, rest, hasNL := bytes.Cut(src, []byte("\n"))
	if string(bytes.TrimRight(first, " \t\r")) != "---" {
		return nil, 0, false, 1, src
	}
	openLine = 1
	masked = append([]byte(nil), src...)

	if !hasNL {
		// The file is exactly "---" with no trailing newline.
		return nil, openLine, false, 1, masked
	}

	offset := len(first) + 1 // start of line 2
	pos := 0
	for {
		eol := bytes.IndexByte(rest[pos:], '\n')
		var line []byte
		if eol < 0 {
			line = rest[pos:]
		} else {
			line = rest[pos : pos+eol]
		}
		if string(bytes.TrimRight(line, " \t\r")) == "---" {
			closeStart := offset + pos
			yamlSrc = src[offset:closeStart]
			for i := offset; i < closeStart; i++ {
				if masked[i] != '\n' && masked[i] != '\r' {
					masked[i] = ' '
				}
			}
			closing := 1 + bytes.Count(src[:closeStart], []byte{'\n'})
			return yamlSrc, openLine, true, closing + 1, masked
		}
		if eol < 0 {
			break
		}
		pos += eol + 1
	}
	return nil, openLine, false, 1, masked
}

// parseFrontmatter validates the frontmatter YAML and fills in d. It reports
// whether the YAML parsed well enough to trust `type` (SPEC.md §2.1).
func parseFrontmatter(yamlSrc []byte, openLine int, d *Document, res *Result) (trusted bool) {
	var root yaml.Node
	if err := yaml.Unmarshal(yamlSrc, &root); err != nil {
		res.add(newDiag(SeverityError, adjustYamlLine(err.Error(), openLine), "invalid frontmatter YAML: "+strings.TrimPrefix(err.Error(), "yaml: ")))
		return false
	}
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			return true
		}
		root = *root.Content[0]
	}
	if root.Kind == 0 || (root.Kind == yaml.ScalarNode && root.Tag == "!!null") {
		return true
	}
	if root.Kind != yaml.MappingNode {
		res.add(newDiag(SeverityError, nodeLine(&root, openLine), "frontmatter must be a YAML mapping"))
		return false
	}

	var unknown []string
	for i := 0; i+1 < len(root.Content); i += 2 {
		key, val := root.Content[i], root.Content[i+1]
		switch key.Value {
		case "type":
			parseType(val, d, res, openLine)
		case "title":
			if value, ok := scalarValue(val, "title", openLine, res); ok {
				d.Title = value
			}
		case "spec":
			parseSpec(val, d, res, openLine)
		case "globals":
			parseGlobals(val, d, res, openLine)
		default:
			unknown = append(unknown, key.Value)
		}
	}
	if len(unknown) > 0 {
		res.add(newDiag(SeverityWarning, nodeLine(&root, openLine), fmt.Sprintf("unknown frontmatter key(s): %s (ignored)", strings.Join(unknown, ", "))))
	}

	d.IsRunbook = d.TypeValue == "runbook"
	if !d.IsRunbook {
		msg := "not a runbook: frontmatter is missing `type: runbook`"
		if d.TypeValue != "" {
			msg = fmt.Sprintf("not a runbook: `type` is `%s`, expected `runbook`", d.TypeValue)
		}
		res.add(newDiag(SeverityError, openLine, msg))
	}
	return true
}

// scalarValue reads a string frontmatter value. It reports an error for a
// non-scalar node and returns false for a null node.
func scalarValue(val *yaml.Node, name string, openLine int, res *Result) (string, bool) {
	if val.Kind == yaml.ScalarNode && val.Tag == "!!null" {
		return "", false
	}
	if val.Kind != yaml.ScalarNode {
		res.add(newDiag(SeverityError, nodeLine(val, openLine),
			fmt.Sprintf("frontmatter `%s` must be a string", name)))
		return "", false
	}
	return val.Value, true
}

func parseType(val *yaml.Node, d *Document, res *Result, openLine int) {
	if value, ok := scalarValue(val, "type", openLine, res); ok {
		d.TypeValue = value
	}
}

func parseSpec(val *yaml.Node, d *Document, res *Result, openLine int) {
	invalid := "frontmatter `spec` must be an integer"
	switch {
	case val.Kind == yaml.ScalarNode && val.Tag == "!!null":
		return
	case val.Kind != yaml.ScalarNode:
		res.add(newDiag(SeverityError, nodeLine(val, openLine), invalid))
		return
	default:
		var v int
		if err := val.Decode(&v); err != nil {
			res.add(newDiag(SeverityError, nodeLine(val, openLine), invalid))
			return
		}
		if v != 1 {
			res.add(newDiag(SeverityError, nodeLine(val, openLine), fmt.Sprintf("unsupported spec version %d (this runner implements version 1)", v)))
		}
	}
}

// parseGlobals validates `globals` (SPEC.md §2.2): a mapping of names to
// scalar values, where a null value (or bare key) declares a global that
// starts absent.
func parseGlobals(val *yaml.Node, d *Document, res *Result, openLine int) {
	switch {
	case val.Kind == yaml.ScalarNode && val.Tag == "!!null":
		return
	case val.Kind != yaml.MappingNode:
		res.add(newDiag(SeverityError, nodeLine(val, openLine), "`globals` must be a mapping (block or flow style); for a one-line list use the flow form, e.g. globals: {JOB_ID, AUTHOR: \"Michael\"}"))
		return
	}

	seen := map[string]int{}
	for i := 0; i+1 < len(val.Content); i += 2 {
		key, v := val.Content[i], val.Content[i+1]
		name := key.Value
		if !globalNameRE.MatchString(name) {
			res.add(newDiag(SeverityError, nodeLine(key, openLine), fmt.Sprintf("invalid global name `%s` (must match [A-Za-z_][A-Za-z0-9_]*)", name)))
			continue
		}
		if line, dup := seen[name]; dup {
			res.add(newDiag(SeverityError, nodeLine(key, openLine), fmt.Sprintf("duplicate global `%s` (first declared at line %d)", name, line)))
			continue
		}
		seen[name] = nodeLine(key, openLine)

		if v.Kind != yaml.ScalarNode {
			res.add(newDiag(SeverityError, nodeLine(v, openLine), fmt.Sprintf("`globals` value for `%s` must be a scalar", name)))
			continue
		}
		if v.Tag == "!!null" {
			d.Globals[name] = nil // bare key: declared, starts absent
			continue
		}
		value := v.Value
		if v.Tag == "!!bool" {
			value = strings.ToLower(value)
		}
		d.Globals[name] = &value
	}
}

// nodeLine converts a yaml.Node line (1-based within the YAML content) to a
// file line, given the frontmatter opening line.
func nodeLine(n *yaml.Node, openLine int) int {
	if n.Line <= 0 {
		return openLine
	}
	return n.Line + openLine
}

var yamlLineRE = regexp.MustCompile(`\bline (\d+)`)

// adjustYamlLine best-effort rewrites a `line N` reference in a yaml error
// message into a file line, offsetting by the frontmatter opening line.
func adjustYamlLine(msg string, openLine int) int {
	if m := yamlLineRE.FindStringSubmatch(msg); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			return n + openLine
		}
	}
	return openLine
}
