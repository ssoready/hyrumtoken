// Package hyrumtoken implements opaque pagination tokens.
//
// Token opacity is implemented using NaCl secretbox:
//
//	https://pkg.go.dev/golang.org/x/crypto/nacl/secretbox
//
// Marshal and Unmarshal require a key. Tokens are only opaque to those who do
// not have this key. Do not publish this key to your API consumers.
package hyrumtoken

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"golang.org/x/crypto/nacl/secretbox"
)

// Marshal returns an encrypted, URL-safe serialization of v using key.
//
// Marshal panics if v cannot be JSON-encoded.
//
// Marshal uses a random nonce. Providing the same key and v in multiple
// invocations will produce different results every time.
func Marshal(key *[32]byte, v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}

	var nonce [24]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		panic(err)
	}

	d := secretbox.Seal(nonce[:], b, &nonce, key)
	return base64.URLEncoding.EncodeToString(d)
}

// Unmarshal uses key to decrypt s and store the decoded value in v.
//
// If s is empty, v is not modified and Unmarshal returns nil.
func Unmarshal(key *[32]byte, s string, v any) error {
	if s == "" {
		return nil
	}

	d, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return fmt.Errorf("decode token: %w", err)
	}

	var nonce [24]byte
	copy(nonce[:], d[:24])

	b, ok := secretbox.Open(nil, d[24:], &nonce, key)
	if !ok {
		return fmt.Errorf("decrypt token: %w", err)
	}

	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("unmarshal token data: %w", err)
	}

	return nil
}
