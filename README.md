# **Bellringer – Terminal‑based Bell Control System**

Bellringer is a bell or relay control system that works with a Raspberry Pi Pico and/or MP3 playback.  
The user interface runs in the terminal, is built on the tview library, and is fully operable from the keyboard.

---

## **Main Features**

### **Schedules**
- Add time entries (HH:MM:SS)
- Save and load schedules from .txt files
- Manage multiple schedule files
- Create new schedule files

### **GPIO Control (Raspberry Pi Pico)**
- Send HIGH and LOW signals
- Automatic USB port detection
- Log Pico responses

### **Impulse Mode**
- Continuous alternating HIGH and LOW signals
- Manual single impulse (trigger)

### **Time Handling**
- Fetch NTP time at startup
- Internal clock updated every second
- Manual time adjustment from menu

### **Weekend Operation**
- Enable or disable weekend ringing
- Scheduler runs only on allowed days

### **Developer Console**
- Manual HIGH, LOW, and TRIGGER commands
- View and clear logs

---

## **Main Menu Items**

1. Manage schedules  
2. Turn system ON or OFF  
3. Impulse mode  
4. Developer console  
5. Set time  
6. Select schedule file  
7. Enable weekend operation  

---

## **File Handling**

Schedules are stored in simple text files.  
Each file contains a list of schedule times, one per line.

Example:

```
07:45:00
08:00:00
12:30:00
13:15:00
```

Files appear automatically in the menu and can be selected.

---

## **Communication with the Pico**

The program automatically searches for the Pico’s USB serial port.  
Communication uses simple text commands:

- HIGH  
- LOW  

The Pico’s response is logged.

---

## **Scheduler Operation**

The background scheduler checks every second:
- whether the system is enabled  
- whether the current time matches a schedule  
- whether weekend operation is allowed  

If all conditions are met, a single impulse is executed.

---

## **Building and Running**

The program is written in Go. Windows and Linux are supported.

Build:

```
go build
```

Run:

```
./bellringer
```

Required libraries:
- github.com/beevik/ntp
- github.com/gdamore/tcell/v2
- github.com/rivo/tview
- go.bug.st/serial

---

## **Execution Flow**

1. At startup, the program attempts to load the serial port from `serial.txt`.  
2. If not set, it lists available ports and asks the user to choose one.  
3. It loads schedules from the selected `.txt` file.  
4. It attempts to fetch NTP time; if it fails, system time is used.  
5. The `clockTicker` starts, updating internal time every second.  
6. The scheduler starts, checking schedules every second.  
7. The user can control the system from the menu:  
   - add schedules  
   - enable impulse mode  
   - send manual HIGH or LOW signals  
   - set time  
   - select schedule file  
8. When a schedule time is reached, the program executes an impulse (HIGH, delay, LOW).  
9. The log updates continuously and can be viewed in the developer console.
