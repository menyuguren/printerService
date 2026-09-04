package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"printerService/internal/config"
	"printerService/internal/printers"
	"printerService/internal/raw"
	"printerService/internal/spooler"
	"printerService/internal/tasks"
	"printerService/internal/web"
)

func main() {
	cfgPath := flag.String("config", defaultConfigPath(), "config file path")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if err := config.Save(*cfgPath, cfg); err != nil {
		log.Printf("save default config: %v", err)
	}

	taskStore := tasks.NewStore(50)
	source := printers.WindowsSource{}
	webHandler := web.NewServer(web.Options{
		Config: cfg,
		Source: source,
		Tasks:  taskStore,
		Save: func(next config.Config) error {
			return config.Save(*cfgPath, next)
		},
	})
	targetProvider, ok := webHandler.(interface {
		TargetPrinterName() (string, error)
	})
	if !ok {
		log.Fatal("web handler does not expose target printer provider")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	controlAddr := fmt.Sprintf("%s:%d", cfg.ControlBind, cfg.ControlPort)
	dataAddr := fmt.Sprintf("%s:%d", cfg.DataBind, cfg.DataPort)

	controlServer := &http.Server{
		Addr:              controlAddr,
		Handler:           webHandler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	dataServer := raw.NewServer(dataAddr, targetProvider.TargetPrinterName, spooler.Windows{}).WithTasks(taskStore)

	errs := make(chan error, 2)
	go func() {
		log.Printf("control port listening on http://%s", controlAddr)
		errs <- controlServer.ListenAndServe()
	}()
	go func() {
		ready := make(chan string, 1)
		go func() {
			addr := <-ready
			if addr != "" {
				log.Printf("printer data port listening on %s", addr)
			}
		}()
		errs <- dataServer.ListenAndServe(ctx, ready)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := controlServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("control server shutdown: %v", err)
		}
	case err := <-errs:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}
}

func defaultConfigPath() string {
	base := os.Getenv("ProgramData")
	if base == "" {
		base = "."
	}
	return filepath.Join(base, "printerService", "config.json")
}
