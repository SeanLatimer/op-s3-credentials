# Changelog

All notable changes to this project are documented here.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- AWS `credential_process` helper backed by the official 1Password Go SDK.
- Desktop-app and service-account authentication, configurable credential fields,
  and an explicitly selected optional session-token field.
- A hard timeout for SDK operations, including blocked desktop-app handshakes.
