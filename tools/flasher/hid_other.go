//go:build !windows

package main

import "errors"

func findDFU() []devInfo { return nil }

func openDevice(string) (transport, error) {
	return nil, errors.New("this flasher only supports Windows")
}
