// Package cli wires the command-line interface for op-s3-credentials.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/SeanLatimer/op-s3-credentials/internal/awsprocess"
	"github.com/SeanLatimer/op-s3-credentials/internal/op"
)

// CredentialFetcher resolves AWS credentials from 1Password. It is an
// interface so tests can inject fakes.
type CredentialFetcher interface {
	Fetch(ctx context.Context, opts op.FetchOptions) (op.Credentials, error)
}

// NewFetcherFunc builds a CredentialFetcher for the given 1Password account
// (empty string for the default account or service-account authentication).
type NewFetcherFunc func(ctx context.Context, account string) (CredentialFetcher, error)

// Run executes the CLI and returns the process exit code. All diagnostics go
// to stderr; stdout receives only the credential_process JSON, and only on
// success.
func Run(args []string, stdout, stderr io.Writer, version string, newFetcher NewFetcherFunc) int {
	cmd := buildCommand(stdout, stderr, version, newFetcher)
	// Take full control of error reporting and exit codes so the library
	// never prints or calls os.Exit on its own.
	cmd.ExitErrHandler = func(context.Context, *cli.Command, error) {}

	err := cmd.Run(context.Background(), args)
	if err == nil {
		return 0
	}
	var exitErr cli.ExitCoder
	if errors.As(err, &exitErr) {
		if msg := err.Error(); msg != "" {
			_, _ = fmt.Fprintln(stderr, msg)
		}
		return exitErr.ExitCode()
	}
	_, _ = fmt.Fprintf(stderr, "op-s3-credentials: %v\n", err)
	return 1
}

// armWatchdog terminates the process with an actionable message once the
// given duration elapses. The returned func disarms it; call it when the run
// completes normally.
func armWatchdog(stderr io.Writer, d time.Duration) (stop func()) {
	t := time.AfterFunc(d, func() {
		_, _ = fmt.Fprintf(stderr, "op-s3-credentials: timed out after %s while contacting 1Password; "+
			"is the desktop app running and unlocked, with Settings > Developer > "+
			"\"Integrate with other apps\" enabled? (OP_SERVICE_ACCOUNT_TOKEN auth needs no app.)\n", d)
		os.Exit(1)
	})
	return func() { t.Stop() }
}

func buildCommand(stdout, stderr io.Writer, version string, newFetcher NewFetcherFunc) *cli.Command {
	return &cli.Command{
		Name:  "op-s3-credentials",
		Usage: "emit AWS credential_process JSON from a 1Password API Credential item",
		UsageText: "op-s3-credentials --vault VAULT --item ITEM [--account ACCOUNT] " +
			"[--access-key-field F] [--secret-field F] [--session-token-field F] [--timeout 60s]",
		Description: `Reads AWS credentials from a 1Password "API Credential" item
(username -> access key ID, credential -> secret access key) and prints them in
the JSON format expected by the AWS CLI/SDKs credential_process setting.

Configure it in ~/.aws/config:

  [profile production]
  credential_process = "C:\Tools\op-s3-credentials.exe" --vault Infrastructure --item "AWS Production" --account xxxxxxx`,
		Version:   version,
		Writer:    stdout,
		ErrWriter: stderr,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "vault",
				Aliases:  []string{"v"},
				Usage:    "1Password vault name or ID holding the item",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "item",
				Aliases:  []string{"i"},
				Usage:    "1Password item name or ID (an API Credential item)",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "account",
				Aliases: []string{"a"},
				Usage: "1Password account name or UUID for desktop app auth " +
					"(env: OP_ACCOUNT; must be unset when OP_SERVICE_ACCOUNT_TOKEN is set)",
				Sources: cli.EnvVars("OP_ACCOUNT"),
			},
			&cli.StringFlag{
				Name:  "access-key-field",
				Usage: "item field holding the AWS access key ID",
				Value: "username",
			},
			&cli.StringFlag{
				Name:  "secret-field",
				Usage: "item field holding the AWS secret access key",
				Value: "credential",
			},
			&cli.StringFlag{
				Name:  "session-token-field",
				Usage: "item field holding an AWS session token (omit unless the item stores one)",
			},
			&cli.DurationFlag{
				Name:  "timeout",
				Usage: "overall time limit for contacting 1Password, e.g. 30s or 1m",
				Value: 60 * time.Second,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if d := cmd.Duration("timeout"); d > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, d)
				defer cancel()
				// The 1Password desktop app integration can block in
				// native code that ignores context cancellation, so back
				// the soft deadline with a hard watchdog that terminates
				// the process no matter what.
				defer armWatchdog(stdout, d+5*time.Second)()
			}

			fetcher, err := newFetcher(ctx, cmd.String("account"))
			if err != nil {
				return err
			}

			creds, err := fetcher.Fetch(ctx, op.FetchOptions{
				Vault:             cmd.String("vault"),
				Item:              cmd.String("item"),
				AccessKeyField:    cmd.String("access-key-field"),
				SecretField:       cmd.String("secret-field"),
				SessionTokenField: cmd.String("session-token-field"),
			})
			if err != nil {
				return err
			}

			return awsprocess.Write(stdout, creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken)
		},
	}
}
