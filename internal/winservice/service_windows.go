//go:build windows

package winservice

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

const (
	serviceName               = "printerService"
	displayName               = "printerService"
	description               = "printerService print proxy"
	serviceWin32OwnProcess    = 0x00000010
	serviceStopped            = 0x00000001
	serviceStartPending       = 0x00000002
	serviceStopPending        = 0x00000003
	serviceRunning            = 0x00000004
	serviceAcceptStop         = 0x00000001
	serviceAcceptShutdown     = 0x00000004
	serviceControlStop        = 0x00000001
	serviceControlInterrogate = 0x00000004
	serviceControlShutdown    = 0x00000005
)

type RunFunc func(context.Context, string) error

var (
	advapi32                        = syscall.NewLazyDLL("advapi32.dll")
	procStartServiceCtrlDispatcherW = advapi32.NewProc("StartServiceCtrlDispatcherW")
	procRegisterServiceCtrlHandlerW = advapi32.NewProc("RegisterServiceCtrlHandlerW")
	procSetServiceStatus            = advapi32.NewProc("SetServiceStatus")
)

func Run(configPath string, run RunFunc) error {
	handler := &serviceHandler{
		configPath: configPath,
		run:        run,
		controls:   make(chan uint32, 4),
	}
	serviceMain := syscall.NewCallback(handler.serviceMain)
	name, err := syscall.UTF16PtrFromString(serviceName)
	if err != nil {
		return err
	}
	table := []serviceTableEntry{
		{serviceName: name, serviceProc: serviceMain},
		{},
	}

	r1, _, callErr := procStartServiceCtrlDispatcherW.Call(uintptr(unsafe.Pointer(&table[0])))
	runtime.KeepAlive(serviceMain)
	runtime.KeepAlive(handler)
	if r1 == 0 {
		if callErr != syscall.Errno(0) {
			return callErr
		}
		return errors.New("StartServiceCtrlDispatcherW failed")
	}
	return nil
}

func Install(exePath string, configPath string) error {
	absPath, err := filepath.Abs(exePath)
	if err != nil {
		return err
	}
	if err := runSC("create", serviceName, "binPath=", serviceBinPath(absPath, configPath), "DisplayName=", displayName, "start=", "auto", "error=", "normal"); err != nil {
		return err
	}
	return runSC("description", serviceName, description)
}

func Uninstall() error {
	return runSC("delete", serviceName)
}

func Start() error {
	return runSC("start", serviceName)
}

func Stop() error {
	return runSC("stop", serviceName)
}

func serviceArgs(configPath string) []string {
	args := []string{"service"}
	if configPath == "" {
		return args
	}
	return append(args, "-config", configPath)
}

func serviceBinPath(exePath string, configPath string) string {
	parts := []string{quoteArg(exePath)}
	for _, arg := range serviceArgs(configPath) {
		parts = append(parts, quoteArg(arg))
	}
	return strings.Join(parts, " ")
}

func quoteArg(arg string) string {
	return strconv.Quote(arg)
}

func runSC(args ...string) error {
	cmd := exec.Command("sc.exe", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sc.exe %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

type serviceHandler struct {
	configPath string
	run        RunFunc
	controls   chan uint32
	handle     uintptr
}

func (h *serviceHandler) serviceMain(_ uint32, _ uintptr) {
	controlHandler := syscall.NewCallback(func(control uint32) uintptr {
		select {
		case h.controls <- control:
		default:
		}
		return 0
	})
	name, err := syscall.UTF16PtrFromString(serviceName)
	if err != nil {
		return
	}
	r1, _, _ := procRegisterServiceCtrlHandlerW.Call(
		uintptr(unsafe.Pointer(name)),
		controlHandler,
	)
	if r1 == 0 {
		return
	}
	h.handle = r1
	h.setStatus(serviceStartPending, 0, 10_000)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errs := make(chan error, 1)
	go func() {
		errs <- h.run(ctx, h.configPath)
	}()

	h.setStatus(serviceRunning, 0, 0)

	for {
		select {
		case control := <-h.controls:
			switch control {
			case serviceControlInterrogate:
				h.setStatus(serviceRunning, 0, 0)
			case serviceControlStop, serviceControlShutdown:
				h.setStatus(serviceStopPending, 0, 10_000)
				cancel()
				if err := <-errs; err != nil {
					h.setStatus(serviceStopped, 1, 0)
					return
				}
				h.setStatus(serviceStopped, 0, 0)
				return
			default:
				h.setStatus(serviceRunning, 0, 0)
			}
		case err := <-errs:
			if err != nil {
				h.setStatus(serviceStopped, 1, 0)
				return
			}
			h.setStatus(serviceStopped, 0, 0)
			return
		}
	}
}

func (h *serviceHandler) setStatus(state uint32, exitCode uint32, waitHint uint32) {
	accepted := uint32(0)
	if state == serviceRunning {
		accepted = serviceAcceptStop | serviceAcceptShutdown
	}
	status := serviceStatus{
		ServiceType:      serviceWin32OwnProcess,
		CurrentState:     state,
		ControlsAccepted: accepted,
		Win32ExitCode:    exitCode,
		WaitHint:         waitHint,
	}
	procSetServiceStatus.Call(h.handle, uintptr(unsafe.Pointer(&status)))
}

type serviceTableEntry struct {
	serviceName *uint16
	serviceProc uintptr
}

type serviceStatus struct {
	ServiceType             uint32
	CurrentState            uint32
	ControlsAccepted        uint32
	Win32ExitCode           uint32
	ServiceSpecificExitCode uint32
	CheckPoint              uint32
	WaitHint                uint32
}
