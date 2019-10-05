void setup() {
  Serial.begin(9600);
  pinMode(4, INPUT);
  pinMode(7, INPUT);
  pinMode(8, INPUT);
}

int revolutions = 0;
bool lastRead = true;
bool leadsOff = false;
//bool bOffReported = false;

void loop() {
  // Constantly read digital pin 4 and report what state it's in when it changes state
  bool read = digitalRead(2);
  if (read == 0 && read != lastRead) {
    Serial.print("R");
    Serial.println(revolutions);
    revolutions +=1;
    if (revolutions > 9999) {
      revolutions = 0;
    }
  }
  lastRead = read;
  readHrMonitor();
  delay(1);
}

void readHrMonitor() {
  bool aOff = digitalRead(7);
  bool bOff = digitalRead(8);
  if((aOff == 1) || (bOff == 1)) {
    if (leadsOff == false) {
      Serial.print('!');
      Serial.print(aOff);
      Serial.println(bOff);
      leadsOff = true;
    }
  } else {
    // send the value of analog input 0:
    leadsOff = false;
    Serial.print("H");
    Serial.println(analogRead(A0));
  }
}
