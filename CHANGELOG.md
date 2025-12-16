# Changelog

All notable changes to RequestBite Proxy will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.0] - 2025-12-16

### Added
- Cross-platform binary releases for macOS (Intel & Apple Silicon), Linux x86, and Windows x86
- One-line installation script for automated setup on macOS and Linux
- Makefile for build automation with cross-compilation support
- Optimized static binaries with reduced size (~5 MB vs ~7.6 MB)
- SHA256 checksum verification for secure downloads
- Build metadata injection (version, build time, git commit)
- GitHub release automation script

### Changed
- Build process now uses Makefile instead of simple `go build`
- Binaries are now statically linked for better portability
- Distribution via GitHub releases with platform-specific archives

### Documentation
- Added comprehensive installation instructions
- Added platform support matrix
- Added CHANGELOG for version tracking
- Enhanced README with quick install guide

## [Unreleased]

### Previous Versions
Version 0.3.0 is the first release with automated cross-platform builds. Previous versions were built manually.
