//go:build !windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// Linux and Mac version. The Uno shows up as /dev/ttyACM0 (original board) or /dev/ttyUSB0 (CH340 clones).
// We use the stty command to set the baud rate, simpler than doing it in code.

func candidatePorts() []string {
	ports := []string{}
	for _, pattern := range []string{"/dev/ttyACM*", "/dev/ttyUSB*", "/dev/cu.usbmodem*", "/dev/cu.usbserial*"} {
		found, _ := filepath.Glob(pattern)
		ports = append(ports, found...)
	}
	return ports
}

func openPort(name string, baud int) (*os.File, error) {
	flag := "-F"
	if _, err := exec.LookPath("stty"); err == nil {
		// mac uses -f instead of -F
		if filepath.HasPrefix(name, "/dev/cu.") {
			flag = "-f"
		}
		cmd := exec.Command("stty", flag, name, strconv.Itoa(baud), "raw", "-echo")
		if err := cmd.Run(); err != nil {
			return nil, err
		}
	}
	return os.OpenFile(name, os.O_RDWR, 0)
}
