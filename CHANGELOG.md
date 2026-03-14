# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-03-14

### Added
- `helm template` invocation with full forwarding of `--values`/`-f`, `--set`, and `--namespace` flags
- Recursive YAML node tree walker that finds every container image reference regardless of nesting depth (containers, initContainers, sidecars, Jobs, etc.)
- Image reference parser handling tags (`nginx:1.25`), digest refs (`nginx@sha256:…`), registry host:port colons, and untagged images (normalized to `latest`)
- `--output`/`-o` flag supporting `table`, `json`, and `csv` output formats
- `--unique`/`-u` flag to deduplicate by image+tag, keeping first occurrence
- Per-image attribution: kind, namespace, resource name, `helm.sh/chart` label, and container name

[Unreleased]: https://github.com/kylegalloway/helm-image-finder/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/kylegalloway/helm-image-finder/releases/tag/v0.1.0
