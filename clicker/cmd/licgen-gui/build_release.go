//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func clickerDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}

func runReleaseBuild() (string, error) {
	dir, err := clickerDir()
	if err != nil {
		return "", err
	}

	buildScript := filepath.Join(dir, "build.ps1")
	if _, err := os.Stat(buildScript); err != nil {
		return "", fmt.Errorf("build.ps1 not found in %s — run licgen-gui.exe from the clicker folder", dir)
	}

	if out, err := runPowerShell(buildScript, dir); err != nil {
		return "", fmt.Errorf("build.ps1 failed: %w\n%s", err, out)
	}
	if out, err := runPowerShell(filepath.Join(dir, "package.ps1"), dir); err != nil {
		return "", fmt.Errorf("package.ps1 failed: %w\n%s", err, out)
	}

	zipPath := filepath.Join(dir, "..", "release", "BelarusChampClicker-Windows-x64.zip")
	return zipPath, nil
}

func runPowerShell(script, dir string) (string, error) {
	cmd := exec.Command(
		"powershell.exe",
		"-NoProfile",
		"-ExecutionPolicy", "Bypass",
		"-File", script,
	)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}
