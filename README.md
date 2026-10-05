# Keychron K1 Ultra ANSI custom ZMK module
This repo contains only the board and shield files required to build Keychron K1 Ultra ANSI on ZMK.
Build from the upstream ZMK app:

cd /workspaces/zmk/app
west build -p -b rtl8762gtu_kb -- -DSHIELD=keychron_k1_ultra_ansi -DZMK_EXTRA_MODULES="/workspaces/keychron-k1-ultra-zmk"

This custom repo is intentionally minimal and only includes the required Keychron board/shield definitions.
