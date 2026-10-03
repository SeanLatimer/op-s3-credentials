package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBestEffort(t *testing.T) {
	for _, scenario := range []string{
		"success", "missing-key", "install-failure", "generation-failure", "timeout",
		"missing-draft", "invalid-json", "empty", "oversized", "markers", "wrong-tag",
		"wrong-repo", "wrong-commit", "wrong-schema", "no-preservation", "publish-failure",
	} {
		t.Run(scenario, func(t *testing.T) {
			cfg := settings{key: "not-a-real-key", tag: "v0.1.0", repo: "SeanLatimer/op-s3-credentials",
				commit: strings.Repeat("a", 40), directory: t.TempDir()}
			if scenario == "missing-key" {
				cfg.key = ""
			}
			published := false
			run := func(ctx context.Context, name string, args ...string) (string, error) {
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("external call has no deadline")
				}
				if name == "mise" {
					if args[0] == "install" {
						if scenario == "install-failure" {
							return "", fmt.Errorf("install failed")
						}
						return "", nil
					}
					return "fake-communique", nil
				}
				if name != "fake-communique" {
					t.Fatalf("unexpected external tool: %s", name)
				}
				if args[0] == "publish" {
					if scenario == "publish-failure" {
						return "", fmt.Errorf("update failed")
					}
					published = true
					return "", nil
				}
				if !strings.Contains(strings.Join(args, " "), "--model "+freeModel) {
					t.Fatal("generation must request exactly the free model")
				}
				if scenario == "generation-failure" {
					return "", fmt.Errorf("model unavailable")
				}
				if scenario == "timeout" {
					return "", context.DeadlineExceeded
				}
				if scenario == "missing-draft" {
					return "", nil
				}
				value := draft{SchemaVersion: 1, PreserveSections: true, Tag: cfg.tag,
					Repo: cfg.repo, TargetCommit: cfg.commit, ReleaseBody: "Verified changes."}
				switch scenario {
				case "empty":
					value.ReleaseBody = " \n"
				case "oversized":
					value.ReleaseBody = strings.Repeat("a", 20001)
				case "markers":
					value.ReleaseBody = "<!-- communique:start -->"
				case "wrong-tag":
					value.Tag = "v9.9.9"
				case "wrong-repo":
					value.Repo = "other/repo"
				case "wrong-commit":
					value.TargetCommit = strings.Repeat("b", 40)
				case "wrong-schema":
					value.SchemaVersion = 2
				case "no-preservation":
					value.PreserveSections = false
				}
				data, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				if scenario == "invalid-json" {
					data = []byte("not json")
				}
				return "", os.WriteFile(filepath.Join(cfg.directory, "editorial-notes", "draft.json"), data, 0o600)
			}
			var output bytes.Buffer
			bestEffort(context.Background(), cfg, run, &output)
			if published != (scenario == "success") {
				t.Fatalf("published = %v", published)
			}
			if scenario != "success" && !strings.Contains(output.String(), "keeping git-cliff release notes") {
				t.Fatal("optional failure must warn without blocking")
			}
		})
	}
}
