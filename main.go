package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/miekg/pkcs11"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"
)

func defaultPkcs11LibPath() (string, error) {
	switch runtime.GOOS {
	case "linux":
		return "/usr/lib/x86_64-linux-gnu/opensc-pkcs11.so", nil
	case "windows":
		return filepath.Join(
			`C:\`,
			"Program Files",
			"Yubico",
			"Yubico PIV Tool",
			"bin",
			"libykcs11.dll",
		), nil
	default:
		return "", errors.New("platform not supported")
	}
}

func main() {
	app := &cli.App{
		Name:  "ec2-win-pkcs11",
		Usage: "Decrypt EC2 Windows password using PKCS#11 token",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "instance-id",
				Aliases:  []string{"i"},
				Usage:    "EC2 instance ID",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "profile",
				Aliases: []string{"p"},
				Value:   "default",
				Usage:   "AWS profile name",
			},
			&cli.StringFlag{
				Name:    "token",
				Aliases: []string{"t"},
				Usage:   "Token serial number",
			},
			&cli.StringFlag{
				Name:    "lib",
				Aliases: []string{"l"},
				Usage:   "PKCS#11 library path",
			},
		},
		Action: run,
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(c *cli.Context) error {
	instanceID := c.String("instance-id")
	profile := c.String("profile")
	tokenSerial := c.String("token")
	libPath := c.String("lib")

	if libPath == "" {
		var err error
		libPath, err = defaultPkcs11LibPath()
		if err != nil {
			return err
		}
	}

	// Load PKCS#11 library
	p := pkcs11.New(libPath)
	if p == nil {
		return fmt.Errorf("failed to load PKCS#11 library: %s", libPath)
	}

	if err := p.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize PKCS#11: %w", err)
	}
	defer p.Destroy()
	defer p.Finalize()

	// Get slots with tokens
	slots, err := p.GetSlotList(true)
	if err != nil {
		return fmt.Errorf("failed to get slot list: %w", err)
	}

	if len(slots) == 0 {
		return fmt.Errorf("no tokens found")
	}

	// Find the token
	var selectedSlot uint
	var selectedTokenInfo pkcs11.TokenInfo
	var found bool

	if tokenSerial != "" {
		// Find token by serial
		for _, slot := range slots {
			tokenInfo, err := p.GetTokenInfo(slot)
			if err != nil {
				continue
			}
			serial := strings.TrimSpace(tokenInfo.SerialNumber)
			if serial == tokenSerial {
				selectedSlot = slot
				selectedTokenInfo = tokenInfo
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("token with serial %q not found", tokenSerial)
		}
	} else if len(slots) == 1 {
		// Only one token, use it
		selectedSlot = slots[0]
		selectedTokenInfo, err = p.GetTokenInfo(selectedSlot)
		if err != nil {
			return fmt.Errorf("failed to get token info: %w", err)
		}
		found = true
	} else {
		// Multiple tokens, list them
		fmt.Println("Multiple tokens found. Please provide token serial number.")
		fmt.Println("Available token serial numbers:")
		for _, slot := range slots {
			tokenInfo, err := p.GetTokenInfo(slot)
			if err != nil {
				continue
			}
			fmt.Println(strings.TrimSpace(tokenInfo.SerialNumber))
		}
		return nil
	}

	fmt.Printf("Using token serial number: %s\n", strings.TrimSpace(selectedTokenInfo.SerialNumber))

	// Load AWS config
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedConfigProfile(profile),
	)
	if err != nil {
		return fmt.Errorf("could not find profile %q: %w", profile, err)
	}

	// Create EC2 client and get password data
	ec2Client := ec2.NewFromConfig(cfg)
	resp, err := ec2Client.GetPasswordData(ctx, &ec2.GetPasswordDataInput{
		InstanceId: &instanceID,
	})
	if err != nil {
		return fmt.Errorf("failed to get password data: %w", err)
	}

	if resp.PasswordData == nil || *resp.PasswordData == "" {
		fmt.Println("Instance is not ready yet.")
		return nil
	}

	// Base64 decode password data
	passwordDataRaw, err := base64.StdEncoding.DecodeString(*resp.PasswordData)
	if err != nil {
		return fmt.Errorf("failed to decode password data: %w", err)
	}

	// Prompt for PIN
	fmt.Print("PIN: ")
	pinBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to read PIN: %w", err)
	}
	fmt.Println()
	pin := string(pinBytes)

	// Open PKCS#11 session
	session, err := p.OpenSession(selectedSlot, pkcs11.CKF_SERIAL_SESSION)
	if err != nil {
		return fmt.Errorf("failed to open session: %w", err)
	}
	defer p.CloseSession(session)

	// Login with PIN
	if err := p.Login(session, pkcs11.CKU_USER, pin); err != nil {
		return fmt.Errorf("failed to login: %w", err)
	}
	defer p.Logout(session)

	// Find the PIV AUTH key
	template := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_ID, []byte{0x01}),
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_PRIVATE_KEY),
	}

	if err := p.FindObjectsInit(session, template); err != nil {
		return fmt.Errorf("failed to init find objects: %w", err)
	}

	objs, _, err := p.FindObjects(session, 1)
	if err != nil {
		return fmt.Errorf("failed to find objects: %w", err)
	}

	if err := p.FindObjectsFinal(session); err != nil {
		return fmt.Errorf("failed to finalize find objects: %w", err)
	}

	if len(objs) == 0 {
		return fmt.Errorf("PIV AUTH key not found")
	}

	// Decrypt the password
	mechanism := []*pkcs11.Mechanism{pkcs11.NewMechanism(pkcs11.CKM_RSA_PKCS, nil)}
	if err := p.DecryptInit(session, mechanism, objs[0]); err != nil {
		return fmt.Errorf("failed to init decrypt: %w", err)
	}

	decrypted, err := p.Decrypt(session, passwordDataRaw)
	if err != nil {
		return fmt.Errorf("failed to decrypt: %w", err)
	}

	fmt.Println(string(decrypted))
	return nil
}
