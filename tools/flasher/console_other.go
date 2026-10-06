//go:build !windows

package main

import "errors"

func readMenuKey() (rune, error) {
	return 0, errors.New("interactive firmware selection requires a Windows console")
}
