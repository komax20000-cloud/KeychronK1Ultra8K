package main

import (
	"errors"
	"fmt"
	"time"
)

// Protocol port of openrgb/flash.py from https://github.com/naaraxi/zmk (MIT, (c) The ZMK
// Contributors), itself a port of the Keychron Launcher's Realtek OTA routine and app/src/dfu/tdfu.c.

const (
	outReportID  = 0xB2
	inReportID   = 0xB1
	reportLen    = 32
	chunk        = 16
	opModelInfo  = 0x60
	opDfuVersion = 0x61
	opStart      = 0x63
	opSendBin    = 0x64
	opVerifyCRC  = 0x65
	opSwitch     = 0x66
	vendorID     = 0x3434
	dfuUsagePage = 0x8C
)

var knownModels = map[string]string{
	"KCZKK18K": "K1 Ultra 8K - ANSI", "KCZKV08K": "V0 Ultra 8K - ANSI", "KCZKV18K": "V1 Ultra 8K - ANSI",
	"KCZKV18I": "V1 Ultra 8K - ISO", "KCZKV18J": "V1 Ultra 8K - JIS",
	"KCZKV28K": "V2 Ultra 8K - ANSI", "KCZKV38K": "V3 Ultra 8K - ANSI",
	"KCZKV58K": "V5 Ultra 8K - ANSI", "KCZKV68K": "V6 Ultra 8K - ANSI",
	"KCZKVA8K": "V10 Ultra 8K - ANSI", "KCZKQ18K": "Q1 Ultra 8K - ANSI",
	"KCZKQ38K": "Q3 Ultra 8K - ANSI", "KCZKQ68K": "Q6 Ultra 8K - ANSI",
	"KCZ270U": "Z2-70 Ultra 8K - ANSI",
}

type transport interface {
	Write([]byte) error
	Read(timeout time.Duration) ([]byte, error) // nil, nil on timeout
	Close()
}

type devInfo struct{ Path, Product string }

// crc32RTK is reflected, poly 0xEDB88320, init 0xFFFFFFFF, no final xor (tdfu.c CRC32).
func crc32RTK(buf []byte) uint32 {
	crc := uint32(0xFFFFFFFF)
	for _, b := range buf {
		t := (crc ^ uint32(b)) & 0xFF
		for i := 0; i < 8; i++ {
			if t&1 != 0 {
				t = (t >> 1) ^ 0xEDB88320
			} else {
				t >>= 1
			}
		}
		crc = (crc >> 8) ^ t
	}
	return crc
}

func build(op byte, data []byte) []byte {
	l := byte(len(data) + 3)
	p := []byte{0xAA, 0x56, l, ^l, 0, op}
	p = append(p, data...)
	s := uint16(op)
	for _, b := range data {
		s += uint16(b)
	}
	return append(p, byte(s), byte(s>>8))
}

type dfu struct{ t transport }

func newDFU(t transport) *dfu {
	d := &dfu{t}
	for {
		r, err := t.Read(0)
		if err != nil || r == nil {
			break
		}
	}
	return d
}

func (d *dfu) write(pkt []byte, sn byte) error {
	pkt[4] = sn
	rep := make([]byte, reportLen+1)
	rep[0] = outReportID
	copy(rep[1:], pkt)
	return d.t.Write(rep)
}

// cmd sends a command and waits for the ack matching sn (byte 7) and opcode (byte 8);
// GET_MODEL_INFO's ack spans two reports, so the next report cannot be assumed to be ours.
func (d *dfu) cmd(op byte, data []byte, sn byte, timeout time.Duration) ([]byte, error) {
	if err := d.write(build(op, data), sn); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(timeout)
	for {
		left := time.Until(deadline)
		if left <= 0 {
			return nil, nil
		}
		r, err := d.t.Read(left)
		if err != nil || r == nil {
			return nil, err
		}
		if len(r) > 9 && r[0] == inReportID && r[7] == sn && r[8] == op {
			return r, nil
		}
	}
}

func payload(r []byte) []byte {
	if len(r) < 10 || r[0] != inReportID {
		return nil
	}
	return r[10:]
}

func cstr(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

func handshake(d *dfu, log func(string, ...any)) (bool, error) {
	log(">> GET_MODEL_INFO (0x60)...")
	r, err := d.cmd(opModelInfo, nil, 1, time.Second)
	if err != nil {
		return false, err
	}
	p := payload(r)
	if len(p) < 22 {
		log("!! no response - wrong interface or device not ready")
		return false, nil
	}
	model := cstr(p[:10])
	log("   model = %q   fw = %q", model, cstr(p[12:22]))
	if r, _ = d.cmd(opDfuVersion, nil, 2, time.Second); len(payload(r)) >= 3 {
		log("   dfu_version = %#04x enc_mode = %d", payload(r)[0], payload(r)[2])
	}
	if name, ok := knownModels[model]; ok {
		log("   identity VALID (Keychron %s)", name)
		return true, nil
	}
	log("   identity MISMATCH (%q is not a known Keychron Ultra)", model)
	return false, nil
}

func nextSN(sn byte) byte {
	if sn >= 255 {
		return 1
	}
	return sn + 1
}

func status(r []byte) byte { return r[9] }

// flash uploads data; it never sends IMAGE_SWITCH unless the device verified the CRC.
func flash(d *dfu, data []byte, log func(string, ...any)) (bool, error) {
	crc := crc32RTK(data)
	log(">> image: %d bytes, crc32=0x%08x", len(data), crc)
	log(">> START (0x63) ...")
	r, err := d.cmd(opStart, []byte{0}, 3, time.Second)
	if err != nil {
		return false, err
	}
	if payload(r) == nil {
		log("!! START not acked")
		return false, nil
	}
	if status(r) != 0 {
		log("!! START rejected by device (status=%d) - not uploading.", status(r))
		return false, nil
	}
	sn := byte(4)
	total := (len(data) + chunk - 1) / chunk
	for i := 0; i < len(data); i += chunk {
		end := i + chunk
		if end > len(data) {
			end = len(data)
		}
		r, err = d.cmd(opSendBin, data[i:end], sn, time.Second)
		if err != nil {
			return false, err
		}
		if r == nil {
			log("!! no ack at chunk %d/%d (sn=%d)", i/chunk, total, sn)
			return false, nil
		}
		if status(r) != 0 {
			log("!! chunk %d/%d refused by device (status=%d)", i/chunk, total, status(r))
			return false, nil
		}
		sn = nextSN(sn)
		if (i/chunk)%256 == 0 {
			log("   sent %d/%d chunks", i/chunk, total)
		}
	}
	log("   sent %d/%d chunks", total, total)
	log(">> VERIFY_CRC32 (0x65) ...")
	v := make([]byte, 8)
	for i := 0; i < 8; i++ {
		v[i] = byte(crc >> (8 * (i % 4)))
	}
	r, err = d.cmd(opVerifyCRC, v, sn, 3*time.Second)
	if err != nil {
		return false, err
	}
	sn = nextSN(sn)
	p := payload(r)
	if len(p) == 0 || status(r) != 0 || p[0] != 0 {
		log("!! CRC VERIFY FAILED - NOT switching. Running firmware is untouched.")
		return false, nil
	}
	log("   CRC verified OK by device.")
	log(">> IMAGE_SWITCH (0x66) - keyboard will reboot into the new image ...")
	if err := d.write(build(opSwitch, nil), sn); err != nil {
		return false, err
	}
	time.Sleep(200 * time.Millisecond)
	log("   switch command sent.")
	return true, nil
}

var errNoDevice = errors.New("DFU HID interface (usage page 0x8C) not found. Keyboard connected over USB?")

func pickDevice(found []devInfo, want string) (string, error) {
	if want != "" {
		for _, f := range found {
			if f.Path == want {
				return want, nil
			}
		}
		return "", fmt.Errorf("%s is not a Keychron DFU interface", want)
	}
	switch len(found) {
	case 0:
		return "", errNoDevice
	case 1:
		return found[0].Path, nil
	}
	s := "more than one Keychron DFU interface found:\n"
	for _, f := range found {
		s += fmt.Sprintf("     %s  %s\n", f.Path, f.Product)
	}
	return "", errors.New(s + "   Pass --path=<one of the above>, or unplug the other keyboard.")
}
