package license

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
)

const (
	privateKeyBlock = "ED25519 PRIVATE KEY"
	publicKeyBlock  = "ED25519 PUBLIC KEY"
)

func EnsureKeyPair(privatePath, publicPath string) error {
	if _, err := os.Stat(publicPath); err == nil {
		return nil
	}
	return GenerateKeyPair(privatePath, publicPath)
}

func GenerateKeyPair(privatePath, publicPath string) error {
	if err := os.MkdirAll(filepath.Dir(privatePath), 0o755); err != nil {
		return err
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}

	if err := os.WriteFile(privatePath, encodePrivateKeyPEM(priv), 0o600); err != nil {
		return err
	}
	return os.WriteFile(publicPath, encodePublicKeyPEM(pub), 0o644)
}

func LoadPrivateKey(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parsePrivateKeyPEM(data)
}

func parsePrivateKeyPEM(data []byte) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil || block.Type != privateKeyBlock {
		return nil, errors.New("invalid private key file")
	}
	if len(block.Bytes) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid private key size")
	}
	return ed25519.PrivateKey(block.Bytes), nil
}

func parsePublicKeyPEM(data []byte) ([]byte, error) {
	block, _ := pem.Decode(data)
	if block == nil || block.Type != publicKeyBlock {
		return nil, errors.New("invalid public key file")
	}
	if len(block.Bytes) != ed25519.PublicKeySize {
		return nil, errors.New("invalid public key size")
	}
	return block.Bytes, nil
}

func encodePrivateKeyPEM(key ed25519.PrivateKey) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  privateKeyBlock,
		Bytes: key,
	})
}

func encodePublicKeyPEM(key ed25519.PublicKey) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  publicKeyBlock,
		Bytes: key,
	})
}
