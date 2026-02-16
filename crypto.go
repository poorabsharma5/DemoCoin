package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// GenerateKeyPair creates a new Ed25519 key pair.
func GenerateKeyPair() (ed25519.PrivateKey, ed25519.PublicKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return priv, pub, nil
}

// SignData signs the data using the private key.
func SignData(priv ed25519.PrivateKey, data []byte) []byte {
	return ed25519.Sign(priv, data)
}

// VerifySignature checks if the signature is valid for the given data and public key.
func VerifySignature(pub ed25519.PublicKey, data []byte, signature []byte) bool {
	if len(pub) != ed25519.PublicKeySize {
		return false
	}
	return ed25519.Verify(pub, data, signature)
}

// PubKeyToAddress derives an address from the public key.
// For simplicity, we use the hex encoding of the SHA256 hash of the public key.
func PubKeyToAddress(pub []byte) string {
	h := sha256.Sum256(pub)
	return hex.EncodeToString(h[:])
}
