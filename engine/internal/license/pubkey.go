package license

import (
	"crypto/ed25519"
	"encoding/base64"
)

// embeddedPubKeyB64 is the std-base64 Ed25519 public key the engine trusts to
// verify license keys. The matching private key signs licenses and is kept by
// the vendor — never committed. Set by `cmd/licensegen -init`.
const embeddedPubKeyB64 = "AL/z8fU23cCmp56oYF/znbMWkZTvlE4qqpOAqzBrQpk="

// EmbeddedPublicKey returns the trusted verification key, or nil if unset (in
// which case no real license verifies — useful before keys are provisioned).
func EmbeddedPublicKey() ed25519.PublicKey {
	b, err := base64.StdEncoding.DecodeString(embeddedPubKeyB64)
	if err != nil || len(b) != ed25519.PublicKeySize {
		return nil
	}
	return ed25519.PublicKey(b)
}
