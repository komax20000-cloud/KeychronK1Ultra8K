# Keychron K1 Ultra 8K ANSI ZMK module

This repository contains the custom ZMK module for the Keychron K1 Ultra 8K ANSI. It provides this project's keymap and configuration, device-side key auto-repeat behaviors, an RGB indicator for repeat mode, and two user-editable key slots.

The board definition, shield implementation, ZMK application, Zephyr, and Realtek HAL come from Keychron's Realtek-enabled ZMK fork. The fork and its west-managed dependencies are pinned to exact revisions; this module does not modify the workspace's ZMK sources.

## Features

- Custom keymap and shield configuration for the K1 Ultra 8K ANSI.
- Optional device-side auto-repeat for held keyboard keys:
  - Repeat is off by default. `Fn+Up` enables it; `Fn+Down` disables it.
  - `Fn+Enter` pauses repeat for five seconds. Press it again during the pause to restart the timer.
  - `Fn+Z`, `Fn+X`, and `Fn+C` set the first-repeat delay to 10, 20, and 50 ms. The default is 20 ms.
  - `Fn+Left` and `Fn+Right` adjust the repeat interval in 5 ms steps from 10 to 100 ms. The default is 30 ms.
  - `Fn+V` toggles multi-key repeat. When enabled, up to six held keys repeat; otherwise only the most recently pressed key repeats. Multi-key mode is enabled and disabled along with repeat.
  - Modifiers, lock keys, and non-keyboard HID usages do not repeat. Delay and interval settings return to their defaults after reboot.
- While repeat is enabled and RGB is on, the F1–F12 LEDs pulse in a wave and the Up LED stays lit. The indicator follows the configured RGB hue and saturation, and is hidden while the keyboard is asleep.
- Two unassigned custom key slots, `&custom_key_1` and `&custom_key_2`, that can be bound to a key, shortcut, or macro in the keymap.

## Build firmware

### Prepare the workspace

Workspace setup requires network access and the build image specified by `BUILD_IMAGE` in [deps.env](deps.env). Run the setup script inside that image:

```sh
/workspaces/KeychronK1Ultra8K/scripts/setup-workspace.sh
```

The script checks out Keychron's ZMK fork, Zephyr, and the Realtek HAL at the exact commits in `deps.env`, runs `west update`, and verifies all west projects against [deps.lock](deps.lock). Keep the resulting ZMK workspace, including its `.west` directory, for subsequent builds. Set `ZMK_DIR` if the workspace is not at `/workspaces/zmk`.

Before building, install `west`, the Python packages required by the fork, and the ARM toolchain in the build environment. If you build in a container, make sure the image is already available locally before going offline.

### Build

After setup, the firmware can be built offline:

```sh
/workspaces/KeychronK1Ultra8K/build.sh
```

The script starts from a clean build directory and invokes `west build` for board `rtl8762gtu_kb` and shield `keychron_k1_ultra_ansi`. It explicitly selects this repository's keymap and merges this repository's shield configuration over the configuration supplied by the fork.

Each successful build copies the firmware and the keymap used to build it into `firmware/`:

```text
firmware/YYYYMMDD_K1UltraAnsi_NN.bin
firmware/YYYYMMDD_K1UltraAnsi_NN.keymap
```

`NN` starts at `00` and increments to avoid overwriting another build made on the same day.

Do not run `west update` while offline; it fetches projects from their remotes. The clean-build option (`-p`) does not fetch dependencies. Use `build.sh` rather than a plain `west build`: it supplies this module and its keymap and configuration, which a direct build would otherwise omit.

### Update pinned dependencies

Change the revisions in `deps.env`, then run `setup-workspace.sh` against a fresh `ZMK_DIR` and update `deps.lock` with the resolved west project SHAs. Review `src/key_auto_repeat.c` and `src/key_repeat_led.c` when changing the fork revision; they rely on ZMK internals, including its keycode event and RGB buffer-update function.

## Continuous integration

[.github/workflows/build.yml](.github/workflows/build.yml) prepares a fresh workspace and builds the firmware in a digest-pinned container. It uploads the contents of `firmware/` as the `K1UltraAnsi-firmware` artifact. Keep the workflow's container image in sync with `BUILD_IMAGE` in `deps.env`.

## Flash firmware

The [Windows Realtek HID flasher](tools/flasher/README.md) can flash the generated `.bin` files. Read its instructions and verify the image is intended for the connected keyboard before flashing.
