package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"printerService/internal/config"
)

func TestRunAppliesSavedConfigByRestartingListeners(t *testing.T) {
	firstControlPort := freeTCPPort(t)
	firstDataPort := freeTCPPort(t)
	secondControlPort := freeTCPPort(t)
	secondDataPort := freeTCPPort(t)

	cfg := config.Default()
	cfg.ControlBind = "127.0.0.1"
	cfg.ControlPort = firstControlPort
	cfg.DataBind = "127.0.0.1"
	cfg.DataPort = firstDataPort

	cfgPath := t.TempDir() + "/config.json"
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("save initial config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errs := make(chan error, 1)
	go func() {
		errs <- Run(ctx, cfgPath)
	}()

	waitForStatus(t, firstControlPort, firstDataPort)

	next := cfg
	next.ControlPort = secondControlPort
	next.DataPort = secondDataPort
	next.LogLevel = config.LogLevelDebug
	putConfig(t, firstControlPort, next)

	waitForStatus(t, secondControlPort, secondDataPort)
	waitForLogLevel(t, secondControlPort, config.LogLevelDebug)

	cancel()
	select {
	case err := <-errs:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop after context cancellation")
	}
}

func waitForLogLevel(t *testing.T, port int, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	url := fmt.Sprintf("http://127.0.0.1:%d/api/status", port)
	client := &http.Client{Timeout: 250 * time.Millisecond}
	var lastErr error

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err != nil {
			lastErr = err
			time.Sleep(50 * time.Millisecond)
			continue
		}
		var body struct {
			Config config.Config `json:"config"`
		}
		err = json.NewDecoder(resp.Body).Decode(&body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if resp.StatusCode == http.StatusOK && body.Config.LogLevel == want {
			return
		}
		lastErr = fmt.Errorf("status code %d with log level %q", resp.StatusCode, body.Config.LogLevel)
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("status on port %d did not report log level %q: %v", port, want, lastErr)
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on free port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func waitForStatus(t *testing.T, port int, wantDataPort int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	url := fmt.Sprintf("http://127.0.0.1:%d/api/status", port)
	client := &http.Client{Timeout: 250 * time.Millisecond}
	var lastErr error

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err != nil {
			lastErr = err
			time.Sleep(50 * time.Millisecond)
			continue
		}
		var body struct {
			Config config.Config `json:"config"`
		}
		err = json.NewDecoder(resp.Body).Decode(&body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if resp.StatusCode == http.StatusOK && body.Config.DataPort == wantDataPort {
			return
		}
		lastErr = fmt.Errorf("status code %d with data port %d", resp.StatusCode, body.Config.DataPort)
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("status on port %d did not report data port %d: %v", port, wantDataPort, lastErr)
}

func putConfig(t *testing.T, controlPort int, cfg config.Config) {
	t.Helper()
	body := fmt.Sprintf(`{
		"control_bind":%q,
		"control_port":%d,
		"data_bind":%q,
		"data_port":%d,
		"printer_name":%q,
		"use_default_printer":%t,
		"log_level":%q
	}`, cfg.ControlBind, cfg.ControlPort, cfg.DataBind, cfg.DataPort, cfg.PrinterName, cfg.UseDefaultPrinter, cfg.LogLevel)

	url := fmt.Sprintf("http://127.0.0.1:%d/api/config", controlPort)
	req, err := http.NewRequest(http.MethodPut, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("create config request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 2 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("put config: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("put config status = %d, want 200", resp.StatusCode)
	}
}
