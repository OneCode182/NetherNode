package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun_HelpAndVersion(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"help", []string{"help"}, "Usage:"},
		{"no args defaults to help", nil, "Usage:"},
		{"-h", []string{"-h"}, "Usage:"},
		{"--help", []string{"--help"}, "Usage:"},
		{"version", []string{"version"}, "nethernode "},
		{"--version", []string{"--version"}, "nethernode "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(tc.args, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("Run(%v) exit code = %d, want 0 (stderr: %s)", tc.args, code, stderr.String())
			}
			if !strings.Contains(stdout.String(), tc.want) {
				t.Fatalf("Run(%v) stdout = %q, want it to contain %q", tc.args, stdout.String(), tc.want)
			}
		})
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"frobnicate"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("Run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "unknown command: frobnicate") {
		t.Fatalf("stderr = %q, want an unknown-command message", stderr.String())
	}
}

func TestRun_DryRunFlagWorksInAnyPosition(t *testing.T) {
	cases := [][]string{
		{"--dry-run", "start"},
		{"start", "--dry-run"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(args, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("Run(%v) exit code = %d, want 0 (stderr: %s)", args, code, stderr.String())
			}
			if !strings.Contains(stdout.String(), "[dry-run]") {
				t.Fatalf("Run(%v) stdout = %q, want a [dry-run] plan", args, stdout.String())
			}
		})
	}
}

func TestExtractDryRun(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantFlag bool
		wantRest []string
	}{
		{"absent", []string{"status", "--json"}, false, []string{"status", "--json"}},
		{"leading", []string{"--dry-run", "status"}, true, []string{"status"}},
		{"trailing", []string{"status", "--dry-run"}, true, []string{"status"}},
		{"empty", nil, false, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotFlag, gotRest := extractDryRun(tc.args)
			if gotFlag != tc.wantFlag {
				t.Errorf("flag = %v, want %v", gotFlag, tc.wantFlag)
			}
			if len(gotRest) != len(tc.wantRest) {
				t.Fatalf("rest = %v, want %v", gotRest, tc.wantRest)
			}
			for i := range gotRest {
				if gotRest[i] != tc.wantRest[i] {
					t.Fatalf("rest = %v, want %v", gotRest, tc.wantRest)
				}
			}
		})
	}
}
