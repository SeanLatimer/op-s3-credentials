// Command op-s3-credentials prints AWS CLI credential_process JSON for
// credentials stored in a 1Password item.
package main

import (
	"context"
	"os"

	"github.com/SeanLatimer/op-s3-credentials/internal/cli"
	"github.com/SeanLatimer/op-s3-credentials/internal/op"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	newFetcher := func(ctx context.Context, account string) (cli.CredentialFetcher, error) {
		client, err := op.NewClient(ctx, account, version)
		if err != nil {
			return nil, err
		}
		return op.Fetcher{Secrets: client.Secrets()}, nil
	}

	os.Exit(cli.Run(os.Args, os.Stdout, os.Stderr, version, newFetcher))
}
