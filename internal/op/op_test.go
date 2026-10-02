package op

import (
	"context"
	"strings"
	"testing"

	"github.com/1password/onepassword-sdk-go"
)

// fakeSecrets is a fake SecretsResolver recording the references it was
// asked to resolve.
type fakeSecrets struct {
	resp onepassword.ResolveAllResponse
	err  error
	refs []string
}

func (f *fakeSecrets) ResolveAll(_ context.Context, refs []string) (onepassword.ResolveAllResponse, error) {
	f.refs = refs
	return f.resp, f.err
}

type resolveResponse = onepassword.Response[onepassword.ResolvedReference, onepassword.ResolveReferenceError]

func okResponse(secret string) resolveResponse {
	return resolveResponse{Content: &onepassword.ResolvedReference{Secret: secret}}
}

func errResponse(t onepassword.ResolveReferenceErrorTypes) resolveResponse {
	return resolveResponse{Error: &onepassword.ResolveReferenceError{Type: t}}
}

func TestFetchDefaults(t *testing.T) {
	secrets := &fakeSecrets{resp: onepassword.ResolveAllResponse{IndividualResponses: map[string]resolveResponse{
		"op://Infra/AWS Production/username":   okResponse("AKIA1"),
		"op://Infra/AWS Production/credential": okResponse("secret1"),
	}}}
	fetcher := Fetcher{Secrets: secrets}

	creds, err := fetcher.Fetch(context.Background(), FetchOptions{
		Vault:          "Infra",
		Item:           "AWS Production",
		AccessKeyField: "username",
		SecretField:    "credential",
	})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if creds.AccessKeyID != "AKIA1" || creds.SecretAccessKey != "secret1" || creds.SessionToken != "" {
		t.Errorf("unexpected credentials: %+v", creds)
	}
	if len(secrets.refs) != 2 {
		t.Fatalf("expected 2 references, got %v", secrets.refs)
	}
}

func TestFetchSessionTokenField(t *testing.T) {
	secrets := &fakeSecrets{resp: onepassword.ResolveAllResponse{IndividualResponses: map[string]resolveResponse{
		"op://V/I/username":      okResponse("AKIA1"),
		"op://V/I/credential":    okResponse("secret1"),
		"op://V/I/session token": okResponse("token1"),
	}}}
	fetcher := Fetcher{Secrets: secrets}

	creds, err := fetcher.Fetch(context.Background(), FetchOptions{
		Vault:             "V",
		Item:              "I",
		AccessKeyField:    "username",
		SecretField:       "credential",
		SessionTokenField: "session token",
	})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if creds.SessionToken != "token1" {
		t.Errorf("unexpected session token: %+v", creds)
	}
	if len(secrets.refs) != 3 {
		t.Fatalf("expected 3 references, got %v", secrets.refs)
	}
}

func TestFetchFieldNotFoundMentionsOverrideFlags(t *testing.T) {
	secrets := &fakeSecrets{resp: onepassword.ResolveAllResponse{IndividualResponses: map[string]resolveResponse{
		"op://V/I/username":   errResponse(onepassword.ResolveReferenceErrorTypeVariantFieldNotFound),
		"op://V/I/credential": okResponse("secret1"),
	}}}
	fetcher := Fetcher{Secrets: secrets}

	_, err := fetcher.Fetch(context.Background(), FetchOptions{
		Vault: "V", Item: "I", AccessKeyField: "username", SecretField: "credential",
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{`"V"/"I"`, `"username"`, "--access-key-field"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestFetchAmbiguousItemSuggestsID(t *testing.T) {
	secrets := &fakeSecrets{resp: onepassword.ResolveAllResponse{IndividualResponses: map[string]resolveResponse{
		"op://V/I/username":   errResponse(onepassword.ResolveReferenceErrorTypeVariantTooManyItems),
		"op://V/I/credential": okResponse("s"),
	}}}
	fetcher := Fetcher{Secrets: secrets}

	_, err := fetcher.Fetch(context.Background(), FetchOptions{
		Vault: "V", Item: "I", AccessKeyField: "username", SecretField: "credential",
	})
	if err == nil || !strings.Contains(err.Error(), "item ID") {
		t.Errorf("expected guidance to pass the item ID, got %v", err)
	}
}

func TestFetchEmptyValue(t *testing.T) {
	secrets := &fakeSecrets{resp: onepassword.ResolveAllResponse{IndividualResponses: map[string]resolveResponse{
		"op://V/I/username":   okResponse(""),
		"op://V/I/credential": okResponse("s"),
	}}}
	fetcher := Fetcher{Secrets: secrets}

	_, err := fetcher.Fetch(context.Background(), FetchOptions{
		Vault: "V", Item: "I", AccessKeyField: "username", SecretField: "credential",
	})
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected empty-value error, got %v", err)
	}
}

func TestFetchMissingResponse(t *testing.T) {
	secrets := &fakeSecrets{resp: onepassword.ResolveAllResponse{IndividualResponses: map[string]resolveResponse{
		"op://V/I/credential": okResponse("s"),
	}}}
	fetcher := Fetcher{Secrets: secrets}

	_, err := fetcher.Fetch(context.Background(), FetchOptions{
		Vault: "V", Item: "I", AccessKeyField: "username", SecretField: "credential",
	})
	if err == nil || !strings.Contains(err.Error(), "no result") {
		t.Errorf("expected missing-result error, got %v", err)
	}
}

func TestNewClientRequiresAccountForDesktopAuth(t *testing.T) {
	t.Setenv(serviceAccountTokenEnv, "")
	_, err := NewClient(context.Background(), "", "test")
	if err == nil || !strings.Contains(err.Error(), "--account") {
		t.Errorf("expected missing-account error, got %v", err)
	}
}

func TestNewClientRejectsAccountWithServiceAccountToken(t *testing.T) {
	t.Setenv(serviceAccountTokenEnv, "some-token")
	_, err := NewClient(context.Background(), "my-account", "test")
	if err == nil {
		t.Fatal("expected an error when both a service account token and an account are set")
	}
	if !strings.Contains(err.Error(), serviceAccountTokenEnv) {
		t.Errorf("error %q does not mention %q", err, serviceAccountTokenEnv)
	}
}
