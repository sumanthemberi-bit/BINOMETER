# Smart Dustbin Monitoring for VIT Chennai

This is the second half of our MDP project. In the first half we built a dustbin with an Arduino Uno that opens its lid on its own when you bring your hand close. In this half we connect bins to one central server, so the housekeeping team can see from one screen which bins on campus are full, which ones need a new battery and which ones have stopped working.

The server is written in Go and uses only the standard library. The dashboard is a single web page with a map of the campus. Our real bin talks to the server over a USB cable, and bins placed around campus would talk to it over WiFi using a NodeMCU board.

We only built one physical bin, so the other twenty bins on the map can be filled with made up readings by a simulator. The dashboard always shows clearly which data is real and which is simulated.

## What it does

The server keeps track of 21 bins placed around the campus, in the academic blocks, Gazebo, North Square, the hostels, MG Auditorium, the main gate and a few other places. For each bin it knows how full it is, its battery voltage, how many times the lid was opened today and when it last sent data.

When a bin reaches 80 percent it raises a FULL alert. It also raises an alert when the battery drops below 7 volts or when a bin goes quiet for five minutes. When a bin is emptied the FULL alert closes on its own.

When a real bin is connected, the dashboard shows the lid opening and closing and the hand distance as it happens, without refreshing the page. When a bin becomes full, the server tells the bin and the bin lights an LED and beeps.

There is also a cleaning route that lists the bins above 60 percent, fullest first, so the staff know where to go.

## Running it on your laptop

You need Go 1.22 or newer, which you can download from go.dev/dl. After installing it, open a new PowerShell window, go into the project folder (the one with go.mod in it) and run:

```
go run . -sim -serial auto
```

Then open http://localhost:8080 in a browser.

The sim option turns on the simulator for the bins that have no hardware. The serial option tells the server to look for the Arduino on any USB port. You can use either one on its own or both together.

```
go run . -sim                 only made up data, no hardware needed
go run . -serial auto         only real data from the Arduino
go run . -serial COM3         look for the Arduino on one port only
```

To stop the server press Ctrl and C in the PowerShell window. It saves its data before it closes.

## Connecting the Arduino Uno

Open bin_uno/bin_uno.ino in the Arduino IDE and upload it to the Uno. The pins are the same as in our report, the ultrasonic sensor trigger on pin 9, echo on pin 11 and the servo on pin 10.

After uploading, close the Serial Monitor in the Arduino IDE. Only one program can use the COM port at a time, so the server cannot see the Uno while the Serial Monitor is open.

Start the server with the serial option set to auto and plug in the Uno. Within a few seconds the server prints that the bin has connected and the dashboard shows it in the Connected Hardware panel. You can unplug it and plug it back in while the server is running.

To see the fill level of the real bin you need a second ultrasonic sensor fixed under the lid, pointing down, with trigger on pin 7 and echo on pin 8. Without it the server still sees the lid and the hand, but it cannot tell how full the bin is.

## Settings

Every setting can be given as an option when starting the program or as an environment variable. Options are easier on a laptop and environment variables are easier on a server.

```
PORT                 port for the website, 8080 if not set
DATA_DIR             folder for data.json and readings.csv, the current folder if not set
SIMULATOR            set to on to turn on the simulator
SERIAL               set to auto, or a port name like COM3, to read an Arduino over USB
BIN_API_KEY          key the bins must send with their data
DASHBOARD_PASSWORD   if set, the dashboard asks for a login with the user name admin
```

If BIN_API_KEY is not set the server uses a default key and prints a warning. Always set your own key and a dashboard password before putting the server on a shared network.

## Deploying on a server

The dashboard is packed inside the program, so a deployment is just one file plus a folder for the data. There are two ways we have set up.

### With Docker

```
docker compose up -d
```

Edit compose.yml first and put your own API key and dashboard password in it. The data is kept in a Docker volume so it survives restarts. Docker cannot reach USB ports on Windows, so this way is for bins that send data over WiFi.

### On a Linux server with systemd

Build the program for Linux, either with build.ps1 on Windows or with this command on the server:

```
go build -o binmonitor .
```

Then follow the steps written at the top of deploy/binmonitor.service. They create a user for the program, copy it into place and start it as a service that comes back on its own after a reboot or a crash.

### Pointing the WiFi bins at the server

In bin_esp8266/bin_esp8266.ino, set your WiFi name and password, the IP address of the server, the same API key as on the server and the id of the bin from seed.go. Upload it to the NodeMCU and it starts sending data every 30 seconds, plus a message every time the lid opens or closes.

## If something goes wrong

If Windows says an Application Control policy has blocked the program, Smart App Control or a policy from your college or company is stopping unsigned programs. Running with go run usually still works. If it does not, run the project on another laptop.

If the server keeps saying no bin was found, check that the Serial Monitor is closed, that the Uno shows up in Device Manager under Ports, and that the BIN_ID at the top of the sketch matches an id in seed.go. Cheap Uno clones use a CH340 chip and need its driver installed on Windows.

If the map is blank, the computer has no internet. The map tiles come from OpenStreetMap. Everything else on the page still works without internet.

## Project files

```
main.go              starts the server and reads the settings
handlers.go          the web API
store.go             keeps the bins and alerts, works out fill levels, saves to disk
events.go            sends live updates to the dashboard
hardware.go          reads the Arduino over USB
serial_windows.go    opens a COM port on Windows
serial_other.go      opens a serial port on Linux and Mac
simulator.go         makes up readings for bins without hardware
seed.go              the list of bins and where they are on campus
models.go            the data types
static/index.html    the dashboard
bin_uno              Arduino sketch for our MDP bin over USB
bin_esp8266          Arduino sketch for WiFi bins around campus
Dockerfile           builds a small Docker image
compose.yml          runs the image with Docker Compose
deploy               systemd service file for a Linux server
build.ps1            builds the program for Windows and Linux
DESIGN.md            how the system is designed and why
```

## Team

Jagannath Chandrasekar, Srikumaran M, Emberi Sumanth Bhrugumaharshi, Arpit Juyal and Praghya Rana, School of Electrical Engineering, VIT Chennai. Guided by Dr. Sourabh Paul.

## Licence

MIT, see the LICENSE file.
