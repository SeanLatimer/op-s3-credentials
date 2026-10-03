// Command release prepares a changelog without committing, tagging, or publishing.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type commandRunner func(string, ...string) (string, error)

type options struct {
	version string
	preview bool
	noFetch bool
}

var stableVersion = regexp.MustCompile(`^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`)
var unreleasedHeading = regexp.MustCompile(`(?m)^## \[Unreleased\][^\n]*\n`)
var releaseHeading = regexp.MustCompile(`(?m)^## \[`)

func main() {
	var opts options
	flag.StringVar(&opts.version, "version", "", "override the proposed stable version (e.g. v0.2.0)")
	flag.BoolVar(&opts.preview, "preview", false, "preview without writing files")
	flag.BoolVar(&opts.noFetch, "no-fetch", false, "use local tags without fetching origin")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		os.Exit(1)
	}
	run := func(name string, args ...string) (string, error) {
		cmd := exec.Command(name, args...)
		cmd.Stderr = os.Stderr
		out, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
		}
		return strings.TrimSpace(string(out)), nil
	}
	if err := prepare(opts, ".", run, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func prepare(opts options, directory string, run commandRunner, output io.Writer) error {
	branch, err := run("git", "branch", "--show-current")
	if err != nil {
		return err
	}
	if branch != "develop" {
		return fmt.Errorf("prepare releases on develop, then review develop -> main")
	}
	if !opts.preview {
		status, err := run("git", "status", "--porcelain")
		if err != nil {
			return err
		}
		if status != "" {
			return fmt.Errorf("commit your work first; use -preview to inspect without writing")
		}
	}
	if !opts.noFetch {
		if _, err := run("git", "fetch", "origin", "--tags"); err != nil {
			return err
		}
	}
	tags, err := run("git", "tag", "--merged", "HEAD", "--list", "v*", "--sort=-version:refname")
	if err != nil {
		return err
	}
	previous := ""
	for _, tag := range strings.Fields(tags) {
		if strings.HasPrefix(tag, "v") && stableVersion.MatchString(tag) {
			previous = tag
			break
		}
	}
	proposed := opts.version
	if proposed == "" {
		proposed, err = run("git-cliff", "--bumped-version")
		if err != nil {
			return err
		}
	}
	if !stableVersion.MatchString(proposed) {
		return fmt.Errorf("version must be a stable semantic version, e.g. v0.1.0")
	}
	tag := "v" + strings.TrimPrefix(proposed, "v")
	allTags, err := run("git", "tag", "--list")
	if err != nil {
		return err
	}
	for _, existing := range strings.Fields(allTags) {
		if tag == existing {
			return fmt.Errorf("tag %s already exists; no release needed or choose an override", tag)
		}
	}
	if previous != "" && !newer(tag, previous) {
		return fmt.Errorf("version must be newer than %s", previous)
	}
	entry, err := run("git-cliff", "--unreleased", "--tag", tag, "--strip", "all")
	if err != nil {
		return err
	}
	path := filepath.Join(directory, "CHANGELOG.md")
	existing, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	prepared, err := insertEntry(string(existing), entry)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "Proposed release: %s (previous: %s)\n", tag, previous); err != nil {
		return err
	}
	if opts.preview {
		_, err = io.WriteString(output, prepared)
		return err
	}
	if err := os.WriteFile(path, []byte(prepared), 0o644); err != nil {
		return err
	}
	_, err = fmt.Fprintln(output, "Updated CHANGELOG.md. Review it, commit with SSH signing, and open develop -> main.\nNo commit, tag, push, or release was performed.")
	return err
}

func newer(candidate, previous string) bool {
	a := strings.Split(strings.TrimPrefix(candidate, "v"), ".")
	b := strings.Split(strings.TrimPrefix(previous, "v"), ".")
	for i := range a {
		// Canonical decimal strings avoid integer overflow for SemVer components.
		if len(a[i]) != len(b[i]) {
			return len(a[i]) > len(b[i])
		}
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

func insertEntry(existing, entry string) (string, error) {
	heading := unreleasedHeading.FindStringIndex(existing)
	if heading == nil {
		return "", fmt.Errorf("CHANGELOG.md must contain ## [Unreleased]")
	}
	if !releaseHeading.MatchString(entry) {
		return "", fmt.Errorf("git-cliff did not return a release entry")
	}
	rest := existing[heading[1]:]
	history := ""
	if next := releaseHeading.FindStringIndex(rest); next != nil {
		history = rest[next[0]:]
	}
	return existing[:heading[0]] + "## [Unreleased]\n\n" + strings.TrimSpace(entry) + "\n\n" + history, nil
}
