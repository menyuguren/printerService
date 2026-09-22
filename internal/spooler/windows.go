//go:build windows

package spooler

import (
	"fmt"
	"log"
	"os/user"
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

func submitRaw(printer string, data []byte, debug bool) (submitErr error) {
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
		return fmt.Errorf("OpenPrinterW %q: %w", printer, win32CallError(err))
	}
	if debug {
		log.Printf("debug spooler OpenPrinter succeeded: printer=%q", printer)
		if current, userErr := user.Current(); userErr == nil {
			log.Printf("debug spooler execution user: %s", current.Username)
		}
	}

	docName, _ := syscall.UTF16PtrFromString("network print job")
	dataType, _ := syscall.UTF16PtrFromString("RAW")
	doc := docInfo1{
		docName:  uintptr(unsafe.Pointer(docName)),
		dataType: uintptr(unsafe.Pointer(dataType)),
	}

	var jobID uint32
	docStarted := false
	pageStarted := false
	defer func() {
		if pageStarted {
			r, _, endErr := procEndPage.Call(handle)
			if debug {
				log.Printf("debug spooler EndPagePrinter: printer=%q job_id=%d success=%t",
					printer, jobID, r != 0)
			}
			if r == 0 && submitErr == nil {
				submitErr = stageError("EndPagePrinter", jobID, win32CallError(endErr))
			}
		}
		if docStarted {
			r, _, endErr := procEndDocPrinter.Call(handle)
			if debug {
				log.Printf("debug spooler EndDocPrinter: printer=%q job_id=%d success=%t",
					printer, jobID, r != 0)
			}
			if r == 0 && submitErr == nil {
				submitErr = stageError("EndDocPrinter", jobID, win32CallError(endErr))
			}
		}
		r, _, closeErr := procClosePrinter.Call(handle)
		if debug {
			log.Printf("debug spooler ClosePrinter: printer=%q job_id=%d success=%t",
				printer, jobID, r != 0)
		}
		if r == 0 && submitErr == nil {
			submitErr = stageError("ClosePrinter", jobID, win32CallError(closeErr))
		}
	}()

	r1, _, err = procStartDocPrinter.Call(
		handle,
		1,
		uintptr(unsafe.Pointer(&doc)),
	)
	if r1 == 0 {
		return fmt.Errorf("StartDocPrinterW %q datatype RAW: %w",
			printer, win32CallError(err))
	}
	jobID = uint32(r1)
	docStarted = true
	if debug {
		log.Printf("debug spooler StartDocPrinter succeeded: printer=%q job_id=%d datatype=RAW",
			printer, jobID)
	}

	r1, _, err = procStartPage.Call(handle)
	if r1 == 0 {
		return fmt.Errorf("StartPagePrinter %q: %w", printer, win32CallError(err))
	}
	pageStarted = true
	if debug {
		log.Printf("debug spooler StartPagePrinter succeeded: printer=%q job_id=%d",
			printer, jobID)
	}

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
		return fmt.Errorf("WritePrinter %q datatype RAW: %w",
			printer, win32CallError(err))
	}
	if written != uint32(len(data)) {
		return fmt.Errorf("WritePrinter wrote %d of %d bytes", written, len(data))
	}
	if debug {
		log.Printf("debug spooler WritePrinter succeeded: printer=%q job_id=%d bytes=%d",
			printer, jobID, written)
	}
	return nil
}

func win32CallError(err error) error {
	if err != nil && err != syscall.Errno(0) {
		return err
	}
	last := syscall.GetLastError()
	if last != nil {
		return last
	}
	return syscall.Errno(0)
}
