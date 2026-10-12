package main

import (
	"errors"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// This file reads the Arduino Uno over the USB cable.
//
// The Uno prints one message per line (see bin_uno/bin_uno.ino):
//   BIN,AB101    which bin this is, sent at startup and every 5 sec
//   HAND,14      front sensor distance in cm, every half second
//   OPEN         lid opened
//   CLOSE        lid closed
//   FILL,23      distance from lid to garbage in cm, every 5 sec, only if the second sensor is fitted
//
// The server sends back:
//   FULL         bin is 80% or more, the Uno turns on the LED and buzzer
//   OK           bin is fine again after emptying

const baudRate = 9600

// how long to wait for the Uno to send its BIN line after opening the port.
// opening the port resets the Uno, which takes about 2 sec to boot
const handshakeTime = 5 * time.Second

// if nothing comes for this long we assume the cable was pulled out
const silenceLimit = 10 * time.Second

type serialConn struct {
	file  *os.File
	binID string
	port  string
	mu    sync.Mutex // so two goroutines dont write at the same time
}

func (c *serialConn) send(msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.file.Write([]byte(msg + "\n"))
	if err != nil {
		log.Println("usb: write failed:", err)
	}
}

// runSerial keeps looking for a bin on USB. When the cable is pulled out
// it goes back to looking, so you can plug it in again any time.
// portName is "auto" to try all ports, or a fixed one like "COM3".
func runSerial(store *Store, hub *Hub, portName string) {
	printedWaiting := false
	for {
		ports := []string{portName}
		if portName == "auto" {
			ports = candidatePorts()
		}

		conn := findBin(store, ports)
		if conn == nil {
			if !printedWaiting {
				log.Println("usb: no bin found yet, plug in the Arduino (checking every 3 sec)")
				printedWaiting = true
			}
			time.Sleep(3 * time.Second)
			continue
		}
		printedWaiting = false

		log.Printf("usb: bin %s connected on %s", conn.binID, conn.port)
		store.SetConnected(conn.binID, "usb", true)

		handleConnection(store, hub, conn)

		conn.file.Close()
		log.Printf("usb: bin %s disconnected", conn.binID)
		store.SetConnected(conn.binID, "usb", false)
		time.Sleep(2 * time.Second)
	}
}

// findBin opens each port and waits for a BIN line
func findBin(store *Store, ports []string) *serialConn {
	for _, p := range ports {
		f, err := openPort(p, baudRate)
		if err != nil {
			continue // port doesnt exist or is being used (close Arduino Serial Monitor!)
		}

		r := newLineReader(f)
		deadline := time.Now().Add(handshakeTime)
		for time.Now().Before(deadline) {
			line, err := r.readLine()
			if err != nil {
				break
			}
			if strings.HasPrefix(line, "BIN,") {
				id := strings.TrimSpace(strings.TrimPrefix(line, "BIN,"))
				if !store.HasBin(id) {
					log.Printf("usb: %s says it is bin %q but that bin is not in the list (seed.go)", p, id)
					break
				}
				return &serialConn{file: f, binID: id, port: p}
			}
		}
		f.Close()
	}
	return nil
}

func handleConnection(store *Store, hub *Hub, conn *serialConn) {
	// tell the bin its current status right away
	if store.GetStatus(conn.binID) == "full" {
		conn.send("FULL")
	} else {
		conn.send("OK")
	}

	// listen for status changes and send them to the Uno
	events := hub.Subscribe()
	stop := make(chan bool)
	go func() {
		for {
			select {
			case e := <-events:
				if e.Type == "status" && e.BinID == conn.binID {
					if e.Data == "full" {
						log.Printf("usb: telling %s it is FULL", conn.binID)
						conn.send("FULL")
					} else if e.Data != "offline" {
						conn.send("OK")
					}
				}
			case <-stop:
				return
			}
		}
	}()
	defer func() {
		hub.Unsubscribe(events)
		close(stop)
	}()

	r := newLineReader(conn.file)
	lastData := time.Now()
	for {
		line, err := r.readLine()
		if err != nil {
			log.Println("usb: read error:", err)
			return
		}
		if line == "" {
			// nothing came within the timeout
			if time.Since(lastData) > silenceLimit {
				log.Println("usb: no data for", silenceLimit)
				return
			}
			continue
		}
		lastData = time.Now()
		handleLine(store, conn.binID, line)
	}
}

func handleLine(store *Store, binID, line string) {
	parts := strings.Split(line, ",")
	cmd := strings.ToUpper(strings.TrimSpace(parts[0]))

	switch cmd {
	case "BIN":
		store.Heartbeat(binID)
	case "OPEN":
		log.Printf("usb: %s lid OPEN", binID)
		store.LidEvent(binID, true)
	case "CLOSE":
		log.Printf("usb: %s lid CLOSE", binID)
		store.LidEvent(binID, false)
	case "HAND":
		if len(parts) < 2 {
			return
		}
		cm, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err == nil {
			store.SetHand(binID, cm)
		}
	case "FILL":
		if len(parts) < 2 {
			return
		}
		cm, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || cm < 2 || cm > 400 {
			return // bad reading, ignore
		}
		// lid opens are already counted from OPEN messages so LidOpens = 0 here
		err = store.AddReading(Reading{BinID: binID, DistanceCM: cm, LidOpens: 0, Time: time.Now()})
		if err != nil {
			log.Println("usb:", err)
		}
	default:
		// probably some debug print from the Arduino, just show it
		log.Printf("usb: %s says: %s", binID, line)
	}
}

// lineReader reads one line at a time from the serial port.
// readLine returns "" with no error if nothing came for about 1 second.
type lineReader struct {
	f   *os.File
	buf []byte
}

func newLineReader(f *os.File) *lineReader {
	return &lineReader{f: f}
}

func (r *lineReader) readLine() (string, error) {
	tmp := make([]byte, 128)
	start := time.Now()
	for {
		// check if we already have a full line
		for i, c := range r.buf {
			if c == '\n' {
				line := strings.TrimSpace(string(r.buf[:i]))
				r.buf = r.buf[i+1:]
				return line, nil
			}
		}
		if time.Since(start) > time.Second {
			return "", nil
		}

		// on linux this makes Read return after 1 sec, on windows the
		// timeout is set in openPort instead (this call just fails there)
		r.f.SetReadDeadline(time.Now().Add(time.Second))

		n, err := r.f.Read(tmp)
		if n > 0 {
			r.buf = append(r.buf, tmp[:n]...)
			if len(r.buf) > 1024 {
				r.buf = nil // garbage on the line (wrong baud rate?), throw it away
			}
		}
		if err != nil {
			// timeout or "nothing to read" is normal, anything else means cable removed
			if errors.Is(err, os.ErrDeadlineExceeded) || err == io.EOF {
				continue
			}
			return "", err
		}
	}
}
