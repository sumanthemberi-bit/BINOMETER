package main

import "time"

// Bin is one smart dustbin placed somewhere on campus
type Bin struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Zone      string  `json:"zone"`
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	DepthCM   int     `json:"depth_cm"` // inside height of the bin, used for fill %
	Installed string  `json:"installed"`

	// latest values, updated every time the bin sends a reading
	FillPercent int       `json:"fill_percent"`
	Battery     float64   `json:"battery"`
	LidOpens    int       `json:"lid_opens"` // lid opens counted today
	LastSeen    time.Time `json:"last_seen"`
	Status      string    `json:"status"` // ok, half, full, offline

	// live values, only filled for bins with real hardware connected
	Source  string `json:"source"`   // usb for the Uno on a cable, wifi for a NodeMCU, empty when simulated or not connected
	LidOpen bool   `json:"lid_open"` // is the lid open right now
	HandCM  int    `json:"hand_cm"`  // latest reading of the front sensor
	Real    bool   `json:"real"`     // true once real hardware has connected, simulator skips these
	Custom  bool   `json:"custom"`   // added through POST /api/bins (not in seed.go)
}

// Reading is what the ESP8266 on each bin posts to the server
type Reading struct {
	BinID      string    `json:"bin_id"`
	DistanceCM int       `json:"distance_cm"` // from 2nd ultrasonic sensor under the lid
	Battery    float64   `json:"battery"`     // volts
	LidOpens   int       `json:"lid_opens"`   // opens since last reading
	Time       time.Time `json:"time"`
}

// Alert is created when something needs attention
type Alert struct {
	ID       int       `json:"id"`
	BinID    string    `json:"bin_id"`
	BinName  string    `json:"bin_name"`
	Type     string    `json:"type"` // FULL, LOW_BATTERY, OFFLINE
	Message  string    `json:"message"`
	Created  time.Time `json:"created"`
	Resolved bool      `json:"resolved"`
}

// ZoneSummary is used on the dashboard to compare areas of campus
type ZoneSummary struct {
	Zone        string `json:"zone"`
	TotalBins   int    `json:"total_bins"`
	FullBins    int    `json:"full_bins"`
	AvgFill     int    `json:"avg_fill"`
	OpensToday  int    `json:"opens_today"`
	OfflineBins int    `json:"offline_bins"`
}
