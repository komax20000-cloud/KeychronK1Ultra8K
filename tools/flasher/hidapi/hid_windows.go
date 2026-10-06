//go:build windows

// Package hidapi wraps the bundled libusb/hidapi (Windows backend, BSD license) via cgo.
package hidapi

/*
#cgo LDFLAGS: -lsetupapi -lhid
#include <stdlib.h>
#include "hidapi.h"
*/
import "C"

import (
	"errors"
	"unsafe"
)

type Info struct {
	Path        string
	UsagePage   uint16
	ProductName string
}

type Device struct{ h *C.hid_device }

// Enumerate lists HID interfaces of a vendor id.
func Enumerate(vendor uint16) []Info {
	var out []Info
	list := C.hid_enumerate(C.ushort(vendor), 0)
	for p := list; p != nil; p = p.next {
		out = append(out, Info{
			Path:        C.GoString(p.path),
			UsagePage:   uint16(p.usage_page),
			ProductName: wstr(p.product_string),
		})
	}
	C.hid_free_enumeration(list)
	return out
}

func wstr(w *C.wchar_t) string {
	if w == nil {
		return ""
	}
	var r []rune
	for i := 0; ; i++ {
		c := *(*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(w)) + uintptr(i)*2))
		if c == 0 {
			break
		}
		r = append(r, rune(c))
	}
	return string(r)
}

func Open(path string) (*Device, error) {
	cp := C.CString(path)
	defer C.free(unsafe.Pointer(cp))
	h := C.hid_open_path(cp)
	if h == nil {
		return nil, errors.New("hid_open_path failed")
	}
	return &Device{h}, nil
}

func (d *Device) Write(b []byte) error {
	if C.hid_write(d.h, (*C.uchar)(unsafe.Pointer(&b[0])), C.size_t(len(b))) < 0 {
		return errors.New("HID write failed")
	}
	return nil
}

// Read returns nil on timeout.
func (d *Device) Read(n, timeoutMs int) ([]byte, error) {
	buf := make([]byte, n)
	r := C.hid_read_timeout(d.h, (*C.uchar)(unsafe.Pointer(&buf[0])), C.size_t(n), C.int(timeoutMs))
	if r < 0 {
		return nil, errors.New("HID read failed")
	}
	if r == 0 {
		return nil, nil
	}
	return buf[:r], nil
}

func (d *Device) Close() { C.hid_close(d.h) }
