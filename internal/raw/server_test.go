package raw

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestPayloadFingerprintUsesDigestAndBoundedPreview(t *testing.T) {
	data := []byte("%PDF-1.7\n%PCLm 1.0\npayload")

	got := payloadFingerprint(data)

	if got.Digest != "da8310e8f74c9677070595ffb140f5708728c375308fe439a2c88497584f4bc1" {
		t.Fatalf("digest = %q, want SHA-256 digest", got.Digest)
	}
	if got.Preview != "255044462d312e370a2550434c6d20312e300a7061796c6f6164" {
		t.Fatalf("preview = %q, want hex payload preview", got.Preview)
	}
	if got.Format != "PDF/PCLm" {
		t.Fatalf("format = %q, want PDF/PCLm", got.Format)
	}
}

func TestPayloadFingerprintClassifiesZipPayload(t *testing.T) {
	got := payloadFingerprint([]byte("PK\x03\x04zip-data"))

	if got.Format != "ZIP-container" {
		t.Fatalf("format = %q, want ZIP-container", got.Format)
	}
}

type captureSpooler struct {
	printer string
	data    []byte
}

func (s *captureSpooler) Submit(_ context.Context, printer string, data []byte) error {
	s.printer = printer
	s.data = append([]byte(nil), data...)
	return nil
}

func TestServerReceivesConnectionAndSubmitsPrintJob(t *testing.T) {
	spooler := &captureSpooler{}
	server := NewServer("127.0.0.1:0", func() (string, error) {
		return "Office", nil
	}, spooler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ready := make(chan string, 1)
	errs := make(chan error, 1)
	go func() {
		errs <- server.ListenAndServe(ctx, ready)
	}()

	addr := <-ready
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Dial returned error: %v", err)
	}
	if _, err := conn.Write([]byte("print-data")); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if spooler.printer == "Office" && string(spooler.data) == "print-data" {
			cancel()
			<-errs
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("spooler captured printer=%q data=%q", spooler.printer, string(spooler.data))
}
