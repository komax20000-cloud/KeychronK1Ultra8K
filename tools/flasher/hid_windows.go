//go:build windows

package main

import (
	"time"

	"keychronflasher/hidapi"
)

type hidTransport struct{ d *hidapi.Device }

func (t hidTransport) Write(b []byte) error { return t.d.Write(b) }
func (t hidTransport) Read(timeout time.Duration) ([]byte, error) {
	return t.d.Read(reportLen+1, int(timeout/time.Millisecond))
}
func (t hidTransport) Close() { t.d.Close() }

func findDFU() []devInfo {
	var out []devInfo
	for _, i := range hidapi.Enumerate(vendorID) {
		if i.UsagePage == dfuUsagePage {
			out = append(out, devInfo{i.Path, i.ProductName})
		}
	}
	return out
}

func openDevice(path string) (transport, error) {
	d, err := hidapi.Open(path)
	if err != nil {
		return nil, err
	}
	return hidTransport{d}, nil
}
