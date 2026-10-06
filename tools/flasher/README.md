# Keychron Ultra HID flasher (Windows exe)

Windows build of the Realtek SC-FWU flasher from [naaraxi/zmk](https://github.com/naaraxi/zmk) (`openrgb/flash.py`, MIT). It flashes self-built images (like `firmware/*.bin`) over USB, which the Keychron Launcher refuses.

## Use
Connect the keyboard by USB, close the Keychron Launcher/browser tabs using it, then either:
- drag a `.bin` onto `keychron_flasher.exe`, or
- `keychron_flasher.exe handshake` (read-only identify), or
- `keychron_flasher.exe flash firmware\20261006_K1UltraAnsi_04.bin`

It identifies the keyboard first and refuses unknown models. The image goes to a staging bank and is only activated after the keyboard verifies its CRC; a failed or interrupted upload leaves the running firmware unchanged. With two Keychron boards connected, pass `--path=<HID path>` as listed by the tool.

## Build the exe
Windows cannot be cross-built from this devcontainer, so the exe is built on a Windows runner: GitHub Actions workflow `Build flasher exe` (artifact `keychron_flasher.exe`), or locally on Windows with `build_exe.bat`.

## Test
`python test_protocol.py` runs the protocol against a simulated keyboard (no hardware). It has not been run against a real device yet.
