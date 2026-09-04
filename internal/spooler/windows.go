//go:build windows

package spooler

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	spoolDLL            = syscall.NewLazyDLL("winspool.drv")
	procOpenPrinterW    = spoolDLL.NewProc("OpenPrinterW")
	procClosePrinter    = spoolDLL.NewProc("ClosePrinter")
	procStartDocPrinter = spoolDLL.NewProc("StartDocPrinterW")
	procEndDocPrinter   = spoolDLL.NewProc("EndDocPrinter")
	procStartPage       = spoolDLL.NewProc("StartPagePrinter")
	procEndPage         = spoolDLL.NewProc("EndPagePrinter")
	procWritePrinter    = spoolDLL.NewProc("WritePrinter")
)

type docInfo1 struct {
	docName    uintptr
	outputFile uintptr
	dataType   uintptr
}

func submitRaw(printer string, data []byte) error {
	if printer == "" {
		return fmt.Errorf("printer name is empty")
	}
	namePtr, err := syscall.UTF16PtrFromString(printer)
	if err != nil {
		return err
	}

	var handle uintptr
	r1, _, err := procOpenPrinterW.Call(
		uintptr(unsafe.Pointer(namePtr)),
		uintptr(unsafe.Pointer(&handle)),
		0,
	)
	if r1 == 0 {
		return fmt.Errorf("OpenPrinterW %q: %w", printer, err)
	}
	defer procClosePrinter.Call(handle)

	docName, _ := syscall.UTF16PtrFromString("network print job")
	dataType, _ := syscall.UTF16PtrFromString("RAW")
	doc := docInfo1{
		docName:  uintptr(unsafe.Pointer(docName)),
		dataType: uintptr(unsafe.Pointer(dataType)),
	}

	r1, _, err = procStartDocPrinter.Call(
		handle,
		1,
		uintptr(unsafe.Pointer(&doc)),
	)
	if r1 == 0 {
		return fmt.Errorf("StartDocPrinterW: %w", err)
	}
	defer procEndDocPrinter.Call(handle)

	r1, _, err = procStartPage.Call(handle)
	if r1 == 0 {
		return fmt.Errorf("StartPagePrinter: %w", err)
	}
	defer procEndPage.Call(handle)

	if len(data) == 0 {
		return nil
	}
	var written uint32
	r1, _, err = procWritePrinter.Call(
		handle,
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(len(data)),
		uintptr(unsafe.Pointer(&written)),
	)
	if r1 == 0 {
		return fmt.Errorf("WritePrinter: %w", err)
	}
	if written != uint32(len(data)) {
		return fmt.Errorf("WritePrinter wrote %d of %d bytes", written, len(data))
	}
	return nil
}
