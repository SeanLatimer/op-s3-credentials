# Releasing

Release preparation is automated; the decision to ship remains manual.
**Preparing a changelog does not create a commit, tag, push, or release.**

## Responsibilities

| Tool | Responsibility |
| --- | --- |
| git-cliff | Propose a version and generate factual changelog/release notes |
| Maintainer | Review changes, sign commits and the release tag, approve publication |
| GoReleaser | Build archives, sign checksums, publish, update Homebrew/Scoop |
| packslip | Publish signed artifact metadata |
| Communiqué | Optionally append editorial notes to the GitHub release |

`CHANGELOG.md` is the reviewed, version-controlled record. Generate its next entry
on `develop` when preparing a release, not on every commit. Historical entries
are preserved by preparation; edit the proposed entry to clarify user impact.
GitHub release notes are independently generated from the tagged commit range.

## Version policy

Use Conventional Commits for individual changes:

- `fix` / `perf`: patch.
- `feat`: minor.
- Breaking changes (`!` and a descriptive `BREAKING CHANGE:` footer): minor
  during `0.x`, major after `1.0`.
- Routine `chore`, `ci`, `docs`, `test`, `style`, `build`, and `refactor` changes
  alone do not trigger a bump. Breaking changes must still be marked.
- The first release is `v0.1.0`. Graduation to `v1.0.0` is an explicit maintainer
  decision, not an automatic consequence of a pre-1.0 breaking change.

Stable `vMAJOR.MINOR.PATCH` tags are supported. Prereleases are not automated by
this preparation workflow. GoReleaser injects the tag-derived version into the
binary; no separate source version or `go.mod` version needs updating.

## One-time setup

- Public `SeanLatimer/homebrew-tap` and `SeanLatimer/scoop-bucket` repositories,
  each initialized with a default branch.
- `TAP_GITHUB_TOKEN` Actions secret: fine-grained PAT with Contents read/write on
  those two repositories. The automatic `GITHUB_TOKEN` publishes this repo's
  release. Release jobs also need `id-token: write` for keyless signing.
- Optional `OPENROUTER_API_KEY` Actions secret for free editorial notes. No paid
  model is configured. Review OpenRouter/provider data-handling settings before
  sending repository context. Missing credentials simply skip AI.
- Go, Git, and mise locally (Windows, macOS, or Linux). Tool tasks fetch pinned git-cliff
  on demand. Keep your existing SSH commit-signing configuration enabled; never
  fall back to unsigned commits after a denied approval prompt.

## Prepare and review

1. On `develop`, commit the intended changes with SSH signing and fetch/pull
   your branch as appropriate. Start preparation from a clean working tree.
2. Preview the proposed version and changelog:

   ```sh
   mise run release:preview
   ```

   For an offline preview, append `-- -no-fetch`. To override the proposed version,
   append `-- -version v0.2.0` (or the version you intend).
3. Write the release entry:

   ```sh
   mise run release:prepare
   # Explicit stability decision, when appropriate:
   # mise run release:prepare -- -version v1.0.0
   ```

   The Go helper fetches tags, rejects existing/older versions, replaces the
   Unreleased section with a dated entry, and keeps prior release entries intact.
   If there are only routine changes, the proposed existing tag is rejected;
   either do not release or consciously override to a newer version.
4. Review `CHANGELOG.md`, especially breaking changes and upgrade instructions.
   Run `mise run test` and `mise run lint`, then create an SSH-signed conventional
   commit for release preparation. Do not stage generated binaries or `dist/`.
5. Open and review `develop` → `main`. Wait for CI. Preserve individual
   Conventional Commits (do not squash an entire release into one maintenance
   commit), so tag-based changelog generation can still see the real changes.

Preparation only supports a clean tree for writes. If you need to redo a written
entry before committing, preview again and edit the entry deliberately rather
than overwriting unrelated work.

## Ship (manual publication gate)

Only after approval and merge, update local `main`. Verify its commit and the
chosen version, then create a **signed annotated tag** and push that tag:

```sh
git switch main
git pull --ff-only
git tag -s v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```

Replace `v0.1.0` with the reviewed version. **The tag push publishes the release.**
The workflow rejects tags whose target is not an ancestor of remote `main`.
Tags are not created by CI. If this is automated later, note that tags created
with a workflow's `GITHUB_TOKEN` generally do not trigger a second tag workflow;
use explicit orchestration or an appropriately scoped GitHub App instead.

Afterward, bring any changes committed only on `main` back into `develop` before
continuing development. Do not regenerate reviewed historical changelog entries.

## Optional editorial notes and fallback

The main job publishes git-cliff notes, binaries, checksums, signatures, package
manifests, and packslip metadata. Build/signing/publication failures remain real
failures. A separate job starts only after that job succeeds:

- Uses pinned Communiqué through OpenRouter's OpenAI-compatible endpoint.
- Requests exactly `nvidia/nemotron-3-ultra-550b-a55b:free`; **no paid fallback**.
- Maps `OPENROUTER_API_KEY` to Communiqué's `OPENAI_API_KEY`.
- Allows three minutes for tool installation and generation (including retries).
- Generates a draft, validates its repository/tag/commit and nonempty body, then
  appends a marked editorial section. Factual notes and installation/verification
  links remain intact. Retries replace only the marked section.
- Missing secrets, rate limits, unavailable endpoints, malformed responses,
  timeouts, or update failures leave the factual notes in place and issue a
  warning. Failures in the optional job do not fail distribution.

AI never chooses versions or edits `CHANGELOG.md`. Basic validation cannot prove
factual accuracy: review early outputs, especially authentication/security claims
and breaking changes. If a summary is wrong, remove its marked section through
the GitHub release editor; leave the factual notes intact. Free endpoint
availability and quotas may change, and generation can consume multiple requests.

## Verify and recover

- Confirm six archives (Linux/macOS/Windows × amd64/arm64), `checksums.txt`,
  `checksums.txt.sigstore.json`, and `packslip.sigstore.json` exist on the release.
- Follow the README's signature-verification instructions and smoke-test the
  binary's `--version` and the actual credential-process integration.
- Verify Homebrew/Scoop manifests and test `mise use -g ubi:SeanLatimer/op-s3-credentials`.
- Confirm the CLI version matches the reviewed tag. Review the final release body.
- If macOS/CGO builds fail, inspect the macOS runner log; do not drop platforms
  silently. If tap/bucket publishing fails, check token expiry and Contents access.
- A failed job may have already published some artifacts or manifests. Inspect
  the release and package repositories before retrying; do not move/delete a
  published tag or blindly rerun the entire publishing job. Repair missing pieces
  deliberately, or ship a new patch tag when source changes are necessary.
- Editorial-only failures require no release repair: git-cliff notes are already
  present. Retrying only the editorial job can replace its managed section.
