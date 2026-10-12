# Design of the Bin Monitoring System

This document explains how the system is put together and why we made the choices we did. It is written for our review panel and for anyone who wants to change the code later.

## 1. The problem

VIT Chennai has dustbins in the academic blocks, the food courts, the hostels, near the main gate, in the parking area and in many other places. Right now the housekeeping staff empty them on a fixed timetable. Busy bins like the ones at Gazebo and North Square overflow before the staff arrive, while quiet bins like the ones at the library or the admin block get emptied when they are almost empty. Nobody knows which bins are full at any given moment.

We wanted one place where anyone in housekeeping can see the fill level of every bin on campus and get told when a bin needs to be emptied.

## 2. What the system has to do

1. Each bin sends its fill level, battery voltage and number of lid opens to the server.
2. The server keeps the latest state of every bin.
3. The server raises an alert when a bin is 80 percent full, when its battery is low, or when it stops sending data.
4. The dashboard shows every bin on a map of the campus, coloured by how full it is.
5. The dashboard shows a summary for each zone: Academic, Food, Hostel and Other.
6. The staff can mark an alert as done.
7. The system gives a list of bins to collect, fullest first.
8. When a real bin is connected, the dashboard shows the lid opening and closing and the hand distance as it happens, without refreshing the page.
9. When a bin becomes full, the server tells the bin, and the bin lights an LED and beeps.

There are also a few things the system has to be, rather than do. It has to handle about 21 bins now and maybe 200 if the whole campus is covered. Each bin sends data every 30 seconds, so even 200 bins means fewer than 7 requests a second, which is a very light load. It has to keep its data if it restarts. And it has to be cheap to run, on a laptop for the demo or on one small server for real use.

## 3. How it fits together

A bin can reach the server in two ways.

Our MDP prototype is an Arduino Uno, which has no WiFi. For the demo we plug it into the laptop running the server with a USB cable. The Uno and the server send short lines of text to each other over the cable.

Bins placed around campus would use a NodeMCU board instead, which has WiFi. They send their data to the server as JSON over plain HTTP.

The server itself is one Go program made of a few parts.

1. hardware.go finds the Uno on a USB port, reads its messages and sends FULL or OK back to it.
2. handlers.go is the web API that the WiFi bins and the dashboard use.
3. store.go holds all the bins and alerts in memory, works out fill levels and decides when to raise an alert. It saves everything to data.json every 30 seconds and writes every reading to readings.csv.
4. events.go passes every change on to anyone who is listening, which means the open dashboard pages and the USB reader.
5. simulator.go makes up readings for the bins that have no hardware, when demo mode is on.

The dashboard is one web page that keeps a connection open to the server and redraws itself whenever something changes.

### What happens when someone uses the bin

1. A hand comes within 20 cm of the front sensor and the Uno opens the lid.
2. The Uno prints OPEN on the USB line.
3. The server reads the line, marks the lid as open and counts one more lid open for today.
4. The server passes the change to every open dashboard page.
5. On the dashboard the lid box turns to OPEN, the bin's dot on the map grows and its row in the table flashes.

In our testing all of this took well under a second.

### What happens when the bin gets full

1. Every 5 seconds the Uno measures the distance from the lid to the garbage and prints it, for example FILL,8.
2. The server works out the fill level. For a bin 60 cm deep, 8 cm of empty space means the bin is 87 percent full.
3. That is over 80 percent, so the bin's status becomes full and a FULL alert is raised.
4. The USB reader hears about the change and sends FULL to the Uno.
5. The Uno turns on its LED and beeps once.
6. After the staff empty the bin, the next reading shows a low fill level, the alert closes on its own and the server sends OK, which turns the LED off.

## 4. Why we chose what we chose

We used Go because it builds into a single program that runs on any laptop or server, and its standard library already has a good web server, so we did not need a framework.

We used plain HTTP and JSON for the WiFi bins instead of MQTT because it is easier to test with curl and Postman, and the load is so low that MQTT would not help much. MQTT is in our future work list.

We keep the data in memory and save it to a JSON file instead of using a database. With 21 bins a database would only add setup work for the demo. All the storage code sits in one file, so it can be swapped for a database later without touching the rest.

We also write every reading to a CSV file because we wanted to plot how each zone fills up during the day in Excel for our report.

For live updates we use server sent events. Our first version made the dashboard ask the server for new data every 5 seconds, but then a lid opening showed up on screen several seconds late. Server sent events only need the standard library and the browser reconnects on its own if the connection drops. WebSockets would have needed an extra library.

The USB serial code is written by hand. We tried to use a serial library, but the Go package proxy was blocked on our network, so we call the Windows serial functions directly. It is about a hundred lines.

The messages between the Uno and the server are simple text lines like HAND,14 instead of JSON, because they are easy to read in the Serial Monitor while debugging and the Uno only has 2 KB of memory.

We used the NodeMCU for the campus bins because it is cheap, around 250 rupees, has WiFi built in and can run the same servo code as the Uno.

The dashboard is packed inside the program when it is built, so deploying the system means copying one file.

## 5. Changes to the bin hardware

We added a second ultrasonic sensor under the lid, pointing down at the garbage. The fill level is worked out like this:

```
fill percent = 100 minus (distance to garbage divided by bin depth, times 100)
```

So for a bin 60 cm deep, if the sensor reads 12 cm, the bin is 100 minus 20, which is 80 percent full.

The fill reading is only taken while the lid is closed. When the lid is open the sensor points at the wall or the ceiling and the reading is wrong.

On the NodeMCU the battery voltage is read through a voltage divider made of a 10k and a 2.2k resistor into pin A0. The Uno version runs from the USB cable during the demo, so it does not report a battery voltage.

## 6. Messages between the Uno and the server

Each message is one line of text at 9600 baud.

The Uno sends these:

```
BIN,AB101    which bin this is, at startup and every 5 seconds. It also tells the server the bin is still there.
HAND,14      distance from the front sensor in cm, every half second. 400 means nothing is in front.
OPEN         the lid just opened
CLOSE        the lid just closed
FILL,23      distance from the lid to the garbage in cm, every 5 seconds, only if the second sensor is fitted
```

The server sends these:

```
FULL         the bin is 80 percent full or more, turn on the LED and beep
OK           the bin is not full
```

This is how the server finds the Uno when the serial option is set to auto.

1. It tries every port from COM1 to COM30. On Linux it tries the ttyACM and ttyUSB devices.
2. Opening a port restarts the Uno, so it waits up to 5 seconds for a line starting with BIN.
3. If the bin id is in our list, that is our bin. If not, it closes the port and tries the next one.
4. If no data comes for 10 seconds, it takes that to mean the cable was pulled out. It marks the bin offline, raises an OFFLINE alert and goes back to step 1.

This means the Uno can be plugged in or pulled out at any time while the server is running.

## 7. The bins on campus

There are 21 bins in four zones. The coordinates are rough, we got them by dropping pins on the map.

1. Academic: AB1 ground floor lobby, AB1 third floor corridor, AB2 entrance, AB2 lab wing, AB3 entrance and the Central Library entrance.
2. Food: Gazebo food court left and right, North Square entrance and North Square seating area.
3. Hostel: Mens Hostel A, D, E and F blocks, the Ladies Hostel entrance and the hostel mess.
4. Other: Admin block reception, main gate, sports complex, MG Auditorium and the two wheeler parking.

The full list with ids is in seed.go. The bin list is loaded from seed.go every time the server starts, so if a location is renamed there, the new name shows up even with an old data.json. Readings that were already saved are kept. Bins added through the API are kept as well.

## 8. What the server stores

For every bin the server keeps its id (like NS01), its name, its zone, its position on the map, how deep it is in cm, how full it is, its battery voltage, how many times the lid was opened today, when it last sent data and its status.

The status is ok when the bin is under 50 percent full, half from 50 to 79 percent, full at 80 percent or more, and offline when the bin has stopped sending data.

For bins with real hardware it also keeps whether the lid is open right now, the latest hand distance, and how the bin is connected (usb or wifi).

A reading from a bin has the bin id, the distance to the garbage, the battery voltage and the number of lid opens since the last reading.

An alert has an id, the bin it is about, its type (FULL, LOW BATTERY or OFFLINE), a message, when it was raised and whether it has been dealt with.

## 9. The web API

```
POST /api/readings              a bin sends a reading, needs the API key
POST /api/events                a bin says its lid opened or closed, needs the API key
GET  /api/bins                  all bins, can be filtered with ?zone=Food or ?status=full
GET  /api/bins/AB101            one bin and its last 50 readings
POST /api/bins                  add a new bin, needs the API key
GET  /api/alerts                open alerts, add ?all=true to include closed ones
POST /api/alerts/3/resolve      mark alert number 3 as done
GET  /api/zones                 summary for each zone
GET  /api/route                 bins above 60 percent, fullest first
GET  /api/stream                live updates for the dashboard
GET  /api/info                  tells the dashboard whether demo mode is on
GET  /health                    tells you the server is running
```

The bins send the API key in a request header with every message, as shown in the sketches. An example reading looks like this:

```
{"bin_id": "NS01", "distance_cm": 14, "battery": 8.6, "lid_opens": 5}
```

The reply to a reading or a lid event includes the bin's status, so a NodeMCU can turn on its LED when the bin is full.

## 10. When alerts are raised

A FULL alert is raised when the fill level reaches 80 percent. It closes by itself when the bin is emptied and the level drops below 80 percent again.

A LOW BATTERY alert is raised when the battery drops below 7 volts. A 9 volt battery stops driving the servo properly at around 6.5 volts, so this gives the staff some time to replace it.

An OFFLINE alert is raised when a bin sends nothing for 5 minutes. Bins send every 30 seconds, so that is 10 missed readings in a row. For the Uno on USB it is raised right away when the cable is pulled out.

The server never raises the same alert twice for the same bin while the first one is still open. Our first version raised a new FULL alert every 30 seconds and the screen filled up with copies of the same alert.

## 11. The demo section, what is real and what is simulated

We only built one physical bin for our MDP, the Uno bin with the id AB101. The other 20 bins on the map have no sensors. To show what the system would look like with the whole campus covered, we wrote a simulator. The simulator data is not real, and the dashboard says so clearly.

These parts of the dashboard show real data from the Uno: the Connected Hardware panel with the lid state and hand distance, the Real Bins table, and the fill level of AB101 if the second sensor is fitted.

These parts show simulated data: the Demo Section table with the other 20 bins, and their fill levels, battery voltages and lid opens. The zone summary, the alerts and the cleaning route are worked out from all bins, so they mix both.

The simulator works like this.

1. Food court bins fill the fastest. Academic bins fill between 8 in the morning and 5 in the evening, and hostel bins fill in the evening.
2. Each lid open adds about 1 cm of garbage.
3. When a bin is nearly full it gets emptied at random, as if the cleaning staff came by.
4. The parking bin skips readings on purpose so the OFFLINE alert can be shown.
5. It never touches a bin that has real hardware connected, or that had hardware connected earlier.

This is how the dashboard keeps the two apart.

1. A yellow banner saying DEMO MODE IS ON appears at the top whenever the simulator is running.
2. The bin list is split into Real Bins and a Demo Section with a dashed border.
3. Every bin has a tag saying where its data comes from: REAL USB, REAL WIFI, REAL NOT CONNECTED, SIMULATED or NO HARDWARE.
4. On the map, simulated bins have a dashed outline and real bins have a thick one.

During the review we tell the panel plainly that AB101 is our real bin and the other 20 are simulated to show how the idea scales to the whole campus.

## 12. Deployment

The program can run in three ways.

1. On a laptop for the demo, started from PowerShell with go run.
2. In Docker, using the Dockerfile and compose.yml. The data is kept in a Docker volume.
3. As a systemd service on a Linux server, using deploy/binmonitor.service. It starts on boot and restarts itself if it crashes.

Settings come from environment variables: the port, the data folder, the API key, the dashboard password, whether the simulator is on and which serial port to use. The same settings can also be given as options on the command line.

When the program is stopped by Ctrl and C, by Docker or by systemd, it saves its data before it exits.

If DASHBOARD_PASSWORD is set, the dashboard and the admin actions ask for a login. The routes the bins use stay open to them because they already check the API key.

## 13. A rough check on size

One reading is about 100 bytes of JSON. With 21 bins sending twice a minute, that is around 60,000 readings a day, or about 4 MB of CSV a day. With 200 bins it would be about 40 MB a day. That is fine for a few months, after which the CSV file would need to be rotated or the data moved into a database. The Go web server handles 200 bins sending every 30 seconds without any trouble.

## 14. What is not good yet

1. The data is kept in a JSON file, not a real database. This will not hold up if the number of bins grows a lot.
2. All bins share one API key. Anyone who gets hold of it can send fake readings for any bin.
3. The dashboard password is a single shared login, there are no separate accounts for staff.
4. The bins depend on the campus WiFi. If the WiFi is down, readings from that time are lost because the NodeMCU does not store them yet.
5. The fill sensor can give a wrong reading if the garbage is piled up on one side of the bin.
6. The cleaning route is just sorted by fill level, not by the walking distance between bins.
7. The bin coordinates are rough.
8. Only one real bin was built and tested. The other 20 are simulated, as described in section 11.
9. The USB connection only works for one bin sitting next to the server. A real rollout needs the WiFi version.
10. The Windows serial code was checked by building it, and the reading logic was tested with a fake serial port on Linux. It has not yet been run against a real COM port on many different machines.
11. The map needs internet for the map tiles. Without internet the map is blank, but the rest of the page works.

## 15. What we would add next

1. Move the data into PostgreSQL or SQLite, with a proper table of readings, so the history can be searched.
2. Use MQTT instead of HTTP so the bins use less power.
3. Give every bin its own API key.
4. Add separate logins for the admin and the housekeeping staff.
5. Send alerts to the housekeeping supervisor on WhatsApp, SMS or Telegram.
6. Plan the cleaning route using the actual distance between bins.
7. Let the NodeMCU store readings while the WiFi is down and send them later.
8. Use the CSV data to predict when each bin will be full. For example, the Gazebo bins seem to fill up by 1 in the afternoon every day.
9. Put a small solar panel on the outdoor bins at the gate, the parking and the sports complex.
