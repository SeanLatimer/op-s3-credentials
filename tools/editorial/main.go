// Command editorial is a best-effort CI release-note enhancement, never a release gate.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const toolVersion = "communique@1.5.0"
const freeModel = "nvidia/nemotron-3-ultra-550b-a55b:free"

type commandRunner func(context.Context, string, ...string) (string, error)

type settings struct {
	key, tag, repo, commit, directory string
}

type draft struct {
	SchemaVersion    int    `json:"schema_version"`
	PreserveSections bool   `json:"preserve_sections"`
	Tag              string `json:"tag"`
	Repo             string `json:"repo"`
	TargetCommit     string `json:"target_commit"`
	ReleaseBody      string `json:"release_body"`
}

func main() {
	cfg := settings{
		key: os.Getenv("OPENAI_API_KEY"), tag: os.Getenv("GITHUB_REF_NAME"),
		repo: os.Getenv("GITHUB_REPOSITORY"), commit: os.Getenv("GITHUB_SHA"),
		directory: os.Getenv("RUNNER_TEMP"),
	}
	run := func(ctx context.Context, name string, args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.WaitDelay = 5 * time.Second
		cmd.Stderr = os.Stderr
		out, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("%s failed: %w", name, err)
		}
		return strings.TrimSpace(string(out)), nil
	}
	// Optional failures deliberately do not set a failing process exit code.
	bestEffort(context.Background(), cfg, run, os.Stdout)
}

func bestEffort(ctx context.Context, cfg settings, run commandRunner, output io.Writer) {
	if err := enhance(ctx, cfg, run); err != nil {
		// GitHub workflow commands must stay on one line.
		message := strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(err.Error())
		_, _ = fmt.Fprintf(output, "::warning::%s; keeping git-cliff release notes.\n", message)
	}
}

func enhance(ctx context.Context, cfg settings, run commandRunner) error {
	if cfg.key == "" {
		return fmt.Errorf("OPENROUTER_API_KEY is not configured")
	}
	if cfg.tag == "" || cfg.repo == "" || cfg.commit == "" || cfg.directory == "" {
		return fmt.Errorf("missing CI release identity or temporary directory")
	}
	work := filepath.Join(cfg.directory, "editorial-notes")
	if err := os.MkdirAll(work, 0o700); err != nil {
		return err
	}
	// Installation, generation, and internal model retries share one deadline.
	generation, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	if _, err := run(generation, "mise", "install", toolVersion); err != nil {
		return err
	}
	binary, err := run(generation, "mise", "which", "communique", "--tool", toolVersion)
	if err != nil {
		return err
	}
	if binary == "" {
		return fmt.Errorf("communiqué executable not found")
	}
	path := filepath.Join(work, "draft.json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	if _, err := run(generation, binary, "generate", cfg.tag,
		"--provider", "openai", "--base-url", "https://openrouter.ai/api/v1",
		"--model", freeModel, "--draft", path,
		"--review-report", filepath.Join(work, "review.md"), "--preserve-sections",
		"--tag-pattern", "v[0-9]*.[0-9]*.[0-9]*", "--channel", "stable"); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := validateDraft(data, cfg); err != nil {
		return err
	}
	// Native publishing verifies the remote tag's commit and managed markers,
	// preserves surrounding factual notes/title, and makes no model request.
	publishing, cancelPublish := context.WithTimeout(ctx, 30*time.Second)
	defer cancelPublish()
	_, err = run(publishing, binary, "publish", path)
	return err
}

func validateDraft(data []byte, cfg settings) error {
	var value draft
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("invalid editorial draft: %w", err)
	}
	if value.SchemaVersion != 1 || !value.PreserveSections || value.Tag != cfg.tag ||
		value.Repo != cfg.repo || value.TargetCommit != cfg.commit {
		return fmt.Errorf("editorial draft identity or preservation settings do not match this release")
	}
	if strings.TrimSpace(value.ReleaseBody) == "" || len(value.ReleaseBody) > 20000 ||
		strings.Contains(value.ReleaseBody, "<!-- communique:") {
		return fmt.Errorf("editorial draft body is empty, oversized, or contains managed markers")
	}
	return nil
}
