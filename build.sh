#!/usr/bin/env bash
# Builds Keychron K1 Ultra ANSI and copies the image and keymap to firmware/YYYYMMDD_K1UltraAnsi_NN.{bin,keymap} (NN auto-increments)
set -euo pipefail

MODULE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ZMK_DIR="${ZMK_DIR:-/workspaces/zmk}"
# Use this module's keymap instead of the one bundled in the ZMK fork
KEYMAP_FILE="$MODULE_DIR/boards/shields/keychron_k1_ultra_ansi/keychron_k1_ultra_ansi.keymap"
# Merged after the fork's shield .conf, so values set here take precedence
CONF_FILE_EXTRA="$MODULE_DIR/boards/shields/keychron_k1_ultra_ansi/keychron_k1_ultra_ansi.conf"

cd "$ZMK_DIR"
west build -s app -d app/build -p -b rtl8762gtu_kb -- \
  -DSHIELD=keychron_k1_ultra_ansi \
  -DZMK_EXTRA_MODULES="$MODULE_DIR" \
  -DKEYMAP_FILE="$KEYMAP_FILE" \
  -DEXTRA_CONF_FILE="$CONF_FILE_EXTRA"

mkdir -p "$MODULE_DIR/firmware"
DATE="$(date +%Y%m%d)"
N=0
while [ -e "$MODULE_DIR/firmware/${DATE}_K1UltraAnsi_$(printf '%02d' "$N").bin" ] ||
      [ -e "$MODULE_DIR/firmware/${DATE}_K1UltraAnsi_$(printf '%02d' "$N").keymap" ]; do
  N=$((N + 1))
done
BASE="$MODULE_DIR/firmware/${DATE}_K1UltraAnsi_$(printf '%02d' "$N")"
cp "$ZMK_DIR/app/build/zephyr/zmk.bin" "$BASE.bin"
cp "$KEYMAP_FILE" "$BASE.keymap"
echo "Firmware saved to $BASE.bin"
echo "Keymap saved to $BASE.keymap"
