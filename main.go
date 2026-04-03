package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
	"golang.org/x/term"
)

var (
	app             = tview.NewApplication()
	pages           = tview.NewPages()
	enabled         = true
	weekdayTimes    = []string{}
	pulseMode       = false
	enableWeekend   = false
	statusText      = "LOW"
	logLines        = []string{}
	pulseRunning    = false
	currentTimeFile = "idobeall1.txt"
	updateTimesMenu func()

	KivalasztottPort string
	port             serial.Port
	reader           *bufio.Reader
	sebesseg         = 115200
	noserial         bool

	bellRinging bool
	ctrl        *beep.Ctrl
	volume      *effects.Volume
)

var htmlPage = `
<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<title>Bell ringer</title>

<style>
body {
background: #0a0a0f;
color: #d0b3ff;
font-family: Arial;
text-align: center;
}

h1 {
color: #bb86fc;
}

.container {
display: flex;
flex-wrap: wrap;
justify-content: center;
}

.card {
background: #12121a;
border: 1px solid #7c3aed;
border-radius: 10px;
padding: 20px;
margin: 10px;
width: 280px;
box-shadow: 0 0 15px #7c3aed44;
}

button {
background: #1a1a2e;
color: #bb86fc;
border: 1px solid #7c3aed;
padding: 10px;
margin: 5px;
cursor: pointer;
width: 100%;
}

button:hover {
background: #7c3aed;
color: white;
}

input {
background: black;
color: #bb86fc;
border: 1px solid #7c3aed;
padding: 8px;
width: 80%;
margin-bottom: 5px;
}

.time-item {
display: flex;
justify-content: space-between;
margin: 5px 0;
}

.small-btn {
width: auto;
padding: 5px 10px;
font-size: 12px;
}
</style>

</head>
<body>

<h1>Bell Ringer CONTROL PANEL</h1>

<div class="container">

<div class="card">
<h3>Controls</h3>
<button onclick="send('/api/high')">HIGH</button>
<button onclick="send('/api/low')">LOW</button>
<button onclick="send('/api/toggle')">ENABLE</button>
<button onclick="send('/api/pulse')">PULSE</button>
<button onclick="send('/api/weekend')">WEEKEND</button>
<button onclick="send('/api/ring')">RING (short)</button>
<button onclick="send('/api/ring-long')">RING (long)</button>
</div>

<div class="card">
<h3>Status</h3>
<div id="status"></div>
</div>

<div class="card">
<h3>Schedules</h3>

<input id="timeInput" placeholder="HH:MM:SS">
<button onclick="addTime()">Add</button>

<div id="times"></div>
</div>

<div class="card">
<h3>Time Files</h3>

<input id="fileInput" placeholder="newfile.txt">
<button onclick="createFile()">Create File</button>

<div id="files"></div>
</div>

</div>

<script>

function send(url){
fetch(url);
}

async function update(){
let res = await fetch('/api/status');
let data = await res.json();

document.getElementById('status').innerHTML =
"TIME: " + data.time + "<br>" +
"STATUS: " + data.status + "<br>" +
"ENABLED: " + data.enabled + "<br>" +
"PULSE: " + data.pulseMode + "<br>" +
"WEEKEND: " + data.weekend + "<br>" +
"FILE: " + data.currentFile;
}

async function loadTimes(){
let res = await fetch('/api/times');
let data = await res.json();

let html = "";
data.forEach(function(t){
html += "<div class='time-item'>" +
"<span>" + t + "</span>" +
"<button class='small-btn' onclick=\"deleteTime('" + t + "')\">X</button>" +
"</div>";
	});

	document.getElementById('times').innerHTML = html;
	}

	async function addTime(){
	let t = document.getElementById('timeInput').value;

	await fetch('/api/add-time?t=' + t);

	document.getElementById('timeInput').value = "";
	await loadTimes();
	}

	async function deleteTime(t){
	await fetch('/api/delete-time?t=' + t);
	await loadTimes();
	}

	async function loadFiles(){
	let res = await fetch('/api/files');
	let data = await res.json();

	let html = "";
	data.forEach(function(f){
	html += "<div class='time-item'>" +
	"<span>" + f + "</span>" +
	"<button class='small-btn' onclick=\"loadFile('" + f + "')\">LOAD</button>" +
	"</div>";
});

document.getElementById('files').innerHTML = html;
}

async function loadFile(name){
await fetch('/api/load-file?name=' + name);
await loadTimes();
}

async function createFile(){
let name = document.getElementById('fileInput').value;

await fetch('/api/new-file?name=' + name);

document.getElementById('fileInput').value = "";
await loadFiles();
}

setInterval(() => {
update();
loadTimes();
loadFiles();
}, 2000);

update();
loadTimes();
loadFiles();

</script>

</body>
</html>
`
var (
	webEnabled  bool
	webPort     string
	webUsername string
	webPassword string
)

func ensureWebConfig() {
	configFile := "webconfig.txt"

	if _, err := os.Stat(configFile); err == nil {
		loadWebConfig()
		return
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Engedélyezze a web szervert? (true/false) [true]: ")
	enabledInput, _ := reader.ReadString('\n')
	enabledInput = strings.TrimSpace(enabledInput)
	if enabledInput == "" {
		enabledInput = "true"
	}
	webEnabled = enabledInput == "true"

	fmt.Print("Port a web szerverhez [80]: ")
	portInput, _ := reader.ReadString('\n')
	portInput = strings.TrimSpace(portInput)
	if portInput == "" {
		portInput = "80"
	}
	webPort = portInput

	fmt.Print("Felhasználónév [admin]: ")
	userInput, _ := reader.ReadString('\n')
	userInput = strings.TrimSpace(userInput)
	if userInput == "" {
		userInput = "admin"
	}
	webUsername = userInput

	fmt.Print("Jelszó [1234]: ")
	bytePassword, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		fmt.Println("Hiba a jelszó beolvasásakor, alapértelmezett jelszó lesz: 1234")
		webPassword = "1234"
	} else {
		webPassword = strings.TrimSpace(string(bytePassword))
		if webPassword == "" {
			webPassword = "1234"
		}
	}

	content := fmt.Sprintf(
		"enabled=%t\nport=%s\nusername=%s\npassword=%s\n",
		webEnabled, webPort, webUsername, webPassword,
	)

	err = os.WriteFile(configFile, []byte(content), 0644)
	if err != nil {
		fmt.Println("Hiba a webconfig.txt létrehozásakor:", err)
		return
	}
}

func loadWebConfig() {
	configFile := "webconfig.txt"

	data, err := os.ReadFile(configFile)
	if err != nil {
		fmt.Println("Hiba a webconfig.txt olvasásakor:", err)
		webEnabled = false
		return
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
			case "enabled":
				webEnabled = (value == "true")
			case "port":
				webPort = value
			case "username":
				webUsername = value
			case "password":
				webPassword = value
		}
	}

}

func startWebServer() {
	if webPort == "" {
		webPort = "80"
	}

	mux := http.NewServeMux()

	authHandler := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			user, pass, ok := r.BasicAuth()
			if !ok || user != webUsername || pass != webPassword {
				w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
				w.WriteHeader(401)
				w.Write([]byte("Unauthorized.\n"))
				return
			}
			next(w, r)
		}
	}

	mux.HandleFunc("/", authHandler(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, htmlPage)
	}))

	mux.HandleFunc("/api/status", authHandler(func(w http.ResponseWriter, r *http.Request) {
		data := map[string]interface{}{
			"enabled":     enabled,
			"pulseMode":   pulseMode,
			"status":      statusText,
			"time":        time.Now().Format("15:04:05"),
						  "weekend":     enableWeekend,
						  "currentFile": currentTimeFile,
		}
		json.NewEncoder(w).Encode(data)
	}))

	mux.HandleFunc("/api/high", authHandler(func(w http.ResponseWriter, r *http.Request) {
		go SetHigh()
		w.Write([]byte("ok"))
	}))

	mux.HandleFunc("/api/ring", authHandler(func(w http.ResponseWriter, r *http.Request) {
		go triggerPulseOnceal()
		w.Write([]byte("ok"))
	}))

	mux.HandleFunc("/api/ring-long", authHandler(func(w http.ResponseWriter, r *http.Request) {
		go triggerPulseOnce()
		w.Write([]byte("ok"))
	}))

	mux.HandleFunc("/api/low", authHandler(func(w http.ResponseWriter, r *http.Request) {
		go SetLow()
		w.Write([]byte("ok"))
	}))

	mux.HandleFunc("/api/toggle", authHandler(func(w http.ResponseWriter, r *http.Request) {
		enabled = !enabled
		addLog(fmt.Sprintf("WEB: enabled -> %v", enabled))
		app.QueueUpdateDraw(func() {})
		w.Write([]byte("ok"))
	}))

	mux.HandleFunc("/api/pulse", authHandler(func(w http.ResponseWriter, r *http.Request) {
		pulseMode = !pulseMode
		if pulseMode {
			startPulse()
		}
		addLog(fmt.Sprintf("WEB: pulse -> %v", pulseMode))
		app.QueueUpdateDraw(func() {})
		w.Write([]byte("ok"))
	}))

	mux.HandleFunc("/api/times", authHandler(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(weekdayTimes)
	}))

	mux.HandleFunc("/api/add-time", authHandler(func(w http.ResponseWriter, r *http.Request) {
		t := r.URL.Query().Get("t")
		if _, err := time.Parse("15:04:05", t); err != nil {
			http.Error(w, "invalid time", 400)
			return
		}
		for _, v := range weekdayTimes {
			if v == t {
				w.Write([]byte("exists"))
				return
			}
		}
		weekdayTimes = append(weekdayTimes, t)
		saveTimesToFile()
		addLog("WEB: idő hozzáadva " + t)
		app.QueueUpdateDraw(func() {
			if updateTimesMenu != nil {
				updateTimesMenu()
			}
		})
		w.Write([]byte("ok"))
	}))

	mux.HandleFunc("/api/files", authHandler(func(w http.ResponseWriter, r *http.Request) {
		files := listAllFiles()
		json.NewEncoder(w).Encode(files)
	}))

	mux.HandleFunc("/api/new-file", authHandler(func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if name == "" {
			http.Error(w, "no name", 400)
			return
		}
		if !strings.HasSuffix(name, ".txt") {
			name += ".txt"
		}
		f, err := os.Create(name)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		f.Close()
		addLog("WEB: új fájl " + name)
		w.Write([]byte("ok"))
	}))

	mux.HandleFunc("/api/weekend", authHandler(func(w http.ResponseWriter, r *http.Request) {
		enableWeekend = !enableWeekend
		addLog(fmt.Sprintf("WEB: hétvége -> %v", enableWeekend))
		app.QueueUpdateDraw(func() {})
		w.Write([]byte("ok"))
	}))

	mux.HandleFunc("/api/load-file", authHandler(func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		loadTimesFromFile(name)
		addLog("WEB: fájl betöltve " + name)
		app.QueueUpdateDraw(func() {
			if updateTimesMenu != nil {
				updateTimesMenu()
			}
		})
		w.Write([]byte("ok"))
	}))

	mux.HandleFunc("/api/delete-time", authHandler(func(w http.ResponseWriter, r *http.Request) {
		t := r.URL.Query().Get("t")
		var newList []string
		for _, v := range weekdayTimes {
			if v != t {
				newList = append(newList, v)
			}
		}
		weekdayTimes = newList
		saveTimesToFile()
		addLog("WEB: idő törölve " + t)
		app.QueueUpdateDraw(func() {
			if updateTimesMenu != nil {
				updateTimesMenu()
			}
		})
		w.Write([]byte("ok"))
	}))

	log.Fatal(http.ListenAndServe("0.0.0.0:"+webPort, mux))
}


func portValaszt() {

	data, err := os.ReadFile("serial.txt")
	if err == nil {

		text := strings.TrimSpace(strings.ToLower(string(data)))
		if text == "no" {
			noserial = true
			fmt.Println("Serial usage disabled (serial.txt: no)")
			return
		} else if text != "" {
			KivalasztottPort = text
			fmt.Println("Serial port loaded from serial.txt:", KivalasztottPort)
			return
		}
	}

	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		log.Fatal(err)
	}

	if len(ports) == 0 {
		fmt.Println("No serial ports available!")
		noserial = true
		return
	}

	fmt.Println("Available serial ports:")
	for i, port := range ports {
		fmt.Printf("[%d] %s", i, port.Name)
		if port.IsUSB {
			fmt.Printf(" (USB VID: %s PID: %s)", port.VID, port.PID)
		}
		fmt.Println()
	}

	fmt.Print("Choose a port number (or type 'no' to disable): ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	valasz := scanner.Text()
	valasz = strings.TrimSpace(strings.ToLower(valasz))

	if valasz == "no" {
		noserial = true
		KivalasztottPort = ""

		os.WriteFile("serial.txt", []byte("no"), 0644)
		return
	}

	index := -1
	fmt.Sscanf(valasz, "%d", &index)
	if index < 0 || index >= len(ports) {
		log.Fatal("Invalid selection")
	}

	KivalasztottPort = ports[index].Name
	fmt.Println("KivĂˇlasztott port:", KivalasztottPort)

	os.WriteFile("serial.txt", []byte(KivalasztottPort), 0644)
}

func stopRing() {
	if !bellRinging {
		return
	}
	bellRinging = false

	fade := 250 * time.Millisecond
	steps := 25
	stepDur := fade / time.Duration(steps)

	go func() {
		for i := 0; i < steps; i++ {
			speaker.Lock()
			volume.Volume -= 1.0 / float64(steps)
			speaker.Unlock()
			time.Sleep(stepDur)
		}

		speaker.Lock()
		ctrl.Paused = true
		speaker.Unlock()
	}()
}

func playMP3() {
	if bellRinging {
		return
	}
	bellRinging = true

	f, err := os.Open("ring.mp3")
	if err != nil {
		bellRinging = false
		addLog("Sound file not found: ring.mp3")
		return
	}

	streamer, format, err := mp3.Decode(f)
	if err != nil {
		bellRinging = false
		addLog("MP3 decoding error")
		_ = f.Close()
		return
	}

	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))

	ctrl = &beep.Ctrl{Streamer: streamer, Paused: false}
	volume = &effects.Volume{
		Streamer: ctrl,
		Base:     2,
		Volume:   0,
		Silent:   false,
	}

	speaker.Play(volume)
}

func addLog(msg string) {
	line := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg)
	logLines = append(logLines, line)
	if len(logLines) > 100 {
		logLines = logLines[len(logLines)-100:]
	}
}

func AutoDetect() error {
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return err
	}

	for _, p := range ports {
		if p.IsUSB && (strings.Contains(strings.ToLower(p.Product), "pico") ||
			strings.Contains(strings.ToLower(p.SerialNumber), "pico")) {

			mode := &serial.Mode{BaudRate: 115200}
			port, err = serial.Open(p.Name, mode)
			if err != nil {
				return err
			}

			reader = bufio.NewReader(port)
			return nil
		}
	}
	return errors.New("no Raspberry Pi Pico detected")
}

func SetHigh() {
	if !enabled {
		return
	}
	if !pulseMode {
		if !canRunNow() {
			addLog("Weekend ringing disabled")
			return
		}
	}
	statusText = "HIGH"
	addLog("GPIO -> HIGH")
	jelKuldes("HIGH")
	go playMP3()

	app.QueueUpdateDraw(func() {})
}

func SetLow() {
	statusText = "LOW"
	addLog("GPIO -> LOW")
	jelKuldes("LOW")
	stopRing()
	app.QueueUpdateDraw(func() {})
}

func jelKuldes(jel string) error {
	if noserial {
		return nil
	}
	if KivalasztottPort == "" {
		addLog("Error: no port selected")
		return fmt.Errorf("nincs kivĂˇlasztott port")
	}

	if jel != "HIGH" && jel != "LOW" {
		addLog(fmt.Sprintf("Error: invalid signal: %s", jel))
		return fmt.Errorf("Ă©rvĂ©nytelen jel: %s", jel)
	}

	mode := &serial.Mode{
		BaudRate: sebesseg,
	}

	port, err := serial.Open(KivalasztottPort, mode)
	if err != nil {
		addLog(fmt.Sprintf("Error opening port: %v", err))
		return err
	}
	defer port.Close()

	_, err = port.Write([]byte(jel + "\n"))
	if err != nil {
		addLog(fmt.Sprintf("Error sending signal: %v", err))
		return err
	}
	addLog(fmt.Sprintf("Signal sent: %s", jel))

	time.Sleep(100 * time.Millisecond)

	buf := make([]byte, 100)
	n, err := port.Read(buf)
	if err != nil {
		addLog(fmt.Sprintf("Error reading response: %v", err))
		return err
	}

	valasz := strings.TrimSpace(string(buf[:n]))
	addLog(fmt.Sprintf("Pico response: %s", valasz))

	return nil
}

func main() {
	portValaszt()
	loadTimesFromFile(currentTimeFile)

	ensureWebConfig()

	if webEnabled {
		go startWebServer()
	}
	go scheduler()

	mainMenu := tview.NewList().
		AddItem("1. Schedules", "", '1', func() {
			pages.SwitchToPage("times")
			app.SetFocus(pages)
		}).
		AddItem("2. ON/OFF", "", '2', func() {
			enabled = !enabled
			addLog(fmt.Sprintf("FunkciĂł BE/KI -> %v", enabled))
		}).
		AddItem("3. Impulse/Fire alarm mode", "", '3', func() {
			pulseMode = !pulseMode
			addLog(fmt.Sprintf("Impulse -> %v", pulseMode))
			if pulseMode {
				startPulse()
			}
		}).
		AddItem("4. Dev console", "", '4', func() {
			pages.SwitchToPage("dev")
			app.SetFocus(pages)
		}).
		AddItem("6. Schedule switcher", "", '6', func() {
			pages.SwitchToPage("filemenu")
			app.SetFocus(pages)
		}).
		AddItem("7. Ring on weekend", "", '7', func() {
			enableWeekend = !enableWeekend
			addLog(fmt.Sprintf("HĂ©tvĂ©ge -> %v", enableWeekend))
		})

	statusBar := tview.NewTextView().SetDynamicColors(true)

	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(statusBar, 1, 1, false).
		AddItem(mainMenu, 0, 1, true)

	pages.AddPage("main", layout, true, true)
	pages.AddPage("times", timesMenu(), true, false)
	pages.AddPage("dev", devConsole(), true, false)
	pages.AddPage("filemenu", fileSelectionMenu(), true, false)

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			app.QueueUpdateDraw(func() {
				statusBar.SetText(fmt.Sprintf(
					"[yellow]Time:[white] %s  [green]Enabled:[white]%v (Weekend:%v)  [blue]Pulse:[white]%v  [red]State:[white]%s",
					now.Format("15:04:05"),
					enabled,
					enableWeekend,
					pulseMode,
					statusText,
				))
			})
		}
	}()

	if err := app.SetRoot(pages, true).Run(); err != nil {
		panic(err)
	}
}

func timesMenu() tview.Primitive {
	input := tview.NewInputField().SetLabel("Time HH:MM:SS): ")
	timesInfo := tview.NewTextView().SetDynamicColors(true)

	updateTimesMenu = func() {
		if len(weekdayTimes) == 0 {
			timesInfo.SetText("No schedules")
		} else {
			timesInfo.SetText("Schedules:\n" + strings.Join(weekdayTimes, ", "))
		}
	}

	updateTimesMenu()

	input.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			txt := input.GetText()
			_, err := time.Parse("15:04:05", txt)
			if err != nil {
				addLog("Invalid time format: " + txt)
				return
			}

			weekdayTimes = append(weekdayTimes, txt)
			addLog("Time added: " + txt)
			saveTimesToFile()
			updateTimesMenu()
			input.SetText("")
		}
	})

	back := tview.NewButton("Back/ESC").SetSelectedFunc(func() {
		pages.SwitchToPage("main")
		app.SetFocus(pages)
	})

	input.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			pages.SwitchToPage("main")
			app.SetFocus(pages)
			return nil
		}
		return event
	})

	return tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(timesInfo, 0, 1, false).
		AddItem(input, 1, 1, true).
		AddItem(back, 1, 1, false)
}

func devConsole() tview.Primitive {
	console := tview.NewTextView().SetDynamicColors(true)

	updateLog := func() {
		console.SetText(
			"DEV MĂ“D\n" +
				"H=HIGH  L=LOW  T=TRIGGER  C=CLEAR  B=BACK U=Update times\n\n" +
				strings.Join(logLines, "\n"),
		)
	}

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(console, 0, 1, true)

	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'h', 'H':
			go SetHigh()
		case 'l', 'L':
			go SetLow()
		case 'u', 'U':
			addLog("Updated times")
			loadTimesFromFile(currentTimeFile)
		case 't', 'T':
			addLog("Manual Trigger")
			go triggerPulseOnce()
		case 'c', 'C':
			logLines = nil
		case 'b', 'B':
			pages.SwitchToPage("main")
			app.SetFocus(pages)
			return nil
		}
		updateLog()
		return nil
	})

	updateLog()
	return flex
}

func startPulse() {
	if pulseRunning {
		return
	}
	pulseRunning = true

	go func() {
		for pulseMode {
			SetHigh()
			sleepWithDraw(1 * time.Second)
			if !pulseMode {
				break
			}
			SetLow()
			sleepWithDraw(1 * time.Second)
		}
		SetLow()
		pulseRunning = false
	}()
}

func triggerPulseOnce() {

	SetLow()
	time.Sleep(500 * time.Millisecond)

	SetHigh()
	sleepWithDraw(8 * time.Second)

	SetLow()

}

func triggerPulseOnceal() {

	SetLow()
	time.Sleep(500 * time.Millisecond)

	SetHigh()
	sleepWithDraw(2 * time.Second)

	SetLow()
	time.Sleep(700 * time.Millisecond)

	SetHigh()
	sleepWithDraw(2 * time.Second)

	SetLow()

}

func sleepWithDraw(d time.Duration) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	done := time.After(d)
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			app.QueueUpdateDraw(func() {})
		}
	}
}

func scheduler() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if !enabled {
			continue
		}

		now := time.Now().Format("15:04:05")

		for _, t := range weekdayTimes {
			if t == now {
				addLog("IDŐZÍTÉS AKTIVÁLVA: " + t)

				if t == "07:55:00" {
					go triggerPulseOnceal() // special case
				} else {
					go triggerPulseOnce() // default
				}
			}
		}
	}
}

func loadTimesFromFile(filename string) {
	weekdayTimes = nil

	if filename == "" {
		filename = "idobeall.txt"
	}

	if !strings.HasSuffix(filename, ".txt") {
		addLog("Only .txt files can be loaded: " + filename)
		return
	}

	currentTimeFile = filename
	logLines = nil

	file, err := os.Open(filename)
	if err != nil {
		addLog(fmt.Sprintf("%s not found, creating new file", filename))
		newFile, err := os.Create(filename)
		if err != nil {
			addLog("Failed to create file: " + err.Error())
			return
		}
		newFile.Close()
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(strings.ReplaceAll(scanner.Text(), "\r", ""))
		line = strings.TrimPrefix(line, "\ufeff")
		if line != "" {
			weekdayTimes = append(weekdayTimes, line)
		}
	}

	if err := scanner.Err(); err != nil {
		addLog("Error reading file: " + err.Error())
	} else {
		addLog(fmt.Sprintf("%d schedule(s) loaded from %s fĂˇjlbĂłl", len(weekdayTimes), filename))
	}

	if updateTimesMenu != nil {
		updateTimesMenu()
	}
}

func saveTimesToFile() {
	if currentTimeFile == "" {
		addLog("No choosen file to save")
		return
	}

	file, err := os.Create(currentTimeFile)
	if err != nil {
		addLog("Failed to save file: " + err.Error())
		return
	}
	defer file.Close()

	for _, t := range weekdayTimes {
		_, _ = file.WriteString(t + "\n")
	}
	addLog("Schedules saved to " + currentTimeFile + " fĂˇjlba")
}

func fileSelectionMenu() tview.Primitive {
	list := tview.NewList()

	var updateList func()
	updateList = func() {
		list.Clear()
		files := listAllFiles()
		for _, f := range files {
			fname := f
			list.AddItem(fname, "Load this file", 0, func() {
				currentTimeFile = fname
				loadTimesFromFile(fname)
				pages.SwitchToPage("times")
				app.SetFocus(pages)
			})
		}
		list.AddItem("Create new file", "Create a new schedule file", 0, func() {
			showNewFilePrompt(updateList)
		})
		list.AddItem("Back/ESC", "Return to main menu", 0, func() {
			pages.SwitchToPage("main")
			app.SetFocus(pages)
		})
	}

	updateList()
	return list
}

func listAllFiles() []string {
	var files []string
	entries, err := os.ReadDir(".")
	if err != nil {
		return files
	}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && strings.HasSuffix(name, ".txt") {
			if name == "serial.txt" || name == "webconfig.txt" {
				continue
			}
			files = append(files, name)
		}
	}
	return files
}

func showNewFilePrompt(updateList func()) {
	form := tview.NewForm()
	inputField := tview.NewInputField().SetLabel("File name").SetFieldWidth(20)
	form.AddFormItem(inputField)

	form.AddButton("Create", func() {
		name := inputField.GetText()
		if name != "" {
			if !strings.HasSuffix(name, ".txt") {
				name += ".txt"
			}
			f, err := os.Create(name)
			if err == nil {
				f.Close()
				addLog("File created: " + name)
				updateList()
			} else {
				addLog("Failed to create file: " + err.Error())
			}
		}
		app.SetRoot(pages, true)
	})

	form.AddButton("Cancel", func() {
		app.SetRoot(pages, true)
	})

	form.SetBorder(true).SetTitle("Create new file").SetTitleAlign(tview.AlignCenter)
	app.SetRoot(form, true)
}

func canRunNow() bool {
	weekday := time.Now().Weekday()
	isWeekend := weekday == time.Saturday || weekday == time.Sunday
	return !isWeekend || enableWeekend
}
