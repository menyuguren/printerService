package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"printerService/internal/app"
	"printerService/internal/winservice"
)

type command string

const (
	commandRun       command = "run"
	commandInstall   command = "install"
	commandUninstall command = "uninstall"
	commandStart     command = "start"
	commandStop      command = "stop"
	commandService   command = "service"
)

type cliOptions struct {
	command    command
	configPath string
}

func main() {
	opts, err := parseCLI(os.Args[1:], defaultConfigPath())
	if err != nil {
		log.Fatal(err)
	}

	switch opts.command {
	case commandRun:
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := app.Run(ctx, opts.configPath); err != nil {
			log.Fatal(err)
		}
	case commandInstall:
		if err := winservice.Install(os.Args[0], opts.configPath); err != nil {
			log.Fatal(err)
		}
		fmt.Println("printerService installed")
	case commandUninstall:
		if err := winservice.Uninstall(); err != nil {
			log.Fatal(err)
		}
		fmt.Println("printerService uninstalled")
	case commandStart:
		if err := winservice.Start(); err != nil {
			log.Fatal(err)
		}
		fmt.Println("printerService started")
	case commandStop:
		if err := winservice.Stop(); err != nil {
			log.Fatal(err)
		}
		fmt.Println("printerService stopped")
	case commandService:
		if err := winservice.Run(opts.configPath, app.Run); err != nil {
			log.Fatal(err)
		}
	}
}

func parseCLI(args []string, defaultConfigPath string) (cliOptions, error) {
	opts := cliOptions{
		command:    commandRun,
		configPath: defaultConfigPath,
	}
	if len(args) > 0 && isCommand(args[0]) {
		opts.command = command(args[0])
		args = args[1:]
	}

	fs := flag.NewFlagSet("printerService", flag.ContinueOnError)
	fs.StringVar(&opts.configPath, "config", opts.configPath, "config file path")
	if err := fs.Parse(args); err != nil {
		return cliOptions{}, err
	}
	if fs.NArg() > 0 {
		return cliOptions{}, fmt.Errorf("unknown command or argument: %s", fs.Arg(0))
	}
	if !commandAllowsConfig(opts.command) && opts.configPath != defaultConfigPath {
		return cliOptions{}, errors.New("-config is only supported for run, install, and service")
	}
	return opts, nil
}

func isCommand(arg string) bool {
	switch command(arg) {
	case commandRun, commandInstall, commandUninstall, commandStart, commandStop, commandService:
		return true
	default:
		return false
	}
}

func commandAllowsConfig(cmd command) bool {
	switch cmd {
	case commandRun, commandInstall, commandService:
		return true
	default:
		return false
	}
}

func defaultConfigPath() string {
	base := os.Getenv("ProgramData")
	if base == "" {
		base = "."
	}
	return filepath.Join(base, "printerService", "config.json")
}
