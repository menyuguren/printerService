//go:build !windows

package printers

import "runtime"

type WindowsSource struct{}

func (WindowsSource) List() ([]Info, error) {
	return nil, ErrUnsupportedPlatform
}

var ErrUnsupportedPlatform = unsupportedPlatformError(runtime.GOOS)

type unsupportedPlatformError string

func (e unsupportedPlatformError) Error() string {
	return "printer enumeration is only supported on Windows, current platform: " + string(e)
}
