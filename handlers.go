package main

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Server struct {
	store      *Store
	hub        *Hub
	apiKey     string // the bins send this in X-API-Key header
	simOn      bool   // true when started with -sim
	serialPort string // value of -serial ("" if not used)
}

// GET /api/info tells the dashboard if demo mode (simulator) is on
func (s *Server) info(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"simulator": s.simOn,
		"serial":    s.serialPort,
	})
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// POST /api/readings  (called by the ESP8266 on each bin)
func (s *Server) postReading(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-API-Key") != s.apiKey {
		writeError(w, http.StatusUnauthorized, "wrong api key")
		return
	}

	var rd Reading
	err := json.NewDecoder(r.Body).Decode(&rd)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if rd.BinID == "" {
		writeError(w, http.StatusBadRequest, "bin_id is required")
		return
	}
	// the HC SR04 can measure from 2 cm to 400 cm, anything else is a bad reading
	if rd.DistanceCM < 2 || rd.DistanceCM > 400 {
		writeError(w, http.StatusBadRequest, "distance out of sensor range")
		return
	}
	// ESP8266 has no RTC so we just use server time
	if rd.Time.IsZero() {
		rd.Time = time.Now()
	}

	err = s.store.AddReading(rd)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.markWifi(rd.BinID)
	// bin_status is sent back so the NodeMCU can turn on its LED when full
	writeJSON(w, http.StatusOK, map[string]string{
		"status":     "saved",
		"bin_status": s.store.GetStatus(rd.BinID),
	})
}

// POST /api/events  (NodeMCU sends this when the lid opens or closes)
// body: {"bin_id": "AB101", "event": "open"}
func (s *Server) postEvent(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-API-Key") != s.apiKey {
		writeError(w, http.StatusUnauthorized, "wrong api key")
		return
	}
	var body struct {
		BinID  string `json:"bin_id"`
		Event  string `json:"event"`
		HandCM int    `json:"hand_cm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	var err error
	switch body.Event {
	case "open":
		err = s.store.LidEvent(body.BinID, true)
	case "close":
		err = s.store.LidEvent(body.BinID, false)
	default:
		writeError(w, http.StatusBadRequest, "event must be open or close")
		return
	}
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if body.HandCM > 0 {
		s.store.SetHand(body.BinID, body.HandCM)
	}
	s.markWifi(body.BinID)
	writeJSON(w, http.StatusOK, map[string]string{
		"status":     "saved",
		"bin_status": s.store.GetStatus(body.BinID),
	})
}

// first message from a NodeMCU marks the bin as a real wifi bin
func (s *Server) markWifi(binID string) {
	b, _, ok := s.store.GetBin(binID)
	if ok && b.Source == "" {
		s.store.SetConnected(binID, "wifi", true)
	}
}

// GET /api/bins   optional ?zone=Food  or ?status=full
func (s *Server) listBins(w http.ResponseWriter, r *http.Request) {
	zone := r.URL.Query().Get("zone")
	status := r.URL.Query().Get("status")

	bins := s.store.GetBins()
	result := []Bin{}
	for _, b := range bins {
		if zone != "" && !strings.EqualFold(b.Zone, zone) {
			continue
		}
		if status != "" && b.Status != status {
			continue
		}
		result = append(result, b)
	}
	writeJSON(w, http.StatusOK, result)
}

// GET /api/bins/{id}
func (s *Server) getBin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	b, history, ok := s.store.GetBin(id)
	if !ok {
		writeError(w, http.StatusNotFound, "bin not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"bin":     b,
		"history": history,
	})
}

// POST /api/bins  (admin adds a new bin)
func (s *Server) addBin(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-API-Key") != s.apiKey {
		writeError(w, http.StatusUnauthorized, "wrong api key")
		return
	}
	var b Bin
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if b.ID == "" || b.Name == "" || b.Zone == "" || b.DepthCM <= 0 {
		writeError(w, http.StatusBadRequest, "id, name, zone and depth_cm are required")
		return
	}
	if err := s.store.AddBin(b); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

// GET /api/alerts   ?all=true to also show resolved ones
func (s *Server) listAlerts(w http.ResponseWriter, r *http.Request) {
	all := r.URL.Query().Get("all") == "true"
	writeJSON(w, http.StatusOK, s.store.GetAlerts(!all))
}

// POST /api/alerts/{id}/resolve  (housekeeping staff marks it done)
func (s *Server) resolveAlert(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad alert id")
		return
	}
	if !s.store.ResolveAlert(id) {
		writeError(w, http.StatusNotFound, "alert not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
}

// GET /api/zones
func (s *Server) zones(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.ZoneSummary())
}

// GET /api/route gives the bins the cleaning staff should visit, fullest first
func (s *Server) route(w http.ResponseWriter, r *http.Request) {
	bins := s.store.GetBins()
	todo := []Bin{}
	for _, b := range bins {
		if b.FillPercent >= 60 && b.Status != "offline" {
			todo = append(todo, b)
		}
	}
	// simple bubble sort by fill, highest first
	for i := 0; i < len(todo); i++ {
		for j := 0; j < len(todo)-1-i; j++ {
			if todo[j].FillPercent < todo[j+1].FillPercent {
				todo[j], todo[j+1] = todo[j+1], todo[j]
			}
		}
	}
	writeJSON(w, http.StatusOK, todo)
}

// GET /health
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "running"})
}

// logs every request in terminal
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/api") {
			println(r.Method, r.URL.Path, time.Since(start).String())
		}
	})
}

// passwordProtect asks the browser for a user name and password (user admin, password from DASHBOARD_PASSWORD).
// the routes the bins use are left open because they already check the API key.
func passwordProtect(next http.Handler, password string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		open := r.URL.Path == "/health" ||
			(r.Method == "POST" && (r.URL.Path == "/api/readings" || r.URL.Path == "/api/events"))
		if open {
			next.ServeHTTP(w, r)
			return
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || subtle.ConstantTimeCompare([]byte(pass), []byte(password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Bin Monitor"`)
			http.Error(w, "login needed", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
