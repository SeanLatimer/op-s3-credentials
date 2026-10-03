package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SeanLatimer/op-s3-credentials/internal/op"
)

type fakeFetcher struct {
	gotOpts op.FetchOptions
	creds   op.Credentials
	err     error
}

func (f *fakeFetcher) Fetch(_ context.Context, opts op.FetchOptions) (op.Credentials, error) {
	f.gotOpts = opts
	return f.creds, f.err
}

// runCLI runs the CLI with a fake fetcher, returning the exit code, captured
// stdout/stderr, and the account value the fetcher factory received.
func runCLI(t *testing.T, args []string, fetcher CredentialFetcher) (int, string, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	var account string
	newFetcher := func(_ context.Context, acct string) (CredentialFetcher, error) {
		account = acct
		return fetcher, nil
	}
	code := Run(args, &stdout, &stderr, "test-version", newFetcher)
	return code, stdout.String(), stderr.String(), account
}

func TestSuccessEmitsJSON(t *testing.T) {
	f := &fakeFetcher{creds: op.Credentials{AccessKeyID: "AKIA1", SecretAccessKey: "secret1"}}
	code, stdout, stderr, _ := runCLI(t,
		[]string{"op-s3-credentials", "--vault", "Infrastructure", "--item", "AWS Production", "--timeout", "30s"}, f)

	if code != 0 {
		t.Fatalf("exit code %d, stderr: %q", code, stderr)
	}
	want := `{"Version":1,"AccessKeyId":"AKIA1","SecretAccessKey":"secret1"}` + "\n"
	if stdout != want {
		t.Errorf("stdout mismatch\n got: %q\nwant: %q", stdout, want)
	}
	if stderr != "" {
		t.Errorf("expected empty stderr, got %q", stderr)
	}
	if f.gotOpts.Vault != "Infrastructure" || f.gotOpts.Item != "AWS Production" {
		t.Errorf("unexpected fetch options: %+v", f.gotOpts)
	}
	if f.gotOpts.AccessKeyField != "username" || f.gotOpts.SecretField != "credential" {
		t.Errorf("default field names not applied: %+v", f.gotOpts)
	}
}

func TestSessionTokenFlagEndsUpInJSON(t *testing.T) {
	f := &fakeFetcher{creds: op.Credentials{AccessKeyID: "a", SecretAccessKey: "s", SessionToken: "tok"}}
	code, stdout, stderr, _ := runCLI(t,
		[]string{"op-s3-credentials", "--vault", "V", "--item", "I", "--session-token-field", "session token"}, f)

	if code != 0 {
		t.Fatalf("exit code %d, stderr: %q", code, stderr)
	}
	if !strings.Contains(stdout, `"SessionToken":"tok"`) {
		t.Errorf("session token missing from output: %q", stdout)
	}
	if f.gotOpts.SessionTokenField != "session token" {
		t.Errorf("session token field not passed through: %+v", f.gotOpts)
	}
}

func TestAccountFromEnvAndFlagPrecedence(t *testing.T) {
	t.Setenv("OP_ACCOUNT", "from-env")

	f := &fakeFetcher{creds: op.Credentials{AccessKeyID: "a", SecretAccessKey: "s"}}
	code, _, _, account := runCLI(t,
		[]string{"op-s3-credentials", "--vault", "V", "--item", "I"}, f)
	if code != 0 || account != "from-env" {
		t.Fatalf("code=%d account=%q", code, account)
	}

	code, _, _, account = runCLI(t,
		[]string{"op-s3-credentials", "--vault", "V", "--item", "I", "--account", "from-flag"}, f)
	if code != 0 || account != "from-flag" {
		t.Fatalf("flag should beat env: code=%d account=%q", code, account)
	}
}

func TestMissingRequiredFlagFails(t *testing.T) {
	f := &fakeFetcher{creds: op.Credentials{AccessKeyID: "a", SecretAccessKey: "s"}}
	code, _, stderr, _ := runCLI(t, []string{"op-s3-credentials", "--item", "I"}, f)
	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if !strings.Contains(stderr, "vault") {
		t.Errorf("stderr should mention the missing flag: %q", stderr)
	}
}

func TestFetcherErrorGoesToStderr(t *testing.T) {
	f := &fakeFetcher{err: errors.New("1Password item \"V\"/\"I\", field \"username\": not found")}
	code, stdout, stderr, _ := runCLI(t,
		[]string{"op-s3-credentials", "--vault", "V", "--item", "I"}, f)
	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if stdout != "" {
		t.Errorf("expected empty stdout on failure, got %q", stdout)
	}
	if !strings.Contains(stderr, "not found") {
		t.Errorf("stderr should contain the fetcher error: %q", stderr)
	}
}

func TestFetcherFactoryError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	newFetcher := func(context.Context, string) (CredentialFetcher, error) {
		return nil, errors.New("cannot connect to 1Password")
	}
	code := Run([]string{"op-s3-credentials", "--vault", "V", "--item", "I"}, &stdout, &stderr, "test", newFetcher)
	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if !strings.Contains(stderr.String(), "cannot connect to 1Password") {
		t.Errorf("stderr should contain the factory error: %q", stderr.String())
	}
}

func TestVersionFlag(t *testing.T) {
	f := &fakeFetcher{creds: op.Credentials{AccessKeyID: "a", SecretAccessKey: "s"}}
	code, stdout, _, _ := runCLI(t, []string{"op-s3-credentials", "--version"}, f)
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	if !strings.Contains(stdout, "test-version") {
		t.Errorf("stdout should contain the version: %q", stdout)
	}
}
