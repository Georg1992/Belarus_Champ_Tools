//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func inputDriverReady() (ready bool, message string) {
	if usbipDriverPresent() {
		return true, ""
	}

	if usbipListedInRegistry() {
		return false, "USBip appears in Programs but the driver is not loaded.\n\n" +
			"Run Install.cmd as administrator again, click Yes on the security prompt, then restart.\n\n" +
			"Or reinstall from:\nhttps://github.com/vadimgrn/usbip-win2/releases\n\n" +
			"USBip в списке программ, но драйвер не загружен. Запустите Install.cmd от администратора."
	}

	return false, "The input driver is not installed on this PC yet.\n\n" +
		"Run Install.cmd from the ZIP package as administrator, then restart once.\n\n" +
		"Запустите Install.cmd от администратора и перезагрузите ПК."
}

func usbipDriverPresent() bool {
	if usbipServiceRunning() {
		return true
	}

	sysDrivers := filepath.Join(os.Getenv("SystemRoot"), "System32", "drivers", "usbip2_ude.sys")
	if _, err := os.Stat(sysDrivers); err == nil {
		return true
	}

	pattern := filepath.Join(os.Getenv("SystemRoot"), "System32", "DriverStore", "FileRepository", "usbip2_ude.inf_*", "usbip2_ude.sys")
	matches, err := filepath.Glob(pattern)
	return err == nil && len(matches) > 0
}

func usbipServiceRunning() bool {
	out, err := exec.Command("sc", "query", "usbip2_ude").CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "RUNNING")
}

func usbipListedInRegistry() bool {
	for _, path := range []string{
		`SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`,
		`SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`,
	} {
		k, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.ENUMERATE_SUB_KEYS)
		if err != nil {
			continue
		}
		names, err := k.ReadSubKeyNames(-1)
		k.Close()
		if err != nil {
			continue
		}
		for _, name := range names {
			sk, err := registry.OpenKey(registry.LOCAL_MACHINE, path+`\`+name, registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			displayName, _, err := sk.GetStringValue("DisplayName")
			sk.Close()
			if err != nil {
				continue
			}
			if strings.HasPrefix(displayName, "USBip version") {
				return true
			}
		}
	}
	return false
}
