//go:build windows

package main

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// Opening a COM port on Windows without any external library.
// We call the Windows API (kernel32.dll) directly to set baud rate and timeouts.

var (
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	procGetCommState    = kernel32.NewProc("GetCommState")
	procSetCommState    = kernel32.NewProc("SetCommState")
	procSetCommTimeouts = kernel32.NewProc("SetCommTimeouts")
)

// DCB struct from the Windows docs (winbase.h)
type dcb struct {
	DCBlength  uint32
	BaudRate   uint32
	Flags      uint32
	wReserved  uint16
	XonLim     uint16
	XoffLim    uint16
	ByteSize   byte
	Parity     byte
	StopBits   byte
	XonChar    byte
	XoffChar   byte
	ErrorChar  byte
	EofChar    byte
	EvtChar    byte
	wReserved1 uint16
}

type commTimeouts struct {
	ReadIntervalTimeout         uint32
	ReadTotalTimeoutMultiplier  uint32
	ReadTotalTimeoutConstant    uint32
	WriteTotalTimeoutMultiplier uint32
	WriteTotalTimeoutConstant   uint32
}

// candidatePorts returns COM1 to COM30. Ports that dont exist just fail to open.
func candidatePorts() []string {
	ports := []string{}
	for i := 1; i <= 30; i++ {
		ports = append(ports, fmt.Sprintf("COM%d", i))
	}
	return ports
}

func openPort(name string, baud int) (*os.File, error) {
	// COM10 and above only work with the \\.\ prefix, so always use it
	path, err := syscall.UTF16PtrFromString(`\\.\` + name)
	if err != nil {
		return nil, err
	}
	h, err := syscall.CreateFile(path,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		0, nil, syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		return nil, err
	}

	var d dcb
	d.DCBlength = uint32(unsafe.Sizeof(d))
	r, _, err := procGetCommState.Call(uintptr(h), uintptr(unsafe.Pointer(&d)))
	if r == 0 {
		syscall.CloseHandle(h)
		return nil, fmt.Errorf("GetCommState: %v", err)
	}
	d.BaudRate = uint32(baud)
	d.ByteSize = 8
	d.Parity = 0   // no parity
	d.StopBits = 0 // 1 stop bit
	// turn on binary mode (bit 0), DTR (bit 4) and RTS (bit 12)
	d.Flags = 0x0001 | 0x0010 | 0x1000
	r, _, err = procSetCommState.Call(uintptr(h), uintptr(unsafe.Pointer(&d)))
	if r == 0 {
		syscall.CloseHandle(h)
		return nil, fmt.Errorf("SetCommState: %v", err)
	}

	// read returns after 1 sec even if nothing came, so our loop doesnt hang forever
	t := commTimeouts{
		ReadIntervalTimeout:       50,
		ReadTotalTimeoutConstant:  1000,
		WriteTotalTimeoutConstant: 1000,
	}
	r, _, err = procSetCommTimeouts.Call(uintptr(h), uintptr(unsafe.Pointer(&t)))
	if r == 0 {
		syscall.CloseHandle(h)
		return nil, fmt.Errorf("SetCommTimeouts: %v", err)
	}

	return os.NewFile(uintptr(h), name), nil
}
