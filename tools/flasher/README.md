# Realtek HID flasher (Windows exe)

Go port of the Realtek SC-FWU flasher from [naaraxi/zmk](https://github.com/naaraxi/zmk) (`openrgb/flash.py`, MIT). It talks to the keyboard through the bundled [hidapi](https://github.com/libusb/hidapi) (Windows backend, BSD license, in `hidapi/`) and flashes self-built images such as `firmware/*.bin`, which the Keychron Launcher refuses. The built exe is committed as `firmware/Realtek_HID_flasher.exe`.

Supported boards: the Ultra models from the original tool plus **K1 Ultra 8K ANSI** (`KCZKK18K`).

## Use
Connect the keyboard by USB and close the Keychron Launcher/browser tabs using it, then:
- drag a `.bin` onto `Realtek_HID_flasher.exe`, or
- `Realtek_HID_flasher.exe handshake` (read-only identify), or
- `Realtek_HID_flasher.exe flash firmware\20261006_K1UltraAnsi_04.bin`

It identifies the keyboard first and aborts before any write on an unknown model. The image goes to a staging bank and is only activated after the keyboard verifies its CRC. With two Keychron boards connected, pass `--path=<HID path>` as listed by the tool.

## Build
`./build.sh` (Linux, needs `go` and `pip install ziglang`) runs the tests and cross-compiles the exe into `firmware/`.

## Test
`go test ./...` runs the protocol against a simulated keyboard. The exe has not been run on Windows or against a real keyboard yet.
