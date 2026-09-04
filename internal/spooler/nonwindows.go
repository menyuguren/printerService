//go:build !windows

package spooler

import "runtime"

func submitRaw(string, []byte) error {
	return unsupportedPlatformError(runtime.GOOS)
}

type unsupportedPlatformError string

func (e unsupportedPlatformError) Error() string {
	return "printing is only supported on Windows, current platform: " + string(e)
}
