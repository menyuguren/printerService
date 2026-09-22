package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"printerService/internal/config"
	"printerService/internal/printers"
	"printerService/internal/tasks"
)

type fakePrinterSource struct {
	items []printers.Info
}

func (f fakePrinterSource) List() ([]printers.Info, error) {
	return f.items, nil
}

func TestIndexUsesEventBoundPrinterButtons(t *testing.T) {
	handler := NewServer(Options{
		Config: config.Default(),
		Source: fakePrinterSource{items: []printers.Info{
			{Name: `Printer "A"`, IsDefault: true},
		}},
		Tasks: tasks.NewStore(5),
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(res, req)

	body := res.Body.String()
	if strings.Contains(body, `onclick="setPrinter(`) {
		t.Fatal("index uses inline setPrinter onclick; printer selection must use event-bound buttons")
	}
	if strings.Contains(body, `replaceChildren`) {
		t.Fatal("index uses replaceChildren, which is not available in the target browser")
	}
	if strings.Contains(body, "` + \"`\" + `") {
		t.Fatal("index contains broken backtick escaping")
	}
	if !strings.Contains(body, `function escapeHtml`) {
		t.Fatal("index calls escapeHtml but does not define it")
	}
	if !strings.Contains(body, `data-action="set-printer"`) {
		t.Fatal("index does not expose event-bound set-printer controls")
	}
	if !strings.Contains(body, `document.addEventListener('click'`) {
		t.Fatal("index does not bind printer button clicks through event delegation")
	}
	if !strings.Contains(body, `id="printer-select"`) {
		t.Fatal("index does not include a printer selection control")
	}
	if !strings.Contains(body, `id="control-port"`) || !strings.Contains(body, `id="data-port"`) {
		t.Fatal("index does not include port configuration controls")
	}
	if !strings.Contains(body, `id="log-level"`) {
		t.Fatal("index does not include log level configuration control")
	}
	if !strings.Contains(body, `document.getElementById('log-level').value`) {
		t.Fatal("index does not submit the selected log level")
	}
}

func TestStatusUsesDefaultPrinterWhenNoPrinterConfigured(t *testing.T) {
	handler := NewServer(Options{
		Config: config.Default(),
		Source: fakePrinterSource{items: []printers.Info{
			{Name: "Office", IsDefault: true},
		}},
		Tasks: tasks.NewStore(5),
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200", res.Code)
	}
	if res.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", res.Header().Get("Cache-Control"))
	}

	var body StatusResponse
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.TargetPrinter.Name != "Office" {
		t.Fatalf("target printer = %q, want Office", body.TargetPrinter.Name)
	}
	if body.TargetMode != printers.ModeDefault {
		t.Fatalf("target mode = %q, want %q", body.TargetMode, printers.ModeDefault)
	}
}

func TestPrinterSelectionCanBeConfiguredAndCleared(t *testing.T) {
	var saved config.Config
	handler := NewServer(Options{
		Config: config.Default(),
		Source: fakePrinterSource{items: []printers.Info{
			{Name: "Office", IsDefault: true},
			{Name: "Label"},
		}},
		Tasks: tasks.NewStore(5),
		Save: func(cfg config.Config) error {
			saved = cfg
			return nil
		},
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/config/printer", strings.NewReader(`{"printer_name":"Label"}`))
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("set status code = %d, want 200: %s", res.Code, res.Body.String())
	}
	if saved.PrinterName != "Label" || saved.UseDefaultPrinter {
		t.Fatalf("saved config = %+v, want Label and UseDefaultPrinter=false", saved)
	}

	res = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/config/printer", nil)
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("clear status code = %d, want 200: %s", res.Code, res.Body.String())
	}
	if saved.PrinterName != "" || !saved.UseDefaultPrinter {
		t.Fatalf("saved config = %+v, want empty printer and UseDefaultPrinter=true", saved)
	}
}

func TestPortConfigurationCanBeSaved(t *testing.T) {
	var saved config.Config
	handler := NewServer(Options{
		Config: config.Default(),
		Source: fakePrinterSource{items: []printers.Info{
			{Name: "Office", IsDefault: true},
		}},
		Tasks: tasks.NewStore(5),
		Save: func(cfg config.Config) error {
			saved = cfg
			return nil
		},
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{
		"control_bind":"127.0.0.1",
		"control_port":18080,
		"data_bind":"0.0.0.0",
		"data_port":19100,
		"printer_name":"Office",
		"use_default_printer":false
	}`))
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200: %s", res.Code, res.Body.String())
	}
	if saved.ControlPort != 18080 || saved.DataPort != 19100 {
		t.Fatalf("saved ports = %d/%d, want 18080/19100", saved.ControlPort, saved.DataPort)
	}
	if saved.PrinterName != "Office" || saved.UseDefaultPrinter {
		t.Fatalf("saved printer = %+v, want Office and UseDefaultPrinter=false", saved)
	}
}

func TestLogLevelConfigurationCanBeSaved(t *testing.T) {
	var saved config.Config
	handler := NewServer(Options{
		Config: config.Default(),
		Source: fakePrinterSource{items: []printers.Info{
			{Name: "Office", IsDefault: true},
		}},
		Tasks: tasks.NewStore(5),
		Save: func(cfg config.Config) error {
			saved = cfg
			return nil
		},
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{
		"control_bind":"127.0.0.1",
		"control_port":18080,
		"data_bind":"0.0.0.0",
		"data_port":19100,
		"use_default_printer":true,
		"log_level":"debug"
	}`))
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200: %s", res.Code, res.Body.String())
	}
	if saved.LogLevel != "debug" {
		t.Fatalf("saved log level = %q, want debug", saved.LogLevel)
	}
}

func TestConfigSaveTriggersConfigChangeCallback(t *testing.T) {
	var changes int
	handler := NewServer(Options{
		Config: config.Default(),
		Source: fakePrinterSource{items: []printers.Info{
			{Name: "Office", IsDefault: true},
		}},
		Tasks: tasks.NewStore(5),
		Save: func(config.Config) error {
			return nil
		},
		OnConfigChange: func() {
			changes++
		},
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{
		"control_bind":"127.0.0.1",
		"control_port":18080,
		"data_bind":"0.0.0.0",
		"data_port":19100,
		"printer_name":"Office",
		"use_default_printer":false
	}`))
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200: %s", res.Code, res.Body.String())
	}
	if changes != 1 {
		t.Fatalf("config changes = %d, want 1", changes)
	}
}
