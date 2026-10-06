# Realtek HID firmware flasher

`Realtek_HID_flasher.exe` is a Windows x64 console application for flashing Realtek SC-FWU firmware images over the keyboard's HID interface. It is included in [`firmware/`](../../firmware/Realtek_HID_flasher.exe) and is intended for `.bin` images built for these keyboards, including the K1 Ultra 8K ANSI.

The Go implementation is based on the Realtek SC-FWU flasher in [naaraxi/zmk](https://github.com/naaraxi/zmk) (`openrgb/flash.py`, MIT). HID communication uses the bundled Windows backend from [hidapi](https://github.com/libusb/hidapi) (BSD license).

## Before flashing

1. Connect the keyboard over USB and make its Realtek DFU HID interface available.
2. Close Keychron Launcher and any other application using the keyboard's HID interface.
3. Use a firmware image built for the connected keyboard. The flasher recognizes these model IDs:
   - K1 Ultra 8K ANSI (`KCZKK18K`)
   - V0 Ultra 8K ANSI (`KCZKV08K`)
   - V1 Ultra 8K ANSI / ISO / JIS (`KCZKV18K`, `KCZKV18I`, `KCZKV18J`)
   - V2, V3, V5, V6, and V10 Ultra 8K ANSI (`KCZKV28K`, `KCZKV38K`, `KCZKV58K`, `KCZKV68K`, `KCZKVA8K`)
   - Q1, Q3, and Q6 Ultra 8K ANSI (`KCZKQ18K`, `KCZKQ38K`, `KCZKQ68K`)
   - Z2-70 Ultra 8K ANSI (`KCZ270U`)

Recognition only identifies the connected keyboard; it does not verify that an image is the correct firmware for that model. Always check the image before confirming.

## Choose and flash an image

Run the executable with no arguments to list `.bin` files in the same folder as the executable. Use Up/Down to change the selection, Enter to choose it, or Esc to cancel. The flasher then asks for confirmation:

```text
Really flash <filename>? [Y/N]
```

Press `Y` to continue or `N` (or Esc) to stop. The executable also asks for confirmation when started with a firmware path or by dragging a `.bin` onto it.

From the repository root, examples are:

```bat
firmware\Realtek_HID_flasher.exe
firmware\Realtek_HID_flasher.exe firmware\YYYYMMDD_K1UltraAnsi_NN.bin
firmware\Realtek_HID_flasher.exe flash firmware\YYYYMMDD_K1UltraAnsi_NN.bin
firmware\Realtek_HID_flasher.exe handshake
```

`handshake` only identifies the keyboard and does not flash. With two supported keyboards connected, add `--path=<HID path>` to select one explicitly. Unknown models and failed handshakes abort before any firmware data is written.

During a flash, the image is sent to a staging bank. The keyboard must verify its CRC before the flasher asks it to activate the image.

## Build and test

From this directory, run:

```sh
go test ./...
```

On Linux, `./build.sh` runs the Go tests and cross-compiles the Windows executable into `firmware/`. It requires Go and the Python `ziglang` package. The protocol tests use a simulated keyboard and do not replace testing on Windows with the target hardware.
