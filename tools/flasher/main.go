package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

const usage = `Keychron Ultra firmware flasher (Realtek SC-FWU over HID)

  Realtek_HID_flasher.exe <image.bin>          flash (also: drag a .bin onto the exe)
  Realtek_HID_flasher.exe flash <image.bin>
  Realtek_HID_flasher.exe handshake            read-only: identify the keyboard
  ... --path=<HID path>                        only if two Keychron boards are attached
`

func logf(f string, a ...any) { fmt.Printf(f+"\n", a...) }

func run(args []string) int {
	var path string
	var rest []string
	for _, a := range args {
		if strings.HasPrefix(a, "--path=") {
			path = strings.TrimPrefix(a, "--path=")
		} else if !strings.HasPrefix(a, "--") {
			rest = append(rest, a)
		}
	}
	if len(rest) == 1 && strings.HasSuffix(strings.ToLower(rest[0]), ".bin") {
		rest = []string{"flash", rest[0]}
	} else if len(rest) == 0 {
		fmt.Print("Firmware .bin path (empty = handshake only): ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		line = strings.Trim(strings.TrimSpace(line), `"`)
		if line == "" {
			rest = []string{"handshake"}
		} else {
			rest = []string{"flash", line}
		}
	}
	if rest[0] != "handshake" && (rest[0] != "flash" || len(rest) < 2) {
		fmt.Print(usage)
		return 2
	}
	var image []byte
	if rest[0] == "flash" {
		var err error
		if image, err = os.ReadFile(rest[1]); err != nil {
			logf("!! cannot read image: %v", err)
			return 2
		}
	}
	p, err := pickDevice(findDFU(), path)
	if err != nil {
		logf("!! %v", err)
		return 2
	}
	t, err := openDevice(p)
	if err != nil {
		logf("!! cannot open the keyboard: %v\n   Close the Keychron Launcher / other HID tools and retry.", err)
		return 2
	}
	defer t.Close()
	d := newDFU(t)
	ok, err := handshake(d, logf)
	if err != nil || !ok {
		if err != nil {
			logf("!! %v", err)
		}
		logf("!! handshake failed - aborting before any write.")
		return 1
	}
	if rest[0] == "handshake" {
		return 0
	}
	if ok, err = flash(d, image, logf); err != nil {
		logf("!! %v", err)
	}
	if !ok {
		return 1
	}
	return 0
}

func main() {
	rc := run(os.Args[1:])
	// Keep the window open when started by double-click or drag-and-drop
	if len(os.Args) <= 2 {
		fmt.Print("Press Enter to close...")
		bufio.NewReader(os.Stdin).ReadString('\n')
	}
	os.Exit(rc)
}
