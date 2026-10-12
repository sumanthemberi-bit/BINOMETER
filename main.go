package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// the dashboard is packed inside the program, so deploying only needs this one file
//
//go:embed static
var staticFiles embed.FS

// getenv reads a setting from the environment, or gives back the default
func getenv(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func main() {
	// every setting can come from a flag or an environment variable.
	// flags are easier on a laptop, environment variables are easier on a server or in Docker.
	port := flag.String("port", getenv("PORT", "8080"), "port to run on")
	dataDir := flag.String("data", getenv("DATA_DIR", "."), "folder where data.json and readings.csv are kept")
	sim := flag.Bool("sim", getenv("SIMULATOR", "") == "on", "make up readings for bins that have no hardware (demo mode)")
	serialPort := flag.String("serial", getenv("SERIAL", ""), "read the Arduino over USB, use auto to find it or give a port like COM3")
	flag.Parse()

	apiKey := os.Getenv("BIN_API_KEY")
	if apiKey == "" {
		apiKey = "vitcbins2026"
		log.Println("BIN_API_KEY is not set, using the default key. Set your own before putting this on a server.")
	}

	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		log.Fatal("cannot create data folder: ", err)
	}

	hub := NewHub()
	store := NewStore(filepath.Join(*dataDir, "data.json"), filepath.Join(*dataDir, "readings.csv"), hub)
	srv := &Server{store: store, hub: hub, apiKey: apiKey, simOn: *sim, serialPort: *serialPort}

	static, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", srv.health)
	mux.HandleFunc("GET /api/info", srv.info)
	mux.HandleFunc("POST /api/readings", srv.postReading)
	mux.HandleFunc("POST /api/events", srv.postEvent)
	mux.HandleFunc("GET /api/stream", srv.stream)
	mux.HandleFunc("GET /api/bins", srv.listBins)
	mux.HandleFunc("POST /api/bins", srv.addBin)
	mux.HandleFunc("GET /api/bins/{id}", srv.getBin)
	mux.HandleFunc("GET /api/alerts", srv.listAlerts)
	mux.HandleFunc("POST /api/alerts/{id}/resolve", srv.resolveAlert)
	mux.HandleFunc("GET /api/zones", srv.zones)
	mux.HandleFunc("GET /api/route", srv.route)
	mux.Handle("/", http.FileServer(http.FS(static)))

	// background jobs
	go func() {
		for {
			time.Sleep(30 * time.Second)
			store.Save()
		}
	}()
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			// bins send every 30 sec, so 5 min without data means something is wrong
			store.CheckOffline(5 * time.Minute)
		}
	}()

	if *serialPort != "" {
		log.Println("usb: looking for a bin on port", *serialPort)
		go runSerial(store, hub, *serialPort)
	}

	if *sim {
		log.Println("simulator is on (demo mode), bins without hardware get made up readings")
		go runSimulator(store)
	}

	// save before exiting, on Ctrl+C or when Docker or systemd stops the program
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		log.Println("saving and shutting down")
		store.Save()
		os.Exit(0)
	}()

	// DASHBOARD_PASSWORD protects the dashboard and admin actions on a shared network.
	// the bins themselves use the API key instead, so they are not affected.
	var handler http.Handler = mux
	if pw := os.Getenv("DASHBOARD_PASSWORD"); pw != "" {
		handler = passwordProtect(mux, pw)
		log.Println("dashboard password is on, user name is admin")
	}

	server := &http.Server{
		Addr:              ":" + *port,
		Handler:           logRequests(handler),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Println("server started on http://localhost:" + *port)
	log.Fatal(server.ListenAndServe())
}
