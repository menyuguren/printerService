//go:build windows

package printers

import (
	"fmt"
	"strconv"
	"syscall"
	"unsafe"
)

const (
	printerEnumLocal       = 0x00000002
	printerEnumConnections = 0x00000004
)

var (
	winspool               = syscall.NewLazyDLL("winspool.drv")
	procEnumPrintersW      = winspool.NewProc("EnumPrintersW")
	procGetDefaultPrinterW = winspool.NewProc("GetDefaultPrinterW")
)

type WindowsSource struct{}

func (WindowsSource) List() ([]Info, error) {
	defaultName, _ := getDefaultPrinterName()
	items, err := enumPrinters()
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].IsDefault = items[i].Name == defaultName
	}
	return items, nil
}

type printerInfo2 struct {
	pServerName         uintptr
	pPrinterName        uintptr
	pShareName          uintptr
	pPortName           uintptr
	pDriverName         uintptr
	pComment            uintptr
	pLocation           uintptr
	pDevMode            uintptr
	pSepFile            uintptr
	pPrintProcessor     uintptr
	pDatatype           uintptr
	pParameters         uintptr
	pSecurityDescriptor uintptr
	Attributes          uint32
	Priority            uint32
	DefaultPriority     uint32
	StartTime           uint32
	UntilTime           uint32
	Status              uint32
	CJobs               uint32
	AveragePPM          uint32
}

func enumPrinters() ([]Info, error) {
	var needed uint32
	var returned uint32
	flags := uint32(printerEnumLocal | printerEnumConnections)

	procEnumPrintersW.Call(
		uintptr(flags),
		0,
		2,
		0,
		0,
		uintptr(unsafe.Pointer(&needed)),
		uintptr(unsafe.Pointer(&returned)),
	)
	if needed == 0 {
		return nil, nil
	}

	buf := make([]byte, needed)
	r1, _, err := procEnumPrintersW.Call(
		uintptr(flags),
		0,
		2,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(needed),
		uintptr(unsafe.Pointer(&needed)),
		uintptr(unsafe.Pointer(&returned)),
	)
	if r1 == 0 {
		return nil, fmt.Errorf("EnumPrintersW: %w", err)
	}

	size := unsafe.Sizeof(printerInfo2{})
	out := make([]Info, 0, returned)
	for i := uint32(0); i < returned; i++ {
		info := (*printerInfo2)(unsafe.Pointer(uintptr(unsafe.Pointer(&buf[0])) + uintptr(i)*size))
		out = append(out, Info{
			Name:   utf16PtrToString(info.pPrinterName),
			Driver: utf16PtrToString(info.pDriverName),
			Port:   utf16PtrToString(info.pPortName),
			Status: strconv.FormatUint(uint64(info.Status), 10),
		})
	}
	return out, nil
}

func getDefaultPrinterName() (string, error) {
	var needed uint32
	procGetDefaultPrinterW.Call(0, uintptr(unsafe.Pointer(&needed)))
	if needed == 0 {
		return "", ErrNoPrinter
	}
	buf := make([]uint16, needed)
	r1, _, err := procGetDefaultPrinterW.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&needed)),
	)
	if r1 == 0 {
		return "", fmt.Errorf("GetDefaultPrinterW: %w", err)
	}
	return syscall.UTF16ToString(buf), nil
}

func utf16PtrToString(ptr uintptr) string {
	if ptr == 0 {
		return ""
	}
	var values []uint16
	for i := uintptr(0); ; i++ {
		value := *(*uint16)(unsafe.Pointer(ptr + i*unsafe.Sizeof(uint16(0))))
		if value == 0 {
			break
		}
		values = append(values, value)
	}
	return syscall.UTF16ToString(values)
}
