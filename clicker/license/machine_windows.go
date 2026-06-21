//go:build windows

package license

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const machineIDLen = 12

func MachineID() (string, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE)
	if err != nil {
		return "", fmt.Errorf("read machine guid: %w", err)
	}
	defer k.Close()

	guid, _, err := k.GetStringValue("MachineGuid")
	if err != nil {
		return "", fmt.Errorf("read machine guid: %w", err)
	}

	host, _ := os.Hostname()
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(guid)) + "|" + strings.ToLower(host)))
	return strings.ToUpper(hex.EncodeToString(sum[:])[:machineIDLen]), nil
}

func FormatMachineID(id string) string {
	if len(id) != machineIDLen {
		return id
	}
	return id[0:4] + "-" + id[4:8] + "-" + id[8:12]
}
