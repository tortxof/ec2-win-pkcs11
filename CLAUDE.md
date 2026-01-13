# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

ec2-win-pkcs11 is a Go CLI tool that decrypts EC2 Windows instance passwords using PKCS#11 hardware security tokens (e.g., Yubico PIV tokens). It bridges AWS EC2 password retrieval with hardware security module authentication.

## Build Commands

```bash
# Standard build (requires CGO for PKCS#11 bindings)
go build

# Dependency management
go mod tidy

# Multi-platform release builds (used by CI)
goreleaser release --clean
```

## Architecture

The entire application is in `main.go` (~250 lines):

- **main()** - Initializes CLI with urfave/cli, defines flags
- **run()** - Primary orchestrator that:
  1. Loads PKCS#11 library (platform-specific default paths)
  2. Discovers tokens and handles multi-token selection
  3. Prompts for PIN via secure terminal input
  4. Retrieves encrypted password from EC2 API
  5. Decrypts using RSA-PKCS mechanism with token's private key

## Key Dependencies

- `github.com/miekg/pkcs11` - PKCS#11 hardware token interface (requires CGO)
- `github.com/aws/aws-sdk-go-v2` - EC2 service for password retrieval
- `github.com/urfave/cli/v2` - CLI argument parsing
- `golang.org/x/term` - Secure PIN input

## Platform-Specific Defaults

PKCS#11 library paths:
- Linux: `/usr/lib/x86_64-linux-gnu/opensc-pkcs11.so`
- Windows: `C:\Program Files\Yubico\Yubico PIV Tool\bin\libykcs11.dll`

## Release Process

GoReleaser handles cross-compilation using Zig CC. Releases are triggered by pushing version tags (v*) via GitHub Actions.
