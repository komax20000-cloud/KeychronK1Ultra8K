package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFirmwareImages(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"z.bin", "A.BIN", "not-firmware.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "folder.bin"), 0700); err != nil {
		t.Fatal(err)
	}

	got, err := firmwareImages(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(dir, "A.BIN"), filepath.Join(dir, "z.bin")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("firmwareImages() = %v, want %v", got, want)
	}
}

func TestMoveSelection(t *testing.T) {
	tests := []struct {
		name  string
		index int
		count int
		key   rune
		want  int
	}{
		{"up", 1, 3, keyUp, 0},
		{"down", 1, 3, keyDown, 2},
		{"clamp at first", 0, 3, keyUp, 0},
		{"clamp at last", 2, 3, keyDown, 2},
		{"single item", 0, 1, keyDown, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := moveSelection(tt.index, tt.count, tt.key); got != tt.want {
				t.Fatalf("moveSelection() = %d, want %d", got, tt.want)
			}
		})
	}
}
