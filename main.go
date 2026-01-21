package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
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

func portValaszt() {

	data, err := os.ReadFile("serial.txt")
	if err == nil {

		text := strings.TrimSpace(strings.ToLower(string(data)))
		if text == "no" {
			noserial = true
			fmt.Println("Serial használat letiltva (serial.txt: no)")
			return
		} else if text != "" {
			KivalasztottPort = text
			fmt.Println("Serial port betöltve serial.txt-ből:", KivalasztottPort)
			return
		}
	}

	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		log.Fatal(err)
	}

	if len(ports) == 0 {
		fmt.Println("Nincs elérhető soros port!")
		noserial = true
		return
	}

	fmt.Println("Elérhető soros portok:")
	for i, port := range ports {
		fmt.Printf("[%d] %s", i, port.Name)
		if port.IsUSB {
			fmt.Printf(" (USB VID: %s PID: %s)", port.VID, port.PID)
		}
		fmt.Println()
	}

	fmt.Print("Válasszon port számot (vagy írjon 'no' a letiltáshoz): ")
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
		log.Fatal("Érvénytelen választás")
	}

	KivalasztottPort = ports[index].Name
	fmt.Println("Kiválasztott port:", KivalasztottPort)

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
		addLog("Hangfájl nem található: ring.mp3")
		return
	}

	streamer, format, err := mp3.Decode(f)
	if err != nil {
		bellRinging = false
		addLog("MP3 dekódolási hiba")
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
			addLog("Hétvégi csengés tiltva")
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
		addLog("Hiba: nincs kiválasztott port")
		return fmt.Errorf("nincs kiválasztott port")
	}

	if jel != "HIGH" && jel != "LOW" {
		addLog(fmt.Sprintf("Hiba: érvénytelen jel: %s", jel))
		return fmt.Errorf("érvénytelen jel: %s", jel)
	}

	mode := &serial.Mode{
		BaudRate: sebesseg,
	}

	port, err := serial.Open(KivalasztottPort, mode)
	if err != nil {
		addLog(fmt.Sprintf("Hiba port megnyitásakor: %v", err))
		return err
	}
	defer port.Close()

	_, err = port.Write([]byte(jel + "\n"))
	if err != nil {
		addLog(fmt.Sprintf("Hiba jel küldésekor: %v", err))
		return err
	}
	addLog(fmt.Sprintf("Jel elküldve: %s", jel))

	time.Sleep(100 * time.Millisecond)

	buf := make([]byte, 100)
	n, err := port.Read(buf)
	if err != nil {
		addLog(fmt.Sprintf("Hiba a válasz olvasásakor: %v", err))
		return err
	}

	valasz := strings.TrimSpace(string(buf[:n]))
	addLog(fmt.Sprintf("Pico válasza: %s", valasz))

	return nil
}

func main() {
	portValaszt()
	loadTimesFromFile(currentTimeFile)

	go scheduler()

	mainMenu := tview.NewList().
		AddItem("1. Időzítések", "", '1', func() {
			pages.SwitchToPage("times")
			app.SetFocus(pages)
		}).
		AddItem("2. Be/Ki kapcsolás", "", '2', func() {
			enabled = !enabled
			addLog(fmt.Sprintf("Funkció BE/KI -> %v", enabled))
		}).
		AddItem("3. Impulzus/Tűzjelző mód", "", '3', func() {
			pulseMode = !pulseMode
			addLog(fmt.Sprintf("Impulzus mód -> %v", pulseMode))
			if pulseMode {
				startPulse()
			}
		}).
		AddItem("4. Dev konzol - CSAK KEZELŐNEK", "", '4', func() {
			pages.SwitchToPage("dev")
			app.SetFocus(pages)
		}).
		AddItem("6. Időzítés választás", "", '6', func() {
			pages.SwitchToPage("filemenu")
			app.SetFocus(pages)
		}).
		AddItem("7. Hétvégén csengessen", "", '7', func() {
			enableWeekend = !enableWeekend
			addLog(fmt.Sprintf("Hétvége -> %v", enableWeekend))
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
					"[yellow]Idő:[white] %s  [green]Engedélyezve:[white]%v (Hétvége:%v)  [blue]Impulzus:[white]%v  [red]Állapot:[white]%s",
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
	input := tview.NewInputField().SetLabel("Idő HH:MM:SS): ")
	timesInfo := tview.NewTextView().SetDynamicColors(true)

	updateTimesMenu = func() {
		if len(weekdayTimes) == 0 {
			timesInfo.SetText("Nincsenek időzítések")
		} else {
			timesInfo.SetText("Időzítések:\n" + strings.Join(weekdayTimes, ", "))
		}
	}

	updateTimesMenu()

	input.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			txt := input.GetText()
			_, err := time.Parse("15:04:05", txt)
			if err != nil {
				addLog("Hibás időformátum: " + txt)
				return
			}

			weekdayTimes = append(weekdayTimes, txt)
			addLog("Idő hozzáadva: " + txt)
			saveTimesToFile()
			updateTimesMenu()
			input.SetText("")
		}
	})

	back := tview.NewButton("Vissza/ESC").SetSelectedFunc(func() {
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
			"DEV MÓD\n" +
				"H=HIGH  L=LOW  T=TRIGGER  C=CLEAR  B=BACK\n\n" +
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
		case 't', 'T':
			addLog("MANUÁLIS IDŐ TRIGGER")
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
	sleepWithDraw(3 * time.Second)

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
				go triggerPulseOnce()
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
		addLog("Csak .txt fájlokat lehet betölteni: " + filename)
		return
	}

	currentTimeFile = filename
	logLines = nil

	file, err := os.Open(filename)
	if err != nil {
		addLog(fmt.Sprintf("%s nem található, új fájl jön létre", filename))
		newFile, err := os.Create(filename)
		if err != nil {
			addLog("Nem sikerült létrehozni a fájlt: " + err.Error())
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
		addLog("Hiba a " + filename + " olvasása közben: " + err.Error())
	} else {
		addLog(fmt.Sprintf("%d időzítés betöltve a %s fájlból", len(weekdayTimes), filename))
	}

	if updateTimesMenu != nil {
		updateTimesMenu()
	}
}

func saveTimesToFile() {
	if currentTimeFile == "" {
		addLog("Nincs kiválasztva fájl a mentéshez")
		return
	}

	file, err := os.Create(currentTimeFile)
	if err != nil {
		addLog("Nem sikerült menteni az időzítéseket: " + err.Error())
		return
	}
	defer file.Close()

	for _, t := range weekdayTimes {
		_, _ = file.WriteString(t + "\n")
	}
	addLog("Időzítések mentve a " + currentTimeFile + " fájlba")
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
				loadTimesFromFile(fname)
				pages.SwitchToPage("times")
				app.SetFocus(pages)
			})
		}
		list.AddItem("Új fájl létrehozása", "Create a new schedule file", 0, func() {
			showNewFilePrompt(updateList)
		})
		list.AddItem("Vissza/ESC", "Return to main menu", 0, func() {
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
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".txt") {
			files = append(files, entry.Name())
		}
	}
	return files
}

func showNewFilePrompt(updateList func()) {
	form := tview.NewForm()
	inputField := tview.NewInputField().SetLabel("Fájl neve").SetFieldWidth(20)
	form.AddFormItem(inputField)

	form.AddButton("Létrehozás", func() {
		name := inputField.GetText()
		if name != "" {
			if !strings.HasSuffix(name, ".txt") {
				name += ".txt"
			}
			f, err := os.Create(name)
			if err == nil {
				f.Close()
				addLog("Új fájl létrehozva: " + name)
				updateList()
			} else {
				addLog("Nem sikerült létrehozni a fájlt: " + err.Error())
			}
		}
		app.SetRoot(pages, true)
	})

	form.AddButton("Mégse", func() {
		app.SetRoot(pages, true)
	})

	form.SetBorder(true).SetTitle("Új fájl létrehozása").SetTitleAlign(tview.AlignCenter)
	app.SetRoot(form, true)
}

func canRunNow() bool {
	weekday := time.Now().Weekday()
	isWeekend := weekday == time.Saturday || weekday == time.Sunday
	return !isWeekend || enableWeekend
}
