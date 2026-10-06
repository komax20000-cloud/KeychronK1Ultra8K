#!/usr/bin/env bash
# Cross-builds the Windows exe from Linux (Go + the bundled hidapi via zig cc) into ../../firmware.
# Needs: go and `pip install ziglang`.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
go test ./...
WRAP="$(mktemp)"
trap 'rm -f "$WRAP"' EXIT
printf '#!/bin/sh\nexec python3 -m ziglang cc -target x86_64-windows-gnu "$@"\n' > "$WRAP"
chmod +x "$WRAP"
mkdir -p ../../firmware
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC="$WRAP" \
  go build -trimpath -ldflags "-s -w" -o ../../firmware/Realtek_HID_flasher.exe .
ls -l ../../firmware/Realtek_HID_flasher.exe
