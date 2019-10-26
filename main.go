package main

import (
	"log"
	"time"
	"fmt"
	"bufio"
	"strconv"
	"os"
	"os/signal"
	"html/template"
	"net/http"
	"io/ioutil"
	"github.com/tarm/serial"
	"github.com/gorilla/websocket"
	"gopkg.in/yaml.v2"
)


type Spun struct {
	Scale int
	Revolutions int64
	LastRemoteRevolutions int64
	TotalRevolutions int64
	Today time.Time
}

var revolutionScale = 950; // scale revolution down to km
var spun = Spun{
	Scale: revolutionScale,
//	Revolutions: 3639,
//	TotalRevolutions: int64(38*revolutionScale),
}

func main() {
	// Load revolution data
	data, err := os.Open("rev.yaml")
	if err == nil {
		b, _ := ioutil.ReadAll(data)
		err = yaml.Unmarshal(b, &spun)
		if err != nil {
			log.Panicf("Could not unmarshal rev.yaml: %s", err)
		}
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	go func() {
		inter := <-signals
		save()
		log.Panicf("intr: %s", inter)
	}()
	webserver(":5432")
	for {
		openAndReadSerialForever()
	}
}

func save() {
	os.Remove("rev.yaml")
	f, err := os.Create("rev.yaml")
	if err != nil {
		log.Panicf("Could not open rev for dumping state: %s", err)
	}
	defer f.Close()
	marshaled, err := yaml.Marshal(spun)
	if err != nil {
		log.Panicf("Could not dump state: %s", err)
	}
	f.Write(marshaled)
}

func openAndReadSerialForever() {
	// Read serial data
	port, err := serial.OpenPort(&serial.Config{Name: "/dev/ttyACM0", Baud: 9600})
	if err != nil {
		log.Panicf("Could not open arduino port")
	}
	defer port.Close()
	log.Print("Connected.")
	scanner := bufio.NewScanner(port)
	for scanner.Scan() {
		txt := scanner.Text()
		b := scanner.Bytes()
		readMessage(txt, b)
	}
}

var templates *template.Template

func loadTemplates() {
	templates = template.Must(template.ParseFiles(
		"tpl/head",
		"tpl/tail",
		"tpl/main"))
}

func webserver(listenAddr string) {
	loadTemplates()
	server := http.Server {
		Addr: listenAddr,
	}
	http.HandleFunc("/", webMain)
	http.HandleFunc("/ws", webSocket)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	go server.ListenAndServe()
}

var sendWebSocket = make(chan string, 10000)

func webSocket(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Panicf("Couldn't open a websocket")
	}
	sendWebSocket = make(chan string, 1000)
	for {
		conn.WriteMessage(websocket.TextMessage, []byte(<-sendWebSocket))
	}
}

type MainView struct {
	Revolutions int64
	TotalRevolutions int64
	Scale int
}

func webMain(w http.ResponseWriter, r *http.Request) {
	loadTemplates()
	err := templates.ExecuteTemplate(w, "head", MainView{spun.Revolutions, spun.TotalRevolutions, spun.Scale})
	if err != nil {
		log.Printf("Could not render template: %s", err)
	}
	templates.ExecuteTemplate(w, "main", nil)
	templates.ExecuteTemplate(w, "tail", nil)
}

func readMessage(txt string, b []byte) {
	value := txt[1:]
	i, err := strconv.ParseInt(value, 10, 64)
	if txt[0] == 'R' {
		if err != nil {
			log.Print("Non int revolution received: %s", value)
			return
		}
		if time.Now().Day() != spun.Today.Day() {
			log.Print("Resetting daily revolutions.")
			spun.Today = time.Now()
			spun.Revolutions = 0
			sendWebSocket <- "RST"
		} else if spun.Revolutions == 0 {
			spun.Revolutions = int64(i)
			spun.TotalRevolutions += spun.Revolutions
		}
		totalIncrease := int64(0)
		if i < spun.LastRemoteRevolutions {
			// Remote device spins have been reset, so comparison is useless
			totalIncrease = i
		} else {
			totalIncrease = i-spun.LastRemoteRevolutions
		}
		spun.LastRemoteRevolutions = i
		spun.Revolutions += totalIncrease
		spun.TotalRevolutions += totalIncrease
		log.Printf("Revolution recv: %d today: %d", i, spun.Revolutions)
		sendWebSocket <- fmt.Sprintf("R%d", spun.Revolutions)
		if spun.Revolutions % 1000 == 0 {
			save()
		}
	} else if txt[0] == '!' {
		log.Printf("Leads off.")
	} else if txt[0] == 'H' {
		log.Printf("Heart monitor update.", txt[1:])
	} else {
		log.Printf("Signal received: %+v", b)
	}
}
