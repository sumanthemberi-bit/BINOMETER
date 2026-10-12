package main

// Bins placed around VIT Chennai on Vandalur Kelambakkam Road.
// The coordinates are rough, we got them by dropping pins on the map.
// Change them if a bin is put somewhere else.

func seedBins() []Bin {
	return []Bin{
		// academic area
		{ID: "AB101", Name: "AB1 Ground Floor Lobby", Zone: "Academic", Lat: 12.84070, Lng: 80.15330, DepthCM: 60},
		{ID: "AB102", Name: "AB1 Third Floor Corridor", Zone: "Academic", Lat: 12.84075, Lng: 80.15345, DepthCM: 60},
		{ID: "AB201", Name: "AB2 Entrance", Zone: "Academic", Lat: 12.84160, Lng: 80.15420, DepthCM: 60},
		{ID: "AB202", Name: "AB2 Lab Wing", Zone: "Academic", Lat: 12.84170, Lng: 80.15440, DepthCM: 60},
		{ID: "AB301", Name: "AB3 Entrance", Zone: "Academic", Lat: 12.84250, Lng: 80.15500, DepthCM: 60},
		{ID: "LIB01", Name: "Central Library Entrance", Zone: "Academic", Lat: 12.84010, Lng: 80.15400, DepthCM: 45},

		// food areas, these fill the fastest
		{ID: "GZ01", Name: "Gazebo Food Court Left", Zone: "Food", Lat: 12.84110, Lng: 80.15260, DepthCM: 80},
		{ID: "GZ02", Name: "Gazebo Food Court Right", Zone: "Food", Lat: 12.84115, Lng: 80.15280, DepthCM: 80},
		{ID: "NS01", Name: "North Square Entrance", Zone: "Food", Lat: 12.84330, Lng: 80.15380, DepthCM: 80},
		{ID: "NS02", Name: "North Square Seating Area", Zone: "Food", Lat: 12.84340, Lng: 80.15400, DepthCM: 80},

		// hostels
		{ID: "MHA01", Name: "Mens Hostel A Block", Zone: "Hostel", Lat: 12.84380, Lng: 80.15180, DepthCM: 80},
		{ID: "MHD01", Name: "Mens Hostel D Block", Zone: "Hostel", Lat: 12.84420, Lng: 80.15220, DepthCM: 80},
		{ID: "MHE01", Name: "Mens Hostel E Block", Zone: "Hostel", Lat: 12.84460, Lng: 80.15260, DepthCM: 80},
		{ID: "MHF01", Name: "Mens Hostel F Block", Zone: "Hostel", Lat: 12.84500, Lng: 80.15300, DepthCM: 80},
		{ID: "LH01", Name: "Ladies Hostel Entrance", Zone: "Hostel", Lat: 12.83880, Lng: 80.15150, DepthCM: 80},
		{ID: "MESS01", Name: "Hostel Mess", Zone: "Hostel", Lat: 12.84400, Lng: 80.15340, DepthCM: 100},

		// other places
		{ID: "ADM01", Name: "Admin Block Reception", Zone: "Other", Lat: 12.84000, Lng: 80.15480, DepthCM: 45},
		{ID: "GATE01", Name: "Main Gate", Zone: "Other", Lat: 12.83850, Lng: 80.15550, DepthCM: 80},
		{ID: "SPT01", Name: "Sports Complex", Zone: "Other", Lat: 12.84300, Lng: 80.15600, DepthCM: 60},
		{ID: "AUD01", Name: "MG Auditorium", Zone: "Other", Lat: 12.84200, Lng: 80.15350, DepthCM: 45},
		{ID: "PARK01", Name: "Two Wheeler Parking", Zone: "Other", Lat: 12.83900, Lng: 80.15450, DepthCM: 60},
	}
}
