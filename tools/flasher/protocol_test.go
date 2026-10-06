package main

import (
	"bytes"
	"testing"
	"time"
)

// fakeKbd simulates a keyboard so the flash sequence can be tested without hardware.
type fakeKbd struct {
	q        [][]byte
	recv     []byte
	corrupt  bool
	switched bool
}

func (f *fakeKbd) Write(r []byte) error {
	p := r[1:]
	if r[0] != outReportID || len(r) != 33 || p[0] != 0xAA || p[1] != 0x56 || p[3] != ^p[2] {
		panic("bad frame")
	}
	sn, op, dl := p[4], p[5], int(p[2])-3
	data := p[6 : 6+dl]
	s := uint16(op)
	for _, b := range data {
		s += uint16(b)
	}
	if uint16(p[6+dl])|uint16(p[7+dl])<<8 != s {
		panic("bad checksum")
	}
	var pay []byte
	switch op {
	case opModelInfo:
		pay = append([]byte("KCZKV18K\x00\x00\x00\x00"), []byte("V1.0.2\x00\x00\x00\x00")...)
	case opDfuVersion:
		pay = []byte{1, 0, 0}
	case opSendBin:
		f.recv = append(f.recv, data...)
	case opVerifyCRC:
		ok := crc32RTK(f.recv) == uint32(data[0])|uint32(data[1])<<8|uint32(data[2])<<16|uint32(data[3])<<24 && !f.corrupt
		pay = []byte{map[bool]byte{true: 0, false: 1}[ok]}
	case opSwitch:
		f.switched = true
		return nil
	}
	f.q = append(f.q, append([]byte{inReportID, 0xAA, 0x56, 0, 0, 0, 0, sn, op, 0}, pay...))
	return nil
}
func (f *fakeKbd) Read(time.Duration) ([]byte, error) {
	if len(f.q) == 0 {
		return nil, nil
	}
	r := f.q[0]
	f.q = f.q[1:]
	return r, nil
}
func (f *fakeKbd) Close() {}

func quiet(string, ...any) {}

func TestFlashOK(t *testing.T) {
	kb := &fakeKbd{}
	img := append(bytes.Repeat([]byte{1, 2, 3, 4, 5}, 5000), 'x', 'y', 'z')
	if ok, _ := handshake(newDFU(kb), quiet); !ok {
		t.Fatal("handshake")
	}
	if ok, err := flash(newDFU(kb), img, quiet); !ok || err != nil {
		t.Fatal("flash", err)
	}
	if !bytes.Equal(kb.recv, img) || !kb.switched {
		t.Fatal("image mismatch or no switch")
	}
}

func TestBadCRCDoesNotSwitch(t *testing.T) {
	kb := &fakeKbd{corrupt: true}
	if ok, _ := flash(newDFU(kb), bytes.Repeat([]byte{7}, 100), quiet); ok || kb.switched {
		t.Fatal("must not switch")
	}
}

func TestCRCKnown(t *testing.T) {
	// no final xor: equals ^(standard CRC-32) = 0x340BC6D9 for "123456789"
	if got := crc32RTK([]byte("123456789")); got != 0x340BC6D9 {
		t.Fatalf("%08x", got)
	}
}

func TestK1UltraAnsiKnown(t *testing.T) {
	if knownModels["KCZKK18K"] == "" {
		t.Fatal("K1 Ultra ANSI model string missing")
	}
}
