package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"printerService/internal/config"
	"printerService/internal/printers"
	"printerService/internal/raw"
	"printerService/internal/spooler"
	"printerService/internal/tasks"
	"printerService/internal/web"
)

func Run(ctx context.Context, cfgPath string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		log.Printf("save default config: %v", err)
	}

	taskStore := tasks.NewStore(50)
	source := printers.WindowsSource{}
	webHandler := web.NewServer(web.Options{
		Config: cfg,
		Source: source,
		Tasks:  taskStore,
		Save: func(next config.Config) error {
			return config.Save(cfgPath, next)
		},
	})
	targetProvider, ok := webHandler.(interface {
		TargetPrinterName() (string, error)
	})
	if !ok {
		return fmt.Errorf("web handler does not expose target printer provider")
	}

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
		return nil
	case err := <-errs:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("server failed: %w", err)
		}
		return nil
	}
}
