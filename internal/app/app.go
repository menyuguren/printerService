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

type serverResult struct {
	err error
}

func Run(ctx context.Context, cfgPath string) error {
	taskStore := tasks.NewStore(50)
	source := printers.WindowsSource{}
	restarts := make(chan struct{}, 1)
	notifyRestart := func() {
		select {
		case restarts <- struct{}{}:
		default:
		}
	}

	for {
		restart, err := runOnce(ctx, cfgPath, taskStore, source, restarts, notifyRestart)
		if err != nil {
			return err
		}
		if !restart {
			return nil
		}
		log.Printf("configuration changed, restarting listeners")
	}
}

func runOnce(
	ctx context.Context,
	cfgPath string,
	taskStore *tasks.Store,
	source printers.Source,
	restarts <-chan struct{},
	notifyRestart func(),
) (bool, error) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return false, fmt.Errorf("load config: %w", err)
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		log.Printf("save default config: %v", err)
	}

	webHandler := web.NewServer(web.Options{
		Config:         cfg,
		Source:         source,
		Tasks:          taskStore,
		OnConfigChange: notifyRestart,
		Save: func(next config.Config) error {
			return config.Save(cfgPath, next)
		},
	})
	targetProvider, ok := webHandler.(interface {
		TargetPrinterName() (string, error)
	})
	if !ok {
		return false, fmt.Errorf("web handler does not expose target printer provider")
	}

	controlAddr := fmt.Sprintf("%s:%d", cfg.ControlBind, cfg.ControlPort)
	dataAddr := fmt.Sprintf("%s:%d", cfg.DataBind, cfg.DataPort)

	generationCtx, stopGeneration := context.WithCancel(ctx)
	controlServer := &http.Server{
		Addr:              controlAddr,
		Handler:           webHandler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	debug := cfg.LogLevel == config.LogLevelDebug
	dataServer := raw.NewServer(dataAddr, targetProvider.TargetPrinterName, spooler.Windows{Debug: debug}).
		WithTasks(taskStore).
		WithDebug(debug)

	errs := make(chan serverResult, 2)
	go func() {
		log.Printf("control port listening on http://%s", controlAddr)
		errs <- serverResult{err: controlServer.ListenAndServe()}
	}()
	go func() {
		ready := make(chan string, 1)
		go func() {
			addr := <-ready
			if addr != "" {
				log.Printf("printer data port listening on %s", addr)
			}
		}()
		errs <- serverResult{err: dataServer.ListenAndServe(generationCtx, ready)}
	}()

	select {
	case <-ctx.Done():
		shutdownGeneration(stopGeneration, controlServer, errs, 0)
		return false, nil
	case <-restarts:
		shutdownGeneration(stopGeneration, controlServer, errs, 0)
		return true, nil
	case result := <-errs:
		shutdownGeneration(stopGeneration, controlServer, errs, 1)
		if result.err != nil && result.err != http.ErrServerClosed {
			return false, fmt.Errorf("server failed: %w", result.err)
		}
		return false, nil
	}
}

func shutdownGeneration(
	stopGeneration context.CancelFunc,
	controlServer *http.Server,
	errs <-chan serverResult,
	alreadyReceived int,
) {
	stopGeneration()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := controlServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("control server shutdown: %v", err)
	}

	for i := alreadyReceived; i < 2; i++ {
		select {
		case result := <-errs:
			if result.err != nil && result.err != http.ErrServerClosed {
				log.Printf("server shutdown: %v", result.err)
			}
		case <-shutdownCtx.Done():
			log.Printf("server shutdown timed out")
			return
		}
	}
}
