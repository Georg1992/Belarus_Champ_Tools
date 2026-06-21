package license

import (
	"crypto/ed25519"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed public.pem
var publicKeyPEM []byte

const (
	codePrefix  = "BCC-"
	licenseFile = "activation.key"
)

var (
	ErrInvalidCode  = errors.New("invalid activation code")
	ErrWrongMachine = errors.New("activation code is for a different computer")

	errNotActivated = errors.New("not activated")
)

type payload struct {
	Machine string `json:"m"`
	Issued  int64  `json:"i"`
	Note    string `json:"n,omitempty"`
}

func publicKey() (ed25519.PublicKey, error) {
	block, err := parsePublicKeyPEM(publicKeyPEM)
	if err != nil {
		return nil, err
	}
	return ed25519.PublicKey(block), nil
}

func licensePath() (string, error) {
	dir := filepath.Join(os.Getenv("LOCALAPPDATA"), "BelarusChampClicker")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, licenseFile), nil
}

func storedCode() (string, error) {
	path, err := licensePath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", errNotActivated
		}
		return "", err
	}
	code := strings.TrimSpace(string(data))
	if code == "" {
		return "", errNotActivated
	}
	return code, nil
}

func Activated() bool {
	code, err := storedCode()
	if err != nil {
		return false
	}
	return VerifyCode(code) == nil
}

func SaveCode(code string) error {
	code = normalizeCode(code)
	if err := VerifyCode(code); err != nil {
		return err
	}
	path, err := licensePath()
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(code), 0o600)
}

func VerifyCode(code string) error {
	pub, err := publicKey()
	if err != nil {
		return err
	}

	raw, err := decodeCode(code)
	if err != nil {
		return ErrInvalidCode
	}
	if len(raw) < ed25519.SignatureSize+1 {
		return ErrInvalidCode
	}

	sigStart := len(raw) - ed25519.SignatureSize
	payloadBytes := raw[:sigStart]
	sig := raw[sigStart:]

	if !ed25519.Verify(pub, payloadBytes, sig) {
		return ErrInvalidCode
	}

	var p payload
	if err := json.Unmarshal(payloadBytes, &p); err != nil {
		return ErrInvalidCode
	}

	machineID, err := MachineID()
	if err != nil {
		return err
	}
	if !strings.EqualFold(p.Machine, machineID) {
		return ErrWrongMachine
	}
	if p.Issued <= 0 {
		return ErrInvalidCode
	}
	if p.Issued > time.Now().Unix()+86400 {
		return ErrInvalidCode
	}

	return nil
}

func normalizeCode(code string) string {
	code = strings.TrimSpace(code)
	code = strings.ReplaceAll(code, " ", "")
	return code
}

func decodeCode(code string) ([]byte, error) {
	code = normalizeCode(code)
	if len(code) < len(codePrefix) || !strings.EqualFold(code[:len(codePrefix)], codePrefix) {
		return nil, ErrInvalidCode
	}
	body := code[len(codePrefix):]
	return base64.RawURLEncoding.DecodeString(body)
}

func IssueCode(privateKey ed25519.PrivateKey, machineDisplayOrRaw string, note string) (string, error) {
	machine := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(machineDisplayOrRaw), "-", ""))
	if len(machine) != 12 {
		return "", fmt.Errorf("machine id must be 12 characters (example: ABCD-EF12-3456)")
	}

	p := payload{
		Machine: machine,
		Issued:  time.Now().Unix(),
		Note:    strings.TrimSpace(note),
	}
	payloadBytes, err := json.Marshal(p)
	if err != nil {
		return "", err
	}

	sig := ed25519.Sign(privateKey, payloadBytes)
	raw := append(append([]byte(nil), payloadBytes...), sig...)
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	return formatCode(encoded), nil
}

func formatCode(encoded string) string {
	return codePrefix + encoded
}
