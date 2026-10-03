package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepare(t *testing.T) {
	for _, tc := range []struct {
		name, branch, status, proposed, tags string
		opts                                 options
		wantError, wantWrite                 bool
	}{
		{name: "preview", branch: "develop", proposed: "v0.2.0", tags: "v0.1.0", opts: options{preview: true}},
		{name: "prepare", branch: "develop", proposed: "v0.2.0", tags: "v0.1.0", wantWrite: true},
		{name: "first release", branch: "develop", proposed: "v0.1.0", wantWrite: true},
		{name: "override", branch: "develop", tags: "v0.1.0", opts: options{version: "v1.0.0"}, wantWrite: true},
		{name: "dirty", branch: "develop", status: " M README.md", wantError: true},
		{name: "dirty preview", branch: "develop", status: " M README.md", proposed: "v0.2.0", tags: "v0.1.0", opts: options{preview: true}},
		{name: "wrong branch", branch: "main", wantError: true},
		{name: "no bump", branch: "develop", proposed: "v0.1.0", tags: "v0.1.0", wantError: true},
		{name: "older", branch: "develop", tags: "v0.1.0", opts: options{version: "v0.0.9"}, wantError: true},
		{name: "prerelease", branch: "develop", opts: options{version: "v1.0.0-beta.1"}, wantError: true},
		{name: "leading zero", branch: "develop", opts: options{version: "v01.0.0"}, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			history := "## [0.1.0] - 2026-01-01\n\n- Reviewed history.\n"
			before := "# Changelog\n\n## [Unreleased]\n\n- Pending.\n\n" + history
			path := filepath.Join(directory, "CHANGELOG.md")
			if err := os.WriteFile(path, []byte(before), 0o644); err != nil {
				t.Fatal(err)
			}
			run := func(name string, args ...string) (string, error) {
				command := name + " " + strings.Join(args, " ")
				switch command {
				case "git branch --show-current":
					return tc.branch, nil
				case "git status --porcelain":
					return tc.status, nil
				case "git fetch origin --tags":
					return "", nil
				case "git tag --merged HEAD --list v* --sort=-version:refname", "git tag --list":
					return tc.tags, nil
				case "git-cliff --bumped-version":
					return tc.proposed, nil
				}
				if name == "git-cliff" && args[0] == "--unreleased" {
					return "## [" + strings.TrimPrefix(args[2], "v") + "] - 2026-10-03\n\n- Feature.", nil
				}
				return "", fmt.Errorf("unexpected command (must not commit/tag/publish): %s", command)
			}
			var output bytes.Buffer
			err := prepare(tc.opts, directory, run, &output)
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v, wantError = %v", err, tc.wantError)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !tc.wantWrite && string(after) != before {
				t.Fatal("preview or rejection wrote a file")
			}
			if tc.wantWrite {
				if string(after) == before || strings.Contains(string(after), "Pending") {
					t.Fatal("entry was not replaced")
				}
				if !strings.HasSuffix(string(after), history) {
					t.Fatal("reviewed history was changed")
				}
			}
		})
	}
}

func TestNewer(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"v0.10.0", "v0.9.9", true}, {"v1.0.0", "v0.99.99", true},
		{"v0.1.0", "v0.1.0", false}, {"v0.0.9", "v0.1.0", false},
		{"v999999999999999999999.0.0", "v1.0.0", true},
	} {
		if got := newer(tc.a, tc.b); got != tc.want {
			t.Errorf("newer(%s, %s) = %v", tc.a, tc.b, got)
		}
	}
}

func TestInsertEntryRejectsMalformedInput(t *testing.T) {
	for _, tc := range []struct{ existing, entry string }{
		{"# No Unreleased", "## [0.1.0]\n"},
		{"## [Unreleased]\n", ""},
	} {
		if _, err := insertEntry(tc.existing, tc.entry); err == nil {
			t.Fatal("accepted malformed changelog")
		}
	}
}
