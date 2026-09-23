package raw

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync/atomic"
	"time"

	"printerService/internal/tasks"
)

type TargetFunc func() (string, error)

type Spooler interface {
	Submit(ctx context.Context, printer string, data []byte) error
}

type Server struct {
	address string
	target  TargetFunc
	spooler Spooler
	tasks   *tasks.Store
	maxSize int64
	debug   bool
	seq     atomic.Uint64
}

func NewServer(address string, target TargetFunc, spooler Spooler) *Server {
	return &Server{
		address: address,
		target:  target,
		spooler: spooler,
		maxSize: 100 * 1024 * 1024,
	}
}

func (s *Server) WithTasks(store *tasks.Store) *Server {
	s.tasks = store
	return s
}

func (s *Server) WithDebug(debug bool) *Server {
	s.debug = debug
	return s
}

func (s *Server) ListenAndServe(ctx context.Context, ready chan<- string) error {
	ln, err := net.Listen("tcp", s.address)
	if err != nil {
		if ready != nil {
			close(ready)
		}
		return err
	}
	defer ln.Close()

	if ready != nil {
		ready <- ln.Addr().String()
	}

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil
			}
			var opErr *net.OpError
			if errors.As(err, &opErr) && ctx.Err() != nil {
				return nil
			}
			return err
		}
		go s.handle(ctx, conn)
	}
}

func (s *Server) handle(ctx context.Context, conn net.Conn) {
	sourceIP := remoteIP(conn.RemoteAddr())
	localAddr := conn.LocalAddr().String()
	startedAt := time.Now()
	defer func() {
		_ = conn.Close()
		if s.debug {
			log.Printf("debug print connection closed: remote=%s local=%s duration=%s",
				sourceIP, localAddr, time.Since(startedAt))
		}
	}()

	if s.debug {
		log.Printf("debug print connection accepted: remote=%s local=%s", sourceIP, localAddr)
	}

	printer, err := s.target()
	if err != nil {
		log.Printf("resolve target printer failed from %s: %v", sourceIP, err)
		return
	}
	if s.debug {
		log.Printf("debug print target resolved: remote=%s printer=%q", sourceIP, printer)
	}

	if s.debug {
		log.Printf("debug print data read started: remote=%s max_bytes=%d", sourceIP, s.maxSize)
	}
	data, err := io.ReadAll(io.LimitReader(conn, s.maxSize+1))
	id := s.nextID()
	task := tasks.Task{
		ID:        id,
		SourceIP:  sourceIP,
		Printer:   printer,
		Size:      int64(len(data)),
		Status:    tasks.StatusReceived,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if s.tasks != nil {
		s.tasks.Add(task)
	}
	if err != nil {
		s.fail(id, fmt.Errorf("read print data: %w", err))
		return
	}
	if s.debug {
		fingerprint := payloadFingerprint(data)
		log.Printf("debug print data read completed: job_id=%s remote=%s bytes=%d format=%s sha256=%s preview=%s",
			id, sourceIP, len(data), fingerprint.Format, fingerprint.Digest, fingerprint.Preview)
	}
	if int64(len(data)) > s.maxSize {
		s.fail(id, fmt.Errorf("print job exceeds max size %d bytes", s.maxSize))
		return
	}

	if s.debug {
		log.Printf("debug print spool submission started: job_id=%s printer=%q bytes=%d",
			id, printer, len(data))
	}
	if err := s.spooler.Submit(ctx, printer, data); err != nil {
		s.fail(id, err)
		return
	}
	if s.tasks != nil {
		s.tasks.Update(id, tasks.StatusSubmitted, "")
	}
	log.Printf("submitted print job %s to %q as RAW, %d bytes", id, printer, len(data))
}

type PayloadFingerprint struct {
	Digest  string
	Preview string
	Format  string
}

func payloadFingerprint(data []byte) PayloadFingerprint {
	sum := sha256.Sum256(data)
	previewLength := len(data)
	if previewLength > 32 {
		previewLength = 32
	}
	return PayloadFingerprint{
		Digest:  hex.EncodeToString(sum[:]),
		Preview: hex.EncodeToString(data[:previewLength]),
		Format:  payloadFormat(data),
	}
}

func payloadFormat(data []byte) string {
	sample := data
	if len(sample) > 256 {
		sample = sample[:256]
	}
	switch {
	case len(data) >= 8 && string(data[:8]) == "%PDF-1.7":
		if containsASCII(sample, "%PCLm") {
			return "PDF/PCLm"
		}
		return "PDF"
	case len(data) >= 4 && string(data[:4]) == "PK\x03\x04":
		return "ZIP-container"
	case len(data) >= 4 && string(data[:4]) == "%!PS":
		return "PostScript"
	case len(data) >= 2 && data[0] == 0x1b:
		return "ESC-prefixed"
	default:
		return "unknown"
	}
}

func containsASCII(data []byte, needle string) bool {
	for i := 0; i+len(needle) <= len(data); i++ {
		if string(data[i:i+len(needle)]) == needle {
			return true
		}
	}
	return false
}

func (s *Server) fail(id string, err error) {
	if s.tasks != nil {
		s.tasks.Update(id, tasks.StatusFailed, err.Error())
	}
	log.Printf("print job %s failed: %v", id, err)
}

func (s *Server) nextID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), s.seq.Add(1))
}

func remoteIP(addr net.Addr) string {
	if tcp, ok := addr.(*net.TCPAddr); ok {
		return tcp.IP.String()
	}
	return addr.String()
}
