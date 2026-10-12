package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Store keeps everything in memory and saves to a json file.
// We used this instead of a database because there are only about 20 bins.
// If the number of bins grows a lot we should move to Postgres (see DESIGN.md).
type Store struct {
	mu          sync.Mutex
	bins        map[string]*Bin
	alerts      []Alert
	nextAlertID int
	history     map[string][]Reading // last 50 readings per bin
	dataFile    string
	csvFile     string
	hub         *Hub // to send live events to the dashboard
}

const maxHistory = 50

func NewStore(dataFile, csvFile string, hub *Hub) *Store {
	s := &Store{
		hub:         hub,
		bins:        make(map[string]*Bin),
		history:     make(map[string][]Reading),
		nextAlertID: 1,
		dataFile:    dataFile,
		csvFile:     csvFile,
	}

	// try to load old data, if not there then use seed bins
	err := s.load()
	if err != nil {
		log.Println("no saved data found, using seed bins")
		for _, b := range seedBins() {
			b := b
			b.Status = "offline"
			b.Installed = time.Now().Format("2006-01-02")
			s.bins[b.ID] = &b
		}
	}
	return s
}

type saveFormat struct {
	Bins        []*Bin  `json:"bins"`
	Alerts      []Alert `json:"alerts"`
	NextAlertID int     `json:"next_alert_id"`
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.dataFile)
	if err != nil {
		return err
	}
	var sf saveFormat
	err = json.Unmarshal(data, &sf)
	if err != nil {
		return err
	}
	saved := map[string]*Bin{}
	for _, b := range sf.Bins {
		// hardware is not connected yet when server starts
		b.Source = ""
		b.LidOpen = false
		b.HandCM = 0
		saved[b.ID] = b
	}

	// the bin list always comes from seed.go, so if we rename a location or add a bin
	// there, it shows up even with an old data.json. fill %, lid opens etc. are kept.
	for _, sb := range seedBins() {
		sb := sb
		if old, ok := saved[sb.ID]; ok {
			old.Name, old.Zone, old.Lat, old.Lng, old.DepthCM = sb.Name, sb.Zone, sb.Lat, sb.Lng, sb.DepthCM
			s.bins[sb.ID] = old
		} else {
			sb.Status = "offline"
			sb.Installed = time.Now().Format("2006-01-02")
			s.bins[sb.ID] = &sb
		}
	}
	// keep bins that were added through the API
	for id, b := range saved {
		if b.Custom {
			s.bins[id] = b
		}
	}
	s.alerts = sf.Alerts
	s.nextAlertID = sf.NextAlertID
	return nil
}

// Save writes current state to disk. Called every 30 sec from main.
func (s *Store) Save() {
	s.mu.Lock()
	sf := saveFormat{Alerts: s.alerts, NextAlertID: s.nextAlertID}
	for _, b := range s.bins {
		sf.Bins = append(sf.Bins, b)
	}
	s.mu.Unlock()

	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		log.Println("save error:", err)
		return
	}
	// write to temp file first so we dont corrupt data if program crashes mid write
	tmp := s.dataFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		log.Println("save error:", err)
		return
	}
	os.Rename(tmp, s.dataFile)
}

func (s *Store) GetBins() []Bin {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := []Bin{}
	for _, b := range s.bins {
		list = append(list, *b)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list
}

func (s *Store) GetBin(id string) (Bin, []Reading, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.bins[id]
	if !ok {
		return Bin{}, nil, false
	}
	h := make([]Reading, len(s.history[id]))
	copy(h, s.history[id])
	return *b, h, true
}

func (s *Store) AddBin(b Bin) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.bins[b.ID]; exists {
		return fmt.Errorf("bin %s already exists", b.ID)
	}
	b.Status = "offline"
	b.Installed = time.Now().Format("2006-01-02")
	b.Custom = true
	s.bins[b.ID] = &b
	return nil
}

// AddReading updates the bin and checks if any alert has to be raised
func (s *Store) AddReading(r Reading) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.bins[r.BinID]
	if !ok {
		return fmt.Errorf("unknown bin %s", r.BinID)
	}

	// fill % = how much of the depth is covered by garbage
	// sensor is at top so smaller distance = more garbage
	fill := 100 - (r.DistanceCM*100)/b.DepthCM
	if fill < 0 {
		fill = 0
	}
	if fill > 100 {
		fill = 100
	}

	// reset lid count at midnight
	if b.LastSeen.Day() != r.Time.Day() {
		b.LidOpens = 0
	}

	wasFull := b.Status == "full"
	oldStatus := b.Status

	b.FillPercent = fill
	b.Battery = r.Battery
	b.LidOpens += r.LidOpens
	b.LastSeen = r.Time
	b.Status = statusFromFill(fill)

	// history
	s.history[b.ID] = append(s.history[b.ID], r)
	if len(s.history[b.ID]) > maxHistory {
		s.history[b.ID] = s.history[b.ID][1:]
	}

	// alerts
	if b.Status == "full" && !wasFull {
		s.addAlert(b, "FULL", fmt.Sprintf("%s is %d%% full, needs to be emptied", b.Name, fill))
	}
	if b.Status != "full" {
		// bin was emptied, close the full alert automatically
		s.resolveAlerts(b.ID, "FULL")
	}
	if r.Battery > 0 && r.Battery < 7.0 {
		s.addAlert(b, "LOW_BATTERY", fmt.Sprintf("%s battery is %.1fV, replace the 9V battery", b.Name, r.Battery))
	} else {
		s.resolveAlerts(b.ID, "LOW_BATTERY")
	}
	s.resolveAlerts(b.ID, "OFFLINE")

	s.appendCSV(r, fill)

	s.hub.Publish(Event{Type: "reading", BinID: b.ID, Data: *b})
	if oldStatus != b.Status {
		s.hub.Publish(Event{Type: "status", BinID: b.ID, Data: b.Status})
	}
	return nil
}

// LidEvent is called when the hardware tells us the lid opened or closed
func (s *Store) LidEvent(binID string, open bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.bins[binID]
	if !ok {
		return fmt.Errorf("unknown bin %s", binID)
	}
	now := time.Now()
	if b.LastSeen.Day() != now.Day() {
		b.LidOpens = 0
	}
	if open && !b.LidOpen {
		b.LidOpens++
	}
	b.LidOpen = open
	s.markAlive(b, now)
	s.hub.Publish(Event{Type: "lid", BinID: b.ID, Data: map[string]interface{}{
		"open": open, "lid_opens": b.LidOpens,
	}})
	return nil
}

// SetHand stores the latest front sensor distance (for the live view)
func (s *Store) SetHand(binID string, cm int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.bins[binID]
	if !ok {
		return
	}
	b.HandCM = cm
	s.markAlive(b, time.Now())
	s.hub.Publish(Event{Type: "hand", BinID: b.ID, Data: cm})
}

// Heartbeat just means "i am still here"
func (s *Store) Heartbeat(binID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b, ok := s.bins[binID]; ok {
		s.markAlive(b, time.Now())
	}
}

// markAlive updates last seen and brings the bin back from offline. must hold lock.
func (s *Store) markAlive(b *Bin, t time.Time) {
	b.LastSeen = t
	if b.Status == "offline" {
		b.Status = statusFromFill(b.FillPercent)
		s.hub.Publish(Event{Type: "status", BinID: b.ID, Data: b.Status})
	}
	s.resolveAlerts(b.ID, "OFFLINE")
}

func (s *Store) HasBin(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.bins[id]
	return ok
}

func (s *Store) GetStatus(id string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b, ok := s.bins[id]; ok {
		return b.Status
	}
	return ""
}

// SetConnected is called when hardware is plugged in or removed
func (s *Store) SetConnected(binID, source string, connected bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.bins[binID]
	if !ok {
		return
	}
	if connected {
		b.Source = source
		b.Real = true
		s.markAlive(b, time.Now())
		s.hub.Publish(Event{Type: "connected", BinID: b.ID, Data: source})
	} else {
		b.Source = ""
		b.LidOpen = false
		b.Status = "offline"
		s.addAlert(b, "OFFLINE", fmt.Sprintf("%s was disconnected (%s)", b.Name, source))
		s.hub.Publish(Event{Type: "disconnected", BinID: b.ID, Data: source})
		s.hub.Publish(Event{Type: "status", BinID: b.ID, Data: b.Status})
	}
}

func statusFromFill(fill int) string {
	if fill >= 80 {
		return "full"
	} else if fill >= 50 {
		return "half"
	}
	return "ok"
}

// addAlert does not add a new one if same type is already open for that bin
// (otherwise we get the same alert every 30 seconds). must hold lock.
func (s *Store) addAlert(b *Bin, typ, msg string) {
	for _, a := range s.alerts {
		if a.BinID == b.ID && a.Type == typ && !a.Resolved {
			return
		}
	}
	s.alerts = append(s.alerts, Alert{
		ID:      s.nextAlertID,
		BinID:   b.ID,
		BinName: b.Name,
		Type:    typ,
		Message: msg,
		Created: time.Now(),
	})
	s.hub.Publish(Event{Type: "alert", BinID: b.ID, Data: s.alerts[len(s.alerts)-1]})
	s.nextAlertID++
	log.Println("ALERT:", typ, msg)
}

func (s *Store) resolveAlerts(binID, typ string) {
	for i := range s.alerts {
		if s.alerts[i].BinID == binID && s.alerts[i].Type == typ {
			s.alerts[i].Resolved = true
		}
	}
}

func (s *Store) GetAlerts(onlyOpen bool) []Alert {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := []Alert{}
	// newest first
	for i := len(s.alerts) - 1; i >= 0; i-- {
		if onlyOpen && s.alerts[i].Resolved {
			continue
		}
		list = append(list, s.alerts[i])
	}
	return list
}

func (s *Store) ResolveAlert(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.alerts {
		if s.alerts[i].ID == id {
			s.alerts[i].Resolved = true
			return true
		}
	}
	return false
}

// CheckOffline marks bins offline if we didnt hear from them for some time
func (s *Store) CheckOffline(limit time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, b := range s.bins {
		if b.LastSeen.IsZero() {
			continue // never connected yet
		}
		if time.Since(b.LastSeen) > limit && b.Status != "offline" {
			b.Status = "offline"
			s.hub.Publish(Event{Type: "status", BinID: b.ID, Data: b.Status})
			s.addAlert(b, "OFFLINE", fmt.Sprintf("%s has not sent data since %s", b.Name, b.LastSeen.Format("15:04")))
		}
	}
}

func (s *Store) ZoneSummary() []ZoneSummary {
	s.mu.Lock()
	defer s.mu.Unlock()

	m := map[string]*ZoneSummary{}
	totalFill := map[string]int{}
	for _, b := range s.bins {
		z, ok := m[b.Zone]
		if !ok {
			z = &ZoneSummary{Zone: b.Zone}
			m[b.Zone] = z
		}
		z.TotalBins++
		z.OpensToday += b.LidOpens
		totalFill[b.Zone] += b.FillPercent
		if b.Status == "full" {
			z.FullBins++
		}
		if b.Status == "offline" {
			z.OfflineBins++
		}
	}

	list := []ZoneSummary{}
	for name, z := range m {
		z.AvgFill = totalFill[name] / z.TotalBins
		list = append(list, *z)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Zone < list[j].Zone })
	return list
}

// appendCSV keeps a log of every reading so we can analyse it later in excel.
// must hold lock.
func (s *Store) appendCSV(r Reading, fill int) {
	_, err := os.Stat(s.csvFile)
	newFile := os.IsNotExist(err)

	f, err := os.OpenFile(s.csvFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println("csv error:", err)
		return
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if newFile {
		w.Write([]string{"time", "bin_id", "distance_cm", "fill_percent", "battery", "lid_opens"})
	}
	w.Write([]string{
		r.Time.Format(time.RFC3339),
		r.BinID,
		strconv.Itoa(r.DistanceCM),
		strconv.Itoa(fill),
		fmt.Sprintf("%.2f", r.Battery),
		strconv.Itoa(r.LidOpens),
	})
	w.Flush()
}
