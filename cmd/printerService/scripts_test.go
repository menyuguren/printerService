package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallServiceScriptInstallsAutoStartService(t *testing.T) {
	body := readRootFile(t, "install-service.bat")

	requireContains(t, body, `set "APP_EXE=%APP_DIR%printerService.exe"`)
	requireContains(t, body, `"%APP_EXE%" install`)
	requireContains(t, body, "sc.exe config printerService start= auto")
	requireContains(t, body, `"%APP_EXE%" start`)
}

func TestUninstallServiceScriptStopsAndUninstallsService(t *testing.T) {
	body := readRootFile(t, "uninstall-service.bat")

	requireContains(t, body, `set "APP_EXE=%APP_DIR%printerService.exe"`)
	requireContains(t, body, `"%APP_EXE%" stop`)
	requireContains(t, body, `"%APP_EXE%" uninstall`)
}

func readRootFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

func requireContains(t *testing.T, body string, want string) {
	t.Helper()
	if !strings.Contains(body, want) {
		t.Fatalf("script does not contain %q:\n%s", want, body)
	}
}
