# ec2-win-pkcs11

A CLI tool for decrypting EC2 Windows instance passwords using PKCS#11 hardware security tokens (e.g., YubiKey PIV).

## Overview

When you launch a Windows EC2 instance, AWS encrypts the administrator password with the public key you specified at launch. Normally, you would decrypt this password using the private key file. This tool allows you to decrypt the password using a private key stored on a PKCS#11-compatible hardware token, such as a YubiKey with PIV credentials.

## Features

- Decrypt EC2 Windows passwords using hardware security tokens
- Support for multiple AWS profiles
- Automatic token detection with multi-token selection support
- Secure PIN entry (hidden input)
- Cross-platform support (Linux and Windows)

## Prerequisites

- A PKCS#11-compatible hardware token (e.g., YubiKey with PIV)
- The token must contain the private key corresponding to the EC2 key pair
- PKCS#11 library installed:
  - **Linux**: OpenSC (`opensc-pkcs11.so`)
  - **Windows**: Yubico PIV Tool (`libykcs11.dll`)
- AWS credentials configured with `ec2:GetPasswordData` permission

## Installation

### Pre-built Binaries

Download the latest release for your platform from the [Releases](https://github.com/tortxof/ec2-win-pkcs11/releases) page.

### Build from Source

Requires Go 1.25+ and CGO (for PKCS#11 bindings).

```bash
git clone https://github.com/tortxof/ec2-win-pkcs11.git
cd ec2-win-pkcs11
go build
```

## Usage

```bash
ec2-win-pkcs11 -i <instance-id> [options]
```

### Options

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--instance-id` | `-i` | EC2 instance ID (required) | - |
| `--profile` | `-p` | AWS profile name | `default` |
| `--token` | `-t` | Token serial number (for multi-token setups) | - |
| `--lib` | `-l` | PKCS#11 library path | Platform default |

### Examples

Basic usage with default AWS profile:

```bash
ec2-win-pkcs11 -i i-0123456789abcdef0
```

Using a specific AWS profile:

```bash
ec2-win-pkcs11 -i i-0123456789abcdef0 -p my-aws-profile
```

Specifying a token when multiple are connected:

```bash
ec2-win-pkcs11 -i i-0123456789abcdef0 -t 12345678
```

Using a custom PKCS#11 library:

```bash
ec2-win-pkcs11 -i i-0123456789abcdef0 -l /path/to/pkcs11.so
```

### Output

The tool will:

1. Detect available PKCS#11 tokens
2. Prompt for your PIN
3. Retrieve the encrypted password from EC2
4. Decrypt and display the Windows administrator password

If multiple tokens are detected and none is specified, the tool lists available token serial numbers.

## How It Works

1. Loads the PKCS#11 library and initializes connection to the hardware token
2. Discovers available tokens and selects the appropriate one
3. Retrieves the encrypted password data from EC2 using the AWS SDK
4. Prompts for the token PIN via secure terminal input
5. Opens a PKCS#11 session and authenticates with the PIN
6. Locates the PIV Authentication key (slot 9a / ID 0x01)
7. Decrypts the password using RSA-PKCS mechanism
8. Outputs the decrypted password

## Platform-Specific Notes

### Linux

Default PKCS#11 library path: `/usr/lib/x86_64-linux-gnu/opensc-pkcs11.so`

Install OpenSC:

```bash
# Debian/Ubuntu
sudo apt install opensc-pkcs11

# Fedora/RHEL
sudo dnf install opensc
```

### Windows

Default PKCS#11 library path: `C:\Program Files\Yubico\Yubico PIV Tool\bin\libykcs11.dll`

Download and install the [Yubico PIV Tool](https://developers.yubico.com/yubico-piv-tool/).

## License

MIT License - see [LICENSE](LICENSE) for details.
