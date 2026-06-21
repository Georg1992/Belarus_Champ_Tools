package license

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIssueAndVerifyRoundtrip(t *testing.T) {
	privPath := filepath.Join("private.pem")
	priv, err := LoadPrivateKey(privPath)
	if err != nil {
		t.Skip("private.pem not available:", err)
	}

	mid, err := MachineID()
	if err != nil {
		t.Fatal(err)
	}

	code, err := IssueCode(priv, FormatMachineID(mid), "test")
	if err != nil {
		t.Fatal(err)
	}

	if err := VerifyCode(code); err != nil {
		t.Fatalf("VerifyCode: %v (code prefix %q)", err, code[:min(24, len(code))])
	}
}

func TestEmbeddedPublicMatchesFile(t *testing.T) {
	filePub, err := os.ReadFile(filepath.Join("public.pem"))
	if err != nil {
		t.Fatal(err)
	}
	if string(filePub) != string(publicKeyPEM) {
		t.Fatal("embedded public.pem does not match license/public.pem — run build.ps1")
	}
}

func TestIssueAndVerifyManyCodes(t *testing.T) {
	priv, err := LoadPrivateKey(filepath.Join("private.pem"))
	if err != nil {
		t.Skip("private.pem not available:", err)
	}
	mid, err := MachineID()
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 100; i++ {
		code, err := IssueCode(priv, FormatMachineID(mid), "test")
		if err != nil {
			t.Fatal(err)
		}
		if err := VerifyCode(code); err != nil {
			t.Fatalf("iteration %d: VerifyCode: %v", i, err)
		}
	}
}
