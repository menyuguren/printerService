package logging

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigureDebugServiceWritesLogFileBesideExecutable(t *testing.T) {
	dir := t.TempDir()
	oldExecutablePath := executablePath
	oldWriter := log.Writer()
	oldFlags := log.Flags()
	defer func() {
		executablePath = oldExecutablePath
		_ = Close()
		log.SetOutput(oldWriter)
		log.SetFlags(oldFlags)
	}()

	executablePath = func() (string, error) {
		return filepath.Join(dir, "printerService.exe"), nil
	}

	if err := Configure(true, "debug"); err != nil {
		t.Fatalf("Configure returned error: %v", err)
	}
	log.Print("debug-file-test")
	if err := Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "printerService.log"))
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if !strings.Contains(string(data), "debug-file-test") {
		t.Fatalf("log file does not contain test entry: %q", string(data))
	}
}

func TestConfigureInfoServiceDoesNotCreateLogFile(t *testing.T) {
	dir := t.TempDir()
	oldExecutablePath := executablePath
	oldWriter := log.Writer()
	oldFlags := log.Flags()
	defer func() {
		executablePath = oldExecutablePath
		_ = Close()
		log.SetOutput(oldWriter)
		log.SetFlags(oldFlags)
	}()

	executablePath = func() (string, error) {
		return filepath.Join(dir, "printerService.exe"), nil
	}

	if err := Configure(true, "info"); err != nil {
		t.Fatalf("Configure returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "printerService.log")); !os.IsNotExist(err) {
		t.Fatalf("info mode created log file, stat error = %v", err)
	}
}
