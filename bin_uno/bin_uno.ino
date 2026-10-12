// Smart Dustbin, Arduino Uno version that talks to the Go server over USB
//
// Wiring is the same as in our MDP report:
//   HC SR04 Trig on D9, Echo on D11
//   Servo signal on D10
//
// Optional parts, the code works without them:
//   Second HC SR04 under the lid for the fill level, Trig on D7, Echo on D8
//   LED with a 220 ohm resistor on D12
//   Buzzer on D4
//   The built in LED on D13 also lights up when the bin is full
//
// Messages sent to the server, one per line:
//   BIN,AB101    every 5 seconds, tells the server which bin this is
//   HAND,14      every half second, front sensor distance in cm
//   OPEN         when the lid opens
//   CLOSE        when the lid closes
//   FILL,23      every 5 seconds, only if the second sensor is connected
//
// Messages from the server:
//   FULL   turn the LED on and beep once
//   OK     turn the LED off
//
// Close the Serial Monitor in the Arduino IDE before starting the server.
// Only one program can use the COM port at a time.

#include <Servo.h>

#define BIN_ID "AB101"    // must match an id in seed.go

#define trigPin 9
#define echoPin 11
#define servoPin 10

#define fillTrig 7
#define fillEcho 8

#define ledPin 12
#define buzzerPin 4

#define THRESHOLD 20          // cm, same as in our report
#define HOLD_TIME 3000        // lid stays open 3 seconds after the hand goes away
#define LOCK_WHEN_FULL false  // set to true to keep the lid shut when the bin is full

Servo myServo;

bool lidOpen = false;
bool binFull = false;
unsigned long lastHandSeen = 0;
unsigned long lastHandPrint = 0;
unsigned long lastFillPrint = 0;
unsigned long lastIdPrint = 0;
String incoming = "";

int readDistance(int trig, int echo) {
  digitalWrite(trig, LOW);
  delayMicroseconds(2);
  digitalWrite(trig, HIGH);
  delayMicroseconds(10);
  digitalWrite(trig, LOW);
  // wait at most 30 ms, otherwise pulseIn waits a full second when nothing is there
  long duration = pulseIn(echo, HIGH, 30000);
  if (duration == 0) return 400;   // nothing in range
  return duration * 0.034 / 2;
}

void openLid() {
  myServo.write(90);
  lidOpen = true;
  Serial.println("OPEN");
}

void closeLid() {
  myServo.write(0);
  lidOpen = false;
  Serial.println("CLOSE");
}

void setFull(bool full) {
  binFull = full;
  digitalWrite(ledPin, full ? HIGH : LOW);
  digitalWrite(LED_BUILTIN, full ? HIGH : LOW);
  if (full) {
    tone(buzzerPin, 2000, 300);  // one short beep
  }
}

// read commands from the server
void checkSerial() {
  while (Serial.available()) {
    char c = Serial.read();
    if (c == '\n') {
      incoming.trim();
      if (incoming == "FULL") setFull(true);
      if (incoming == "OK") setFull(false);
      incoming = "";
    } else {
      incoming += c;
      if (incoming.length() > 20) incoming = "";  // junk on the line
    }
  }
}

void setup() {
  pinMode(trigPin, OUTPUT);
  pinMode(echoPin, INPUT);
  pinMode(fillTrig, OUTPUT);
  pinMode(fillEcho, INPUT);
  pinMode(ledPin, OUTPUT);
  pinMode(buzzerPin, OUTPUT);
  pinMode(LED_BUILTIN, OUTPUT);

  myServo.attach(servoPin);
  myServo.write(0);

  Serial.begin(9600);
  Serial.print("BIN,");
  Serial.println(BIN_ID);
}

void loop() {
  checkSerial();

  int hand = readDistance(trigPin, echoPin);
  unsigned long now = millis();

  // lid logic, the timer starts again every time the hand is seen
  if (hand <= THRESHOLD) {
    lastHandSeen = now;
    if (!lidOpen) {
      if (binFull && LOCK_WHEN_FULL) {
        tone(buzzerPin, 1000, 150);  // beep to say the bin is full
      } else {
        openLid();
      }
    }
  }
  if (lidOpen && now - lastHandSeen > HOLD_TIME) {
    closeLid();
  }

  // send the hand distance to the server
  if (now - lastHandPrint > 500) {
    Serial.print("HAND,");
    Serial.println(hand);
    lastHandPrint = now;
  }

  // fill reading only when the lid is closed, when it is open the sensor points at the wall
  if (!lidOpen && now - lastFillPrint > 5000) {
    delay(30);  // let the echo from the front sensor die out first
    int fill = readDistance(fillTrig, fillEcho);
    if (fill < 400) {          // 400 means the second sensor is not connected
      Serial.print("FILL,");
      Serial.println(fill);
    }
    lastFillPrint = now;
  }

  if (now - lastIdPrint > 5000) {
    Serial.print("BIN,");
    Serial.println(BIN_ID);
    lastIdPrint = now;
  }

  delay(100);
}
