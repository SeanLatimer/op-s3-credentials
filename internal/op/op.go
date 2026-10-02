// Package op resolves AWS credentials from 1Password items using the
// official 1Password SDK.
package op

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/1password/onepassword-sdk-go"
)

// serviceAccountTokenEnv is the environment variable conventionally used by
// the 1Password SDKs for service account (non-interactive) authentication.
const serviceAccountTokenEnv = "OP_SERVICE_ACCOUNT_TOKEN"

// NewClient builds a 1Password SDK client. It authenticates with a service
// account when serviceAccountTokenEnv is set (CI and other automation), and
// otherwise through the 1Password desktop app (Windows Hello, Touch ID, ...)
// using the given account name or UUID. An empty account is passed through
// for the desktop app to resolve, which works when it is signed in to a
// single account.
func NewClient(ctx context.Context, account, integrationVersion string) (*onepassword.Client, error) {
	opts := []onepassword.ClientOption{
		onepassword.WithIntegrationInfo("op-s3-credentials", integrationVersion),
	}
	if token := os.Getenv(serviceAccountTokenEnv); token != "" {
		if account != "" {
			return nil, fmt.Errorf(
				"%s is set while an account was also given (%q): service accounts already belong to a single 1Password account, so unset --account/OP_ACCOUNT or unset %s",
				serviceAccountTokenEnv, account, serviceAccountTokenEnv)
		}
		opts = append(opts, onepassword.WithServiceAccountToken(token))
	} else {
		if account == "" {
			return nil, errors.New("no 1Password account specified: pass --account or set OP_ACCOUNT " +
				"(the account name shown at the top of the 1Password desktop app sidebar, or its UUID)")
		}
		opts = append(opts, onepassword.WithDesktopAppIntegration(account))
	}
	return onepassword.NewClient(ctx, opts...)
}

// SecretsResolver is the slice of the 1Password SDK client that Fetch needs.
// It is satisfied by the SDK client's Secrets() API and by fakes in tests.
type SecretsResolver interface {
	ResolveAll(ctx context.Context, secretReferences []string) (onepassword.ResolveAllResponse, error)
}

// Fetcher resolves AWS credentials from a 1Password item.
type Fetcher struct {
	Secrets SecretsResolver
}

// FetchOptions identifies the item to read and which of its fields hold the
// AWS credentials. Vault and Item accept names or unique IDs.
type FetchOptions struct {
	Vault             string
	Item              string
	AccessKeyField    string
	SecretField       string
	SessionTokenField string // optional; empty means no session token
}

// Credentials holds the AWS credential values read from a 1Password item.
type Credentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string // empty unless FetchOptions.SessionTokenField was set
}

// fieldTarget pairs a field name on the item with the credential it feeds.
type fieldTarget struct {
	field string
	dst   *string
}

// Fetch reads the configured fields from the item and returns them as AWS
// credentials. Fields are resolved by name via secret references of the form
// op://<vault>/<item>/<field>.
func (f Fetcher) Fetch(ctx context.Context, opts FetchOptions) (Credentials, error) {
	var creds Credentials
	targets := []fieldTarget{
		{opts.AccessKeyField, &creds.AccessKeyID},
		{opts.SecretField, &creds.SecretAccessKey},
	}
	if opts.SessionTokenField != "" {
		targets = append(targets, fieldTarget{opts.SessionTokenField, &creds.SessionToken})
	}

	refs := make([]string, len(targets))
	for i, t := range targets {
		refs[i] = fmt.Sprintf("op://%s/%s/%s", opts.Vault, opts.Item, t.field)
	}

	resp, err := f.Secrets.ResolveAll(ctx, refs)
	if err != nil {
		return Credentials{}, fmt.Errorf("resolving secrets from 1Password: %w", err)
	}

	for i, t := range targets {
		secret, err := secretFrom(resp, refs[i])
		if err != nil {
			return Credentials{}, fmt.Errorf("1Password item %q/%q, field %q: %w",
				opts.Vault, opts.Item, t.field, err)
		}
		*t.dst = secret
	}
	return creds, nil
}

// secretFrom extracts the resolved secret for a single reference from a
// ResolveAll response.
func secretFrom(resp onepassword.ResolveAllResponse, ref string) (string, error) {
	r, ok := resp.IndividualResponses[ref]
	if !ok {
		return "", fmt.Errorf("1Password returned no result for %s", ref)
	}
	if r.Error != nil {
		return "", resolveError(r.Error)
	}
	if r.Content == nil || r.Content.Secret == "" {
		return "", errors.New("the field is empty")
	}
	return r.Content.Secret, nil
}

// resolveError turns the SDK's typed resolve errors into actionable guidance.
func resolveError(e *onepassword.ResolveReferenceError) error {
	switch e.Type {
	case onepassword.ResolveReferenceErrorTypeVariantVaultNotFound:
		return errors.New("no vault matched; check the vault name, or pass the vault ID instead")
	case onepassword.ResolveReferenceErrorTypeVariantTooManyVaults:
		return errors.New("multiple vaults match this name; pass the vault ID instead")
	case onepassword.ResolveReferenceErrorTypeVariantItemNotFound:
		return errors.New("no item matched; check the item name, or pass the item ID instead")
	case onepassword.ResolveReferenceErrorTypeVariantTooManyItems:
		return errors.New("multiple items match this name; pass the item ID instead")
	case onepassword.ResolveReferenceErrorTypeVariantFieldNotFound:
		return errors.New(`not found on the item; expected an "API Credential" item, or override with --access-key-field/--secret-field/--session-token-field`)
	case onepassword.ResolveReferenceErrorTypeVariantTooManyMatchingFields:
		return errors.New("multiple fields match this name; use a unique field name or the field ID")
	case onepassword.ResolveReferenceErrorTypeVariantNoMatchingSections:
		return errors.New("no matching section on the item")
	default:
		return fmt.Errorf("1Password error: %s", e.Type)
	}
}
