package main

import "testing"

func TestParseCLIUsesRunByDefault(t *testing.T) {
	opts, err := parseCLI(nil, `C:\ProgramData\printerService\config.json`)
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}

	if opts.command != commandRun {
		t.Fatalf("command = %q, want %q", opts.command, commandRun)
	}
	if opts.configPath != `C:\ProgramData\printerService\config.json` {
		t.Fatalf("configPath = %q, want default path", opts.configPath)
	}
}

func TestParseCLIParsesServiceCommandAndConfig(t *testing.T) {
	opts, err := parseCLI([]string{"install", "-config", `D:\printer\config.json`}, `C:\ProgramData\printerService\config.json`)
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}

	if opts.command != commandInstall {
		t.Fatalf("command = %q, want %q", opts.command, commandInstall)
	}
	if opts.configPath != `D:\printer\config.json` {
		t.Fatalf("configPath = %q, want custom path", opts.configPath)
	}
}

func TestParseCLIParsesInternalServiceCommand(t *testing.T) {
	opts, err := parseCLI([]string{"service", "-config", `D:\printer\config.json`}, `C:\ProgramData\printerService\config.json`)
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}

	if opts.command != commandService {
		t.Fatalf("command = %q, want %q", opts.command, commandService)
	}
	if opts.configPath != `D:\printer\config.json` {
		t.Fatalf("configPath = %q, want custom path", opts.configPath)
	}
}

func TestParseCLIParsesRunConfig(t *testing.T) {
	opts, err := parseCLI([]string{"-config", `D:\printer\config.json`}, `C:\ProgramData\printerService\config.json`)
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}

	if opts.command != commandRun {
		t.Fatalf("command = %q, want %q", opts.command, commandRun)
	}
	if opts.configPath != `D:\printer\config.json` {
		t.Fatalf("configPath = %q, want custom path", opts.configPath)
	}
}

func TestParseCLIRejectsUnknownCommand(t *testing.T) {
	if _, err := parseCLI([]string{"restart"}, `C:\ProgramData\printerService\config.json`); err == nil {
		t.Fatal("parseCLI returned nil error for unknown command")
	}
}
