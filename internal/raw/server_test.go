package raw

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestPayloadFingerprintUsesDigestAndBoundedPreview(t *testing.T) {
	data := []byte("abcdef")

	got := payloadFingerprint(data)

	if got.Digest != "bef57ec7f53a6d40beb640a780a639c83bc29ac8a9816f1fc6c5c6dcd93c4721" {
		t.Fatalf("digest = %q, want SHA-256 digest", got.Digest)
	}
	if got.Preview != "616263646566" {
		t.Fatalf("preview = %q, want hex payload preview", got.Preview)
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
