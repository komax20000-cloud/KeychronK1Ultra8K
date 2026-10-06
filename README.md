# Keychron K1 Ultra ANSI custom ZMK module
Customizations for the Keychron K1 Ultra ANSI, built on Keychron's Realtek-enabled ZMK fork: keymap, `.conf`, key auto-repeat behaviors, the repeat LED indicator, and two custom key slots. Everything else (Zephyr, HAL, ZMK app, drivers) still comes from the fork, which is pinned.

## Prepare the build workspace (requires network access)

Run this once while online. Keep the resulting ZMK workspace and its `.west` directory; west-managed dependencies are checked out there.

```sh
# inside the image named by BUILD_IMAGE in deps.env
/workspaces/KeychronK1Ultra8K/scripts/setup-workspace.sh
```

The script checks out the Keychron ZMK fork, Keychron's Zephyr and the Realtek HAL at the exact commits pinned in [deps.env](deps.env) (the fork's `west.yml` only names branches), so builds do not drift when those branches move. It points the fork's `app/west.yml` at the pinned Zephyr commit before `west update` (so Zephyr's module list is the pinned one too; this edit stays inside the workspace clone), then verifies every west project against [deps.lock](deps.lock) and fails on any difference. Set `ZMK_DIR` to use a workspace location other than `/workspaces/zmk`.

To upgrade, change the revisions in `deps.env`, run the setup on a fresh `ZMK_DIR`, and regenerate `deps.lock` from it. Also re-check `src/key_auto_repeat.c` (`zmk_keycode_state_changed` event) and `src/key_repeat_led.c` (wrapped `zmk_rgb_matrix_update_pwm_buffers`), which depend on fork internals.

The machine must also have `west`, the Python packages required by this ZMK fork, and the ARM toolchain installed before disconnecting from the network. If using a container, download the build image in advance as well.

## Build offline

After workspace preparation and installation of the build tools, disconnect from the network and run:

```sh
/workspaces/KeychronK1Ultra8K/build.sh
```

The script runs the equivalent of `west build -s app -d app/build -p -b rtl8762gtu_kb -- -DSHIELD=keychron_k1_ultra_ansi -DZMK_EXTRA_MODULES=<this repo> -DKEYMAP_FILE=<module keymap> -DEXTRA_CONF_FILE=<module conf>` in `ZMK_DIR` (default `/workspaces/zmk`) and builds with this repo's keymap (`boards/shields/keychron_k1_ultra_ansi/keychron_k1_ultra_ansi.keymap`) instead of the one in the ZMK fork, merges this repo's `keychron_k1_ultra_ansi.conf` on top of the fork's shield `.conf` (module values win; settings that exist only in the fork's `.conf` still apply), and saves the image and that keymap as `firmware/YYYYMMDD_K1UltraAnsi_NN.bin` and `.keymap` in this repo. `NN` starts at `00` and auto-increments for each build on the same day.

Do not run `west update` while offline: it fetches projects from their remotes. Offline builds work as long as the ZMK checkout, all west-managed projects, and the required build tools remain available locally. The `-p` option starts from a clean build directory; it does not fetch dependencies.

The module's `boards/shields/keychron_k1_ultra_ansi/` holds only the keymap and `.conf`. The shield definition (overlay, LED headers) and the board come from the fork, and build only through `build.sh`: a plain `west build` would use the fork's keymap and `.conf` and silently skip this module's.

## CI

[.github/workflows/build.yml](.github/workflows/build.yml) runs the same `setup-workspace.sh` + `build.sh` in the digest-pinned image and uploads `firmware/*` as an artifact. Keep its `image:` in sync with `BUILD_IMAGE` in `deps.env`.

## Key auto-repeat LED indication

While key auto-repeat is enabled (`repeat_toggle 1`), the F1–F12 LEDs are forced into a pulsing wave and the Up key LED is kept lit, regardless of the active RGB effect. Both turn off again when repeat is disabled (`repeat_toggle 0`). Implemented in `src/key_repeat_led.c` by wrapping `zmk_rgb_matrix_update_pwm_buffers()` at link time, so the ZMK fork is not modified. The overlay uses the current RGB hue/saturation and brightness, and is not drawn while RGB is off or the keyboard is asleep.

## Flasher

[tools/flasher/](tools/flasher) has a Windows exe flasher for the built `.bin` (see its README).
