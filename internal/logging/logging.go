package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

const (
	LevelInfo  = "info"
	LevelDebug = "debug"
	logName    = "printerService.log"
)

var (
	mu             sync.Mutex
	file           *os.File
	serviceMode    bool
	currentLevel   string
	executablePath = os.Executable
)

// Configure applies the process-wide logger mode. In service debug mode,
// logs are appended to printerService.log beside the executable.
func Configure(isService bool, level string) error {
	mu.Lock()
	defer mu.Unlock()

	if level == "" {
		level = LevelInfo
	}
	if level != LevelInfo && level != LevelDebug {
		return fmt.Errorf("invalid log level %q", level)
	}

	if file != nil && (!isService || level != LevelDebug) {
		if err := file.Close(); err != nil {
			return fmt.Errorf("close debug log: %w", err)
		}
		file = nil
	}

	serviceMode = isService
	currentLevel = level
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	if isService && level == LevelDebug && file == nil {
		exe, err := executablePath()
		if err != nil {
			return fmt.Errorf("resolve executable path for debug log: %w", err)
		}
		path := filepath.Join(filepath.Dir(exe), logName)
		opened, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return fmt.Errorf("open debug log %q: %w", path, err)
		}
		file = opened
		log.SetOutput(file)
		log.Printf("logging configured: mode=debug service=true file=%q", path)
		return nil
	}

	log.SetOutput(os.Stderr)
	return nil
}

func Close() error {
	mu.Lock()
	defer mu.Unlock()

	if file == nil {
		return nil
	}
	err := file.Close()
	file = nil
	serviceMode = false
	currentLevel = ""
	log.SetOutput(os.Stderr)
	if err != nil {
		return fmt.Errorf("close debug log: %w", err)
	}
	return nil
}

func IsDebug() bool {
	mu.Lock()
	defer mu.Unlock()
	return currentLevel == LevelDebug
}

func IsService() bool {
	mu.Lock()
	defer mu.Unlock()
	return serviceMode
}

func LogPath() (string, error) {
	exe, err := executablePath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), logName), nil
}
