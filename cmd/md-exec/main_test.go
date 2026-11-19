package main

import (
	"errors"
	"reflect"
	"testing"
)

const demo = "../../examples/demo/demo.md"

func TestParseFlag(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		want   []string
		rest   []string
		hasErr bool
	}{
		{"long separate", []string{"--width", "40", "doc"}, []string{"40"}, []string{"doc"}, false},
		{"long equals", []string{"--width=40", "doc"}, []string{"40"}, []string{"doc"}, false},
		{"short separate", []string{"-width", "40", "doc"}, []string{"40"}, []string{"doc"}, false},
		{"short equals", []string{"-width=40", "doc"}, []string{"40"}, []string{"doc"}, false},
		{"repeated", []string{"--width=40", "-width=80"}, []string{"40", "80"}, nil, false},
		{"missing value", []string{"doc", "--width"}, nil, nil, true},
		{"unknown flag is positional", []string{"--other", "doc"}, nil, []string{"--other", "doc"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values, rest, err := parseFlag(tt.args, "width", "width")
			if tt.hasErr {
				if err == nil {
					t.Fatal("want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseFlag: %v", err)
			}
			if !reflect.DeepEqual(values, tt.want) {
				t.Errorf("values = %v, want %v", values, tt.want)
			}
			if !reflect.DeepEqual(rest, tt.rest) {
				t.Errorf("rest = %v, want %v", rest, tt.rest)
			}
		})
	}
}

func TestParseSetArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantKeys []string
		wantPos  []string
		wantErr  error
		hasErr   bool
	}{
		{"long", []string{"doc", "--set", "A=1"}, []string{"A"}, []string{"doc"}, nil, false},
		{"short", []string{"doc", "-set", "A=1"}, []string{"A"}, []string{"doc"}, nil, false},
		{"short equals", []string{"doc", "-set=A=1"}, []string{"A"}, []string{"doc"}, nil, false},
		{"many", []string{"--set=A=1", "doc", "-set", "B=2"}, []string{"A", "B"}, []string{"doc"}, nil, false},
		{"help", []string{"doc", "-h"}, nil, nil, errHelp, false},
		{"long help", []string{"doc", "--help"}, nil, nil, errHelp, false},
		{"missing value", []string{"doc", "--set"}, nil, nil, nil, true},
		{"bad value", []string{"doc", "--set", "BAD"}, nil, nil, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sets, pos, err := parseSetArgs(tt.args)
			if tt.hasErr {
				if err == nil {
					t.Fatal("want error")
				}
				return
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && err != nil {
				t.Fatalf("parseSetArgs: %v", err)
			}
			var keys []string
			for _, kv := range sets {
				keys = append(keys, kv.key)
			}
			if !reflect.DeepEqual(keys, tt.wantKeys) {
				t.Errorf("keys = %v, want %v", keys, tt.wantKeys)
			}
			if !reflect.DeepEqual(pos, tt.wantPos) {
				t.Errorf("positional = %v, want %v", pos, tt.wantPos)
			}
		})
	}
}

func TestParsePrintArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantWidth int
		wantTheme string
		hasErr    error
		fails     bool
	}{
		{"no flags", []string{"doc"}, 0, "", nil, false},
		{"long width", []string{"--width", "40", "doc"}, 40, "", nil, false},
		{"short equals width", []string{"-width=40", "doc"}, 40, "", nil, false},
		{"theme", []string{"--theme=dark", "doc"}, 0, "dark", nil, false},
		{"both", []string{"doc", "-width=80", "-theme=notty"}, 80, "notty", nil, false},
		{"last width wins", []string{"--width=40", "-width=80", "doc"}, 80, "", nil, false},
		{"help", []string{"doc", "-h"}, 0, "", errHelp, false},
		{"zero width", []string{"--width=0", "doc"}, 0, "", nil, true},
		{"negative width", []string{"--width=-1", "doc"}, 0, "", nil, true},
		{"bad width", []string{"--width=x", "doc"}, 0, "", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			themeName, width, pos, err := parsePrintArgs(tt.args)
			if tt.fails {
				if err == nil {
					t.Fatal("want error")
				}
				return
			}
			if tt.hasErr != nil && !errors.Is(err, tt.hasErr) {
				t.Fatalf("err = %v, want %v", err, tt.hasErr)
			}
			if tt.hasErr == nil && err != nil {
				t.Fatalf("parsePrintArgs: %v", err)
			}
			if width != tt.wantWidth || themeName != tt.wantTheme {
				t.Errorf("got width=%d theme=%q, want %d/%q", width, themeName, tt.wantWidth, tt.wantTheme)
			}
			if tt.hasErr == nil && len(pos) != 1 {
				t.Errorf("positional = %v, want one document", pos)
			}
		})
	}
}

func TestRunExitCodes(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"no args", nil, 2},
		{"help", []string{"--help"}, 0},
		{"version", []string{"version"}, 0},
		{"check valid", []string{"check", demo}, 0},
		{"check missing", []string{"check", "does-not-exist.md"}, 2},
		{"list valid", []string{"list", demo}, 0},
		{"globals with override", []string{"globals", demo, "--set", "JOB_NAME=prod"}, 0},
		{"globals bad override", []string{"globals", demo, "--set", "BAD"}, 2},
		{"print short width", []string{"print", demo, "-width=40"}, 0},
		{"print help", []string{"print", demo, "-h"}, 0},
		{"unknown command", []string{"frobnicate"}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(tt.args); got != tt.want {
				t.Errorf("run(%v) = %d, want %d", tt.args, got, tt.want)
			}
		})
	}
}
