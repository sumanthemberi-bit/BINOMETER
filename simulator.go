package main

import (
	"log"
	"math/rand"
	"time"
)

// runSimulator pretends to be all the bins on campus.
// We only built 1 real bin for MDP so this is used to test the dashboard.
// Food court bins fill faster, hostel bins are busy at night, academic in day time.
func runSimulator(store *Store) {
	type simBin struct {
		distance int
		battery  float64
	}
	state := map[string]*simBin{}
	for _, b := range store.GetBins() {
		state[b.ID] = &simBin{distance: b.DepthCM - rand.Intn(10), battery: 9.0}
	}

	// one bin will stop sending data to test the offline alert
	deadBin := "PARK01"

	for {
		hour := time.Now().Hour()
		for _, b := range store.GetBins() {
			// dont touch bins that have real hardware connected
			if b.Real {
				continue
			}
			if b.ID == deadBin && rand.Intn(100) < 70 {
				continue
			}
			st, ok := state[b.ID]
			if !ok {
				st = &simBin{distance: b.DepthCM, battery: 9.0}
				state[b.ID] = st
			}

			opens := rand.Intn(3)
			switch b.Zone {
			case "Food":
				opens += rand.Intn(5)
			case "Academic":
				if hour >= 8 && hour <= 17 {
					opens += rand.Intn(3)
				}
			case "Hostel":
				if hour >= 18 || hour <= 1 {
					opens += rand.Intn(4)
				}
			}

			// every open drops about 1cm of garbage
			st.distance -= opens
			if st.distance < 3 {
				st.distance = 3
			}

			// cleaning staff empties full bins sometimes
			if st.distance < b.DepthCM/5 && rand.Intn(10) == 0 {
				st.distance = b.DepthCM - 2
				log.Println("sim: emptied", b.ID)
			}

			st.battery -= 0.005 * float64(opens)
			if st.battery < 6.5 {
				st.battery = 9.0 // battery replaced
			}

			err := store.AddReading(Reading{
				BinID:      b.ID,
				DistanceCM: st.distance,
				Battery:    st.battery,
				LidOpens:   opens,
				Time:       time.Now(),
			})
			if err != nil {
				log.Println("sim error:", err)
			}
		}
		time.Sleep(5 * time.Second) // real bins send every 30s, sim is faster
	}
}
