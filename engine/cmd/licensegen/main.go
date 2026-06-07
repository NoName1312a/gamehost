// Command licensegen is the vendor-side tool for the GameHost license system.
//
//	licensegen -init                      generate a signing keypair
//	licensegen -sign -email a@b.com       mint a Pro license key
//
// The private key is written to ~/.gamehost/license-signing.key and must never
// be committed; the printed public key goes into internal/license/pubkey.go.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/leop1/gamehost/engine/internal/license"
)

func keyPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".gamehost", "license-signing.key")
}

func main() {
	doInit := flag.Bool("init", false, "generate a new signing keypair")
	doSign := flag.Bool("sign", false, "sign a license key")
	doVerify := flag.String("verify", "", "verify a license key against the embedded public key")
	email := flag.String("email", "", "license email (for -sign)")
	tier := flag.String("tier", "pro", "license tier (for -sign)")
	days := flag.Int("days", 0, "validity in days, 0 = perpetual (for -sign)")
	flag.Parse()

	switch {
	case *doVerify != "":
		lic, err := license.Verify(*doVerify)
		if err != nil {
			fail("invalid: " + err.Error())
		}
		fmt.Printf("valid: tier=%s email=%s pro=%v\n", lic.Tier, lic.Email, lic.IsPro(time.Now()))

	case *doInit:
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		must(err)
		p := keyPath()
		must(os.MkdirAll(filepath.Dir(p), 0o700))
		must(os.WriteFile(p, []byte(base64.StdEncoding.EncodeToString(priv)), 0o600))
		fmt.Printf("Private signing key written to %s (keep safe, never commit).\n\n", p)
		fmt.Println("Set embeddedPubKeyB64 in engine/internal/license/pubkey.go to:")
		fmt.Println(base64.StdEncoding.EncodeToString(pub))

	case *doSign:
		if *email == "" {
			fail("-sign requires -email")
		}
		b, err := os.ReadFile(keyPath())
		if err != nil {
			fail("read signing key (run -init first): " + err.Error())
		}
		raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(b)))
		if err != nil || len(raw) != ed25519.PrivateKeySize {
			fail("signing key file is corrupt")
		}
		var exp int64
		if *days > 0 {
			exp = time.Now().AddDate(0, 0, *days).Unix()
		}
		key, err := license.Sign(ed25519.PrivateKey(raw), license.License{Email: *email, Tier: *tier, Expires: exp})
		must(err)
		fmt.Println(key)

	default:
		flag.Usage()
		os.Exit(2)
	}
}

func must(err error) {
	if err != nil {
		fail(err.Error())
	}
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "licensegen: "+msg)
	os.Exit(1)
}
