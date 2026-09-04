package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.ControlBind != "127.0.0.1" {
		t.Fatalf("ControlBind = %q, want 127.0.0.1", cfg.ControlBind)
	}
	if cfg.ControlPort != 8080 {
		t.Fatalf("ControlPort = %d, want 8080", cfg.ControlPort)
	}
	if cfg.DataBind != "0.0.0.0" {
		t.Fatalf("DataBind = %q, want 0.0.0.0", cfg.DataBind)
	}
	if cfg.DataPort != 9100 {
		t.Fatalf("DataPort = %d, want 9100", cfg.DataPort)
	}
	if !cfg.UseDefaultPrinter {
		t.Fatal("UseDefaultPrinter = false, want true")
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	want := Default()
	want.ControlPort = 18080
	want.DataPort = 19100
	want.PrinterName = "Office Printer"
	want.UseDefaultPrinter = false

	if err := Save(path, want); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if got != want {
		t.Fatalf("Load() = %+v, want %+v", got, want)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("saved config is not readable: %v", err)
	}
}
