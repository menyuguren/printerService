package printers

import (
	"errors"
	"testing"

	"printerService/internal/config"
)

type fakeSource struct {
	printers []Info
	err      error
}

func (f fakeSource) List() ([]Info, error) {
	return f.printers, f.err
}

func TestResolveUsesConfiguredPrinterFirst(t *testing.T) {
	cfg := config.Default()
	cfg.UseDefaultPrinter = false
	cfg.PrinterName = "Label"

	target, mode, err := ResolveTarget(cfg, fakeSource{printers: []Info{
		{Name: "Default", IsDefault: true},
		{Name: "Label"},
	}})
	if err != nil {
		t.Fatalf("ResolveTarget returned error: %v", err)
	}
	if target.Name != "Label" {
		t.Fatalf("target = %q, want Label", target.Name)
	}
	if mode != ModeConfigured {
		t.Fatalf("mode = %q, want %q", mode, ModeConfigured)
	}
}

func TestResolveFallsBackToDefaultWhenNoConfiguredPrinter(t *testing.T) {
	cfg := config.Default()

	target, mode, err := ResolveTarget(cfg, fakeSource{printers: []Info{
		{Name: "Receipt"},
		{Name: "Default", IsDefault: true},
	}})
	if err != nil {
		t.Fatalf("ResolveTarget returned error: %v", err)
	}
	if target.Name != "Default" {
		t.Fatalf("target = %q, want Default", target.Name)
	}
	if mode != ModeDefault {
		t.Fatalf("mode = %q, want %q", mode, ModeDefault)
	}
}

func TestResolveFallsBackWhenConfiguredPrinterMissing(t *testing.T) {
	cfg := config.Default()
	cfg.UseDefaultPrinter = false
	cfg.PrinterName = "Missing"

	target, mode, err := ResolveTarget(cfg, fakeSource{printers: []Info{
		{Name: "Default", IsDefault: true},
	}})
	if err != nil {
		t.Fatalf("ResolveTarget returned error: %v", err)
	}
	if target.Name != "Default" {
		t.Fatalf("target = %q, want Default", target.Name)
	}
	if mode != ModeFallbackDefault {
		t.Fatalf("mode = %q, want %q", mode, ModeFallbackDefault)
	}
}

func TestResolveFailsWhenNoPrinterAvailable(t *testing.T) {
	_, _, err := ResolveTarget(config.Default(), fakeSource{})
	if !errors.Is(err, ErrNoPrinter) {
		t.Fatalf("ResolveTarget error = %v, want ErrNoPrinter", err)
	}
}
