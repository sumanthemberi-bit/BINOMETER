// Smart Dustbin, NodeMCU (ESP8266) version for bins placed around campus
//
// It does the same lid work as the Uno version, and also:
//   reads the fill level with a second ultrasonic sensor under the lid
//   sends the fill level and battery voltage to the server every 30 seconds over WiFi
//   tells the server right away when the lid opens or closes, so the dashboard updates live
//   turns on an LED on D7 when the server says the bin is full
//
// The Uno has no WiFi, which is why this part uses a NodeMCU.
// The HC SR04 echo pin gives 5V but NodeMCU pins take 3.3V,
// so put a voltage divider (1k and 2k resistors) on both echo pins.

#include <ESP8266WiFi.h>
#include <ESP8266HTTPClient.h>
#include <Servo.h>

const char* ssid = "YOUR_WIFI";
const char* password = "YOUR_PASSWORD";
// IP address of the computer running the server (run ipconfig on it to find this)
const char* server = "http://192.168.1.10:8080/api/readings";
const char* eventURL = "http://192.168.1.10:8080/api/events";
const char* apiKey = "vitcbins2026";   // same as BIN_API_KEY on the server
const char* binID = "GZ01";            // must match an id in seed.go

// front sensor, for the hand
#define trig1 D1
#define echo1 D2
// inside sensor, for the fill level
#define trig2 D5
#define echo2 D6
#define servoPin D4
#define ledPin D7   // turns on when the server says the bin is full

Servo myServo;
bool lidOpen = false;
unsigned long lastSend = 0;
unsigned long lastSeenHand = 0;

// sends a POST and gives back the reply text, or an empty string if it failed
String postJSON(const char* url, String body) {
  if (WiFi.status() != WL_CONNECTED) return "";
  WiFiClient client;
  HTTPClient http;
  http.begin(client, url);
  http.addHeader("Content-Type", "application/json");
  http.addHeader("X-API-Key", apiKey);
  int code = http.POST(body);
  String resp = "";
  if (code == 200) resp = http.getString();
  Serial.println("sent " + body + " got " + String(code));
  http.end();
  return resp;
}

// the server replies with bin_status, light the LED if the bin is full
void checkReply(String resp) {
  if (resp.indexOf("\"bin_status\":\"full\"") >= 0) {
    digitalWrite(ledPin, HIGH);
  } else if (resp.length() > 0) {
    digitalWrite(ledPin, LOW);
  }
}

void sendLidEvent(String ev, int hand) {
  String body = "{\"bin_id\":\"" + String(binID) + "\",\"event\":\"" + ev + "\",\"hand_cm\":" + String(hand) + "}";
  checkReply(postJSON(eventURL, body));
}

int readDistance(int trig, int echo) {
  digitalWrite(trig, LOW);
  delayMicroseconds(2);
  digitalWrite(trig, HIGH);
  delayMicroseconds(10);
  digitalWrite(trig, LOW);
  long duration = pulseIn(echo, HIGH, 30000); // wait at most 30 ms
  if (duration == 0) return 400;              // nothing detected
  return duration * 0.034 / 2;
}

float readBattery() {
  // 9V battery goes through a divider (10k and 2.2k) into A0.
  // A0 on the NodeMCU takes at most 3.3V. Check this value with a multimeter.
  int raw = analogRead(A0);
  return raw * (3.3 / 1023.0) * 5.55;
}

void setup() {
  Serial.begin(9600);
  pinMode(trig1, OUTPUT); pinMode(echo1, INPUT);
  pinMode(trig2, OUTPUT); pinMode(echo2, INPUT);
  pinMode(ledPin, OUTPUT);
  myServo.attach(servoPin);
  myServo.write(0);

  WiFi.begin(ssid, password);
  Serial.print("connecting");
  while (WiFi.status() != WL_CONNECTED) {
    delay(500);
    Serial.print(".");
  }
  Serial.println(" connected");
}

void loop() {
  // lid part, same as the Uno version with a 3 second hold
  int hand = readDistance(trig1, echo1);
  if (hand <= 20) {
    lastSeenHand = millis();
    if (!lidOpen) {
      myServo.write(90);
      lidOpen = true;
      sendLidEvent("open", hand);   // the dashboard shows it right away
    }
  }
  if (lidOpen && millis() - lastSeenHand > 3000) {
    myServo.write(0);
    lidOpen = false;
    sendLidEvent("close", hand);
  }

  // send to the server every 30 seconds, only with the lid closed or the reading is wrong
  if (!lidOpen && millis() - lastSend > 30000) {
    int fill = readDistance(trig2, echo2);
    float bat = readBattery();

    // lid opens were already sent as events, so lid_opens is 0 here
    String body = "{\"bin_id\":\"" + String(binID) + "\",\"distance_cm\":" + String(fill) +
                  ",\"battery\":" + String(bat, 2) + ",\"lid_opens\":0}";
    checkReply(postJSON(server, body));
    lastSend = millis();
  }

  delay(100);
}
