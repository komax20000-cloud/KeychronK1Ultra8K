#!/usr/bin/env python3
"""Keychron Ultra firmware flasher (Realtek SC-FWU over HID) for Windows/Linux/macOS.

Protocol port of openrgb/flash.py from https://github.com/naaraxi/zmk (MIT, (c) The ZMK
Contributors), which in turn follows the Keychron Launcher's Realtek OTA routine and
app/src/dfu/tdfu.c. Uses the `hid` (hidapi) package instead of /dev/hidraw.

The device stages the image in the "OTA Tmp" bank and only switches to it after
VERIFY_CRC32 succeeds. This tool aborts before IMAGE_SWITCH if the CRC differs.

Usage:
  keychron_flasher.exe                      # prompts for a .bin (or drag a .bin onto the exe)
  keychron_flasher.exe handshake            # read-only: identify the keyboard
  keychron_flasher.exe flash <image.bin>    # START -> SEND -> VERIFY -> SWITCH
  ... --path=<hid path>                     # only if two Keychron boards are attached
"""
import sys
import time

OUT_REPORT_ID = 0xB2
IN_REPORT_ID = 0xB1
HDR0, HDR1_ACK = 0xAA, 0x56
REPORT_LEN = 32
CHUNK = 16
OP_GET_MODEL_INFO = 0x60
OP_GET_DFU_VERSION = 0x61
OP_START = 0x63
OP_SEND_BIN = 0x64
OP_VERIFY_CRC32 = 0x65
OP_IMAGE_SWITCH = 0x66
VENDOR_ID = 0x3434
DFU_USAGE_PAGE = 0x8C

KNOWN_MODELS = {
    b"KCZKV08K": "V0 Ultra 8K - ANSI",
    b"KCZKV18K": "V1 Ultra 8K - ANSI",
    b"KCZKV18I": "V1 Ultra 8K - ISO",
    b"KCZKV18J": "V1 Ultra 8K - JIS",
    b"KCZKV28K": "V2 Ultra 8K - ANSI",
    b"KCZKV38K": "V3 Ultra 8K - ANSI",
    b"KCZKV58K": "V5 Ultra 8K - ANSI",
    b"KCZKV68K": "V6 Ultra 8K - ANSI",
    b"KCZKVA8K": "V10 Ultra 8K - ANSI",
    b"KCZKQ18K": "Q1 Ultra 8K - ANSI",
    b"KCZKQ38K": "Q3 Ultra 8K - ANSI",
    b"KCZKQ68K": "Q6 Ultra 8K - ANSI",
    b"KCZ270U": "Z2-70 Ultra 8K - ANSI",
}


def crc32_rtk(buf, crc=0xFFFFFFFF):
    # Reflected, poly 0xEDB88320, init 0xFFFFFFFF, no final xor (tdfu.c CRC32)
    for b in buf:
        t = (crc ^ b) & 0xFF
        for _ in range(8):
            t = (t >> 1) ^ 0xEDB88320 if (t & 1) else (t >> 1)
        crc = (crc >> 8) ^ t
    return crc & 0xFFFFFFFF


def build(opcode, data=b""):
    length = len(data) + 3
    pkt = bytearray([HDR0, HDR1_ACK, length, (~length) & 0xFF, 0, opcode])
    pkt += data
    s = (opcode + sum(data)) & 0xFFFF
    pkt += bytes([s & 0xFF, (s >> 8) & 0xFF])
    return pkt


class HidTransport:
    """Thin wrapper over hidapi so the protocol code can be tested with a fake."""

    def __init__(self, path):
        import hid
        self.dev = hid.device()
        self.dev.open_path(path)

    def write(self, report):
        n = self.dev.write(report)
        if n < 0:
            raise OSError("HID write failed")

    def read(self, timeout_s):
        data = self.dev.read(REPORT_LEN + 1, int(timeout_s * 1000))
        return bytes(data) if data else None

    def close(self):
        self.dev.close()


def find_dfu_devices():
    import hid
    return [d for d in hid.enumerate(VENDOR_ID, 0) if d.get("usage_page") == DFU_USAGE_PAGE]


class Dfu:
    def __init__(self, transport):
        self.t = transport
        self._drain()

    def _write(self, pkt, sn):
        pkt = bytearray(pkt)
        pkt[4] = sn
        self.t.write(bytes([OUT_REPORT_ID]) + bytes(pkt) + b"\x00" * (REPORT_LEN - len(pkt)))

    def _drain(self):
        while self.t.read(0) is not None:
            pass

    def cmd(self, opcode, data=b"", sn=0, timeout=1.0):
        # GET_MODEL_INFO's ack spans two input reports, so match sn (byte 7) and opcode (byte 8)
        self._write(build(opcode, data), sn)
        deadline = time.monotonic() + timeout
        while True:
            left = deadline - time.monotonic()
            if left <= 0:
                return None
            resp = self.t.read(left)
            if resp is None:
                return None
            if len(resp) > 9 and resp[0] == IN_REPORT_ID and resp[7] == sn and resp[8] == opcode:
                return resp


def parse_model(resp):
    if not resp or resp[0] != IN_REPORT_ID:
        return None
    return resp[10:]


def ack_status(resp):
    return None if not resp else resp[9]


def handshake(d, log=print):
    log(">> GET_MODEL_INFO (0x60)...")
    p = parse_model(d.cmd(OP_GET_MODEL_INFO, sn=1))
    if not p:
        log("!! no response - wrong interface or device not ready")
        return False
    model = bytes(p[:10]).split(b"\x00")[0]
    fw = bytes(p[12:22]).split(b"\x00")[0]
    log(f"   model = {model!r}   fw = {fw!r}")
    p = parse_model(d.cmd(OP_GET_DFU_VERSION, sn=2))
    if p:
        log(f"   dfu_version = {p[0]:#04x} enc_mode = {p[2]}")
    if model in KNOWN_MODELS:
        log(f"   identity VALID (Keychron {KNOWN_MODELS[model]})")
        return True
    log(f"   identity MISMATCH ({model!r} is not a known Keychron Ultra)")
    return False


def flash(d, data, log=print):
    crc = crc32_rtk(data)
    log(f">> image: {len(data)} bytes, crc32=0x{crc:08x}")
    log(">> START (0x63) ...")
    r = d.cmd(OP_START, b"\x00", sn=3)
    if not r or parse_model(r) is None:
        log("!! START not acked")
        return False
    if ack_status(r):
        log(f"!! START rejected by device (status={ack_status(r)}) - not uploading.")
        return False
    sn = 4
    total = (len(data) + CHUNK - 1) // CHUNK
    for i in range(0, len(data), CHUNK):
        r = d.cmd(OP_SEND_BIN, data[i:i + CHUNK], sn=sn)
        if not r:
            log(f"!! no ack at chunk {i // CHUNK}/{total} (sn={sn})")
            return False
        if ack_status(r):
            log(f"!! chunk {i // CHUNK}/{total} refused by device (status={ack_status(r)})")
            return False
        sn = 1 if sn >= 255 else sn + 1
        if (i // CHUNK) % 256 == 0:
            log(f"   sent {i // CHUNK}/{total} chunks")
    log(f"   sent {total}/{total} chunks")
    log(">> VERIFY_CRC32 (0x65) ...")
    r = d.cmd(OP_VERIFY_CRC32, crc.to_bytes(4, "little") * 2, sn=sn, timeout=3.0)
    sn = 1 if sn >= 255 else sn + 1
    p = parse_model(r)
    if not p or ack_status(r) or p[0] != 0:
        log(f"!! CRC VERIFY FAILED (ack_status={ack_status(r)}, "
            f"crc_result={p[0] if p else 'none'}) - NOT switching. Running firmware is untouched.")
        return False
    log("   CRC verified OK by device.")
    log(">> IMAGE_SWITCH (0x66) - keyboard will reboot into the new image ...")
    d._write(build(OP_IMAGE_SWITCH), sn)
    time.sleep(0.2)
    log("   switch command sent.")
    return True


def pick_device(path):
    found = find_dfu_devices()
    if path:
        for dev in found:
            if dev["path"] == path.encode():
                return dev["path"]
        print(f"!! {path} is not a Keychron DFU interface.")
        sys.exit(2)
    if not found:
        print("!! DFU HID interface (usage page 0x8C) not found. Keyboard connected over USB?")
        sys.exit(2)
    if len(found) > 1:
        print("!! more than one Keychron DFU interface found:")
        for dev in found:
            print(f"     {dev['path'].decode(errors='replace')}  {dev.get('product_string')}")
        print("   Pass --path=<one of the above>, or unplug the other keyboard.")
        sys.exit(2)
    return found[0]["path"]


def run(argv):
    chosen = next((a.split("=", 1)[1] for a in argv if a.startswith("--path=")), None)
    args = [a for a in argv if not a.startswith("--")]
    # Drag-and-drop or double-click: a lone .bin path means flash it, no args means ask
    if len(args) == 1 and args[0].lower().endswith(".bin"):
        args = ["flash", args[0]]
    elif not args:
        entered = input("Firmware .bin path (empty = handshake only): ").strip().strip('"')
        args = ["flash", entered] if entered else ["handshake"]
    mode = args[0]

    if mode not in ("handshake", "flash") or (mode == "flash" and len(args) < 2):
        print(__doc__)
        return 2
    image = None
    if mode == "flash":
        try:
            image = open(args[1], "rb").read()
        except OSError as e:
            print(f"!! cannot read image: {e}")
            return 2

    path = pick_device(chosen)
    try:
        t = HidTransport(path)
    except OSError as e:
        print(f"!! cannot open the keyboard: {e}\n   Close the Keychron Launcher / other HID tools and retry.")
        return 2
    try:
        d = Dfu(t)
        if not handshake(d):
            print("!! handshake failed - aborting before any write.")
            return 1
        if mode == "handshake":
            return 0
        return 0 if flash(d, image) else 1
    finally:
        t.close()


def main():
    try:
        rc = run(sys.argv[1:])
    except SystemExit as e:
        rc = e.code if isinstance(e.code, int) else 1
    # Keep the console open when started by double-click or drag-and-drop
    if getattr(sys, "frozen", False) and len(sys.argv) <= 2 and sys.stdin and sys.stdin.isatty():
        input("Press Enter to close...")
    return rc


if __name__ == "__main__":
    sys.exit(main())
