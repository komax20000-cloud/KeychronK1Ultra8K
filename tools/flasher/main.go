package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const usage = `Keychron Ultra firmware flasher (Realtek SC-FWU over HID)

  Realtek_HID_flasher.exe <image.bin>          flash (also: drag a .bin onto the exe)
  Realtek_HID_flasher.exe flash <image.bin>
  Realtek_HID_flasher.exe handshake            read-only: identify the keyboard
  ... --path=<HID path>                        only if two Keychron boards are attached
`

const (
	keyUp rune = 0x110000 + iota
	keyDown
	keyEnter
	keyEscape
)

func logf(f string, a ...any) { fmt.Printf(f+"\n", a...) }

func firmwareImages(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("cannot list firmware images in %s: %w", dir, err)
	}
	var images []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".bin") {
			images = append(images, filepath.Join(dir, entry.Name()))
		}
	}
	sort.Slice(images, func(i, j int) bool {
		return strings.ToLower(images[i]) < strings.ToLower(images[j])
	})
	return images, nil
}

func moveSelection(index, count int, key rune) int {
	switch key {
	case keyUp:
		if index > 0 {
			return index - 1
		}
	case keyDown:
		if index < count-1 {
			return index + 1
		}
	}
	return index
}

func chooseFirmware(images []string) (string, error) {
	if len(images) == 0 {
		return "", fmt.Errorf("no .bin firmware images found beside the flasher")
	}
	fmt.Println("Firmware images beside the flasher:")
	for _, image := range images {
		fmt.Printf("  %s\n", filepath.Base(image))
	}
	index := 0
	fmt.Printf("Selected: %s (Up/Down, Enter to select, Esc to cancel)\n", filepath.Base(images[index]))
	for {
		key, err := readMenuKey()
		if err != nil {
			return "", fmt.Errorf("cannot read keyboard input: %w", err)
		}
		switch key {
		case keyEnter:
			return images[index], nil
		case keyEscape:
			return "", nil
		case keyUp, keyDown:
			index = moveSelection(index, len(images), key)
			fmt.Printf("Selected: %s\n", filepath.Base(images[index]))
		}
	}
}

func confirmFlash(image string) (bool, error) {
	fmt.Printf("Really flash %s? [Y/N] ", filepath.Base(image))
	for {
		key, err := readMenuKey()
		if err != nil {
			return false, fmt.Errorf("cannot read confirmation: %w", err)
		}
		switch key {
		case 'y', 'Y':
			fmt.Println("Y")
			return true, nil
		case 'n', 'N', keyEscape:
			fmt.Println("N")
			return false, nil
		}
	}
}

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
		exe, err := os.Executable()
		if err != nil {
			logf("!! cannot locate the flasher executable: %v", err)
			return 2
		}
		images, err := firmwareImages(filepath.Dir(exe))
		if err != nil {
			logf("!! %v", err)
			return 2
		}
		image, err := chooseFirmware(images)
		if err != nil {
			logf("!! %v", err)
			return 2
		}
		if image == "" {
			logf("Flash cancelled.")
			return 0
		}
		rest = []string{"flash", image}
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
		confirmed, err := confirmFlash(rest[1])
		if err != nil {
			logf("!! %v", err)
			return 2
		}
		if !confirmed {
			logf("Flash cancelled.")
			return 0
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
