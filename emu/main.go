package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/tarm/serial"
)

func main() {

	fmt.Print("Enter the serial port (e.g. COM8 or /dev/ttyUSB0): ")
	inputReader := bufio.NewReader(os.Stdin)
	port, _ := inputReader.ReadString('\n')
	port = strings.TrimSpace(port)

	baud := 115200

	config := &serial.Config{
		Name:        port,
		Baud:        baud,
		ReadTimeout: time.Millisecond * 50,
	}

	s, err := serial.OpenPort(config)
	if err != nil {
		log.Fatal("Failed to open port:", err)
	}
	defer s.Close()

	fmt.Println("Pico USB CDC emulator running on port:", port)

	var buffer strings.Builder

	for {

		buf := make([]byte, 1)
		n, err := s.Read(buf)
		if err != nil || n == 0 {
			continue
		}

		c := string(buf[0])
		if c == "\n" || c == "\r" {
			parancs := buffer.String()
			buffer.Reset()

			switch parancs {
			case "HIGH":
				s.Write([]byte("OK HIGH\n"))
				fmt.Println("GPIO1 = HIGH")
			case "LOW":
				s.Write([]byte("OK LOW\n"))
				fmt.Println("GPIO1 = LOW")
			case "":

			default:
				s.Write([]byte("ERR UNKNOWN\n"))
				fmt.Println("ERROR UNKNOWN COMMAND:", parancs)
			}
		} else {

			buffer.WriteString(c)
		}
	}
}
