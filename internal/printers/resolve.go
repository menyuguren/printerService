package printers

import (
	"errors"

	"printer-network-service/internal/config"
)

var ErrNoPrinter = errors.New("no available printer")

type Mode string

const (
	ModeConfigured      Mode = "configured"
	ModeDefault         Mode = "default"
	ModeFallbackDefault Mode = "fallback_default"
)

type Info struct {
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
	Driver    string `json:"driver,omitempty"`
	Port      string `json:"port,omitempty"`
	Status    string `json:"status,omitempty"`
}

type Source interface {
	List() ([]Info, error)
}

func ResolveTarget(cfg config.Config, source Source) (Info, Mode, error) {
	list, err := source.List()
	if err != nil {
		return Info{}, "", err
	}
	if len(list) == 0 {
		return Info{}, "", ErrNoPrinter
	}

	if !cfg.UseDefaultPrinter && cfg.PrinterName != "" {
		for _, p := range list {
			if p.Name == cfg.PrinterName {
				return p, ModeConfigured, nil
			}
		}
		if def, ok := defaultFrom(list); ok {
			return def, ModeFallbackDefault, nil
		}
		return Info{}, "", ErrNoPrinter
	}

	if def, ok := defaultFrom(list); ok {
		return def, ModeDefault, nil
	}
	return Info{}, "", ErrNoPrinter
}

func defaultFrom(list []Info) (Info, bool) {
	for _, p := range list {
		if p.IsDefault {
			return p, true
		}
	}
	return Info{}, false
}
