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
	defer func() {
		_ = conn.Close()
		if s.debug {
			log.Printf("debug print connection closed from %s", sourceIP)
		}
	}()

	if s.debug {
		log.Printf("debug print connection accepted from %s", sourceIP)
	}

	printer, err := s.target()
	if err != nil {
		log.Printf("resolve target printer failed from %s: %v", sourceIP, err)
		return
	}
	if s.debug {
		log.Printf("debug print target resolved from %s to %q", sourceIP, printer)
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
		log.Printf("debug print job %s received from %s: bytes=%d sha256=%s preview=%s",
			id, sourceIP, len(data), fingerprint.Digest, fingerprint.Preview)
	}
	if int64(len(data)) > s.maxSize {
		s.fail(id, fmt.Errorf("print job exceeds max size %d bytes", s.maxSize))
		return
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
	}
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
