//go:build !windows

package winservice

import (
	"context"
	"errors"
)

type RunFunc func(context.Context, string) error

func IsService() bool {
	return false
}

func Run(_ string, _ RunFunc) error {
	return errors.New("Windows services are only supported on Windows")
}

func Install(_ string, _ string) error {
	return errors.New("Windows services are only supported on Windows")
}

func Uninstall() error {
	return errors.New("Windows services are only supported on Windows")
}

func Start() error {
	return errors.New("Windows services are only supported on Windows")
}

func Stop() error {
	return errors.New("Windows services are only supported on Windows")
}
