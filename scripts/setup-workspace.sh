#!/usr/bin/env bash
# Prepares the ZMK/Zephyr workspace at the exact revisions listed in deps.env.
# Needs network access. Run inside the build image (see BUILD_IMAGE in deps.env).
set -euo pipefail

MODULE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck disable=SC1091
source "$MODULE_DIR/deps.env"
ZMK_DIR="${ZMK_DIR:-/workspaces/zmk}"

git config --global --add safe.directory '*'

# Fetches and checks out an exact commit in a git directory
checkout_rev() {
  local dir="$1" rev="$2" remote
  if [ "$(git -C "$dir" rev-parse HEAD 2>/dev/null || true)" = "$rev" ]; then
    return
  fi
  remote="$(git -C "$dir" remote | head -n1)"
  git -C "$dir" fetch --depth 1 "$remote" "$rev"
  git -C "$dir" checkout -q --detach "$rev"
}

if [ ! -d "$ZMK_DIR/.git" ]; then
  mkdir -p "$ZMK_DIR"
  git init -q "$ZMK_DIR"
  git -C "$ZMK_DIR" remote add origin "$ZMK_REPO"
fi
checkout_rev "$ZMK_DIR" "$ZMK_REV"

cd "$ZMK_DIR"
# Point Zephyr at the pinned commit so the module list imported from its west.yml is the pinned one too
sed -i -E "/name: zephyr/,/revision:/ s/(revision:).*/\\1 $ZEPHYR_REV/" app/west.yml
[ -d .west ] || west init -l app
west update
# hal_realtek is declared by Zephyr's west.yml with a branch name; put it back on the pinned commit
checkout_rev modules/hal/realtek "$HAL_REALTEK_REV"
west zephyr-export

current_shas() {
  local n
  for n in $(west list -f '{name}' | grep -vx manifest); do
    echo "$n $(west list -f '{sha}' "$n")"
  done | sort
}

# Verify every project is at the commit recorded in deps.lock
if ! diff <(current_shas) <(sort "$MODULE_DIR/deps.lock"); then
  echo "Workspace differs from deps.lock" >&2
  exit 1
fi
