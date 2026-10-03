# op-s3-credentials

A tiny Windows-first CLI that feeds AWS credentials from [1Password](https://1password.com)
to the AWS CLI and SDKs via the shared-config `credential_process` hook.

It reads a **1Password "API Credential" item** and prints the JSON document the
AWS CLI/SDKs expect on stdout:

| 1Password field | credential_process key | default field name |
| --- | --- | --- |
| `username` | `AccessKeyId` | `username` |
| `credential` | `SecretAccessKey` | `credential` |
| *(optional)* | `SessionToken` | only read if `--session-token-field` is set |

Built on the official [1Password Go SDK](https://github.com/1Password/onepassword-sdk-go)
and [urfave/cli v3](https://cli.urfave.org). No credential caching, nothing
written to disk.

## Installation

Prebuilt binaries are published for Linux, macOS, and Windows (amd64 and
arm64):

- **Homebrew**: `brew install SeanLatimer/tap/op-s3-credentials`
- **Scoop**: `scoop bucket add SeanLatimer https://github.com/SeanLatimer/scoop-bucket && scoop install op-s3-credentials`
- **mise**: `mise use -g ubi:SeanLatimer/op-s3-credentials`
- **Manual**: download from [releases](https://github.com/SeanLatimer/op-s3-credentials/releases) — archives are named `op-s3-credentials_<os>_<arch>`

Or from source: `go install github.com/SeanLatimer/op-s3-credentials/cmd/op-s3-credentials@latest`.

## 1Password setup

1. Install the 1Password desktop app, sign in, and enable
   **Settings → Developer → "Integrate with other apps"** (under *Integrate
   with the 1Password SDKs*). Biometric unlock (Windows Hello) is optional but
   recommended.
2. Create an API Credential item holding your AWS keys:

   ```
   op item create --category="API Credential" --title "AWS Production" --vault Infrastructure \
     username=AKIAIOSFODNN7EXAMPLE 'credential=wJalr...EXAMPLEKEY'
   ```

## AWS config

In `~/.aws/config` (note: because the command contains spaces, the whole
command is quoted and args with spaces use inner quotes, per the
[AWS docs](https://docs.aws.amazon.com/sdkref/latest/guide/feature-process-credentials.html)):

```ini
[profile production]
credential_process = "C:\Tools\op-s3-credentials.exe" --vault Infrastructure --item "AWS Production" --account my-account
```

Then verify with:

```
aws sts get-caller-identity --profile production
```

`--vault` and `--item` accept names or unique IDs. If names are ambiguous the
tool tells you to pass IDs instead.

## Authentication

Two modes, picked automatically:

- **Desktop app** (default): talks to the 1Password desktop app — you may get
  a Windows Hello / approval prompt when the app is locked. Requires
  `--account` (or `OP_ACCOUNT`): the account name shown at the top of the
  app's sidebar, or the account UUID. The SDK does not auto-select an account,
  so this is required, not cosmetic.
  **Gotcha:** `OP_ACCOUNT` does not accept the *user* UUID. This is a
  1Password Go SDK limitation, not a choice in this tool: the SDK matches
  accounts by sidebar name or account UUID only (`Account not found`
  otherwise), and every SDK-based integration inherits that strictness —
  including the onepassword Terraform provider v3.x. Only op-CLI-backed tools
  (the op CLI itself, Terraform provider v1/v2) accept user UUIDs, emails, and
  URLs — which is why the user UUID is the identifier people end up with:
  `op account list` prints the *user* UUID in its default output and hides the
  account UUID unless `--format json` is used. Run
  `op account list --format json` and use the `account_uuid` (an explicit
  `--account` flag also overrides the environment variable).
- **Service account**: set `OP_SERVICE_ACCOUNT_TOKEN` to authenticate for CI
  and automation. Service accounts belong to a single 1Password account
  already, so `--account` must be unset in this mode.

## Usage

```
op-s3-credentials --vault VAULT --item ITEM [--account ACCOUNT]
                  [--access-key-field F] [--secret-field F]
                  [--session-token-field F] [--timeout 60s] [--version]
```

| Flag | Env | Default | Description |
| --- | --- | --- | --- |
| `--vault`, `-v` | | *required* | vault name or ID |
| `--item`, `-i` | | *required* | item name or ID |
| `--account`, `-a` | `OP_ACCOUNT` | | 1Password account name or UUID (desktop auth) |
| `--access-key-field` | | `username` | field holding the AWS access key ID |
| `--secret-field` | | `credential` | field holding the AWS secret access key |
| `--session-token-field` | | | field holding an AWS session token, if the item stores one |
| `--timeout` | | `60s` | hard limit for contacting 1Password (see below) |

All diagnostics go to stderr and set a non-zero exit code; stdout only ever
carries the credential JSON, which is what `credential_process` requires.

### Timeouts and hangs

The desktop-app handshake can block in native code (e.g. the app is not
running, is locked behind a prompt you never see, or the developer integration
is disabled). `--timeout` is enforced twice: as a context deadline on the SDK
calls, and as a watchdog that terminates the process with an explanatory
message even if the block ignores cancellation — so the AWS CLI never waits on
a stuck helper forever.

## Verifying releases

Every release ships `checksums.txt`, a keyless Cosign signature bundle
(`checksums.txt.sigstore.json`), and a
`packslip.sigstore.json` manifest — all signed by this repository's GitHub
Actions identity:

```sh
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity "https://github.com/SeanLatimer/op-s3-credentials/.github/workflows/release.yml@refs/tags/v0.1.1" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  checksums.txt
```

Use the tag of the release you downloaded in the certificate identity. After
signature verification succeeds, compare each downloaded archive's SHA-256 hash
against its entry in `checksums.txt`.

mise reads the packslip manifest directly when installing, selecting and
verifying artifacts from the signed digests instead of filename conventions.

## Development

Tooling is managed with [mise](https://mise.jdx.dev):

```
mise run build          # debug binary into bin/
mise run snapshot       # local release snapshot (full matrix requires macOS)
mise run test           # unit tests
mise run lint           # golangci-lint
mise run fmt            # gofmt
mise run install        # go install
mise run release:preview # propose version/changelog without writing
mise run release:prepare # write changelog only; never publishes
```

Building for Windows needs no C toolchain (no CGO). Cross-compiling the
desktop-app integration for Linux/macOS requires CGO; a plain
`CGO_ENABLED=0` linux build also works but only supports service-account
auth.

See [the release guide](docs/releasing.md) for version policy, signed tags,
publication, and optional free editorial release notes.
