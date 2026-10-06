"""Runs the flash protocol against a simulated keyboard (no hardware needed)."""
import keychron_flasher as k

class FakeKbd:
    def __init__(self, corrupt=False):
        self.q, self.recv, self.corrupt, self.switched = [], bytearray(), corrupt, False
    def write(self, report):
        assert report[0] == k.OUT_REPORT_ID and len(report) == 33
        p = report[1:]
        assert p[0] == 0xAA and p[1] == 0x56 and p[3] == (~p[2]) & 0xFF
        sn, op, dlen = p[4], p[5], p[2] - 3
        data = p[6:6 + dlen]
        assert (p[6 + dlen] | p[7 + dlen] << 8) == (op + sum(data)) & 0xFFFF
        pay = b""
        if op == k.OP_GET_MODEL_INFO:
            pay = b"KCZKV18K\0\0\0\0" + b"V1.0.2\0\0\0\0"
        elif op == k.OP_GET_DFU_VERSION:
            pay = bytes([1, 0, 0])
        elif op == k.OP_SEND_BIN:
            self.recv += data
        elif op == k.OP_VERIFY_CRC32:
            ok = k.crc32_rtk(self.recv) == int.from_bytes(data[:4], "little") and not self.corrupt
            pay = bytes([0 if ok else 1])
        elif op == k.OP_IMAGE_SWITCH:
            self.switched = True
            return
        self.q.append(bytes([k.IN_REPORT_ID, 0xAA, 0x56, 0, 0, 0, 0, sn, op, 0]) + pay)
    def read(self, t):
        return self.q.pop(0) if self.q else None

def test_flash_ok():
    kb = FakeKbd(); img = bytes(range(256)) * 50 + b"xyz"
    assert k.handshake(k.Dfu(kb), log=lambda *_: None)
    assert k.flash(k.Dfu(kb), img, log=lambda *_: None)
    assert bytes(kb.recv) == img and kb.switched

def test_bad_crc_does_not_switch():
    kb = FakeKbd(corrupt=True)
    assert not k.flash(k.Dfu(kb), b"a" * 100, log=lambda *_: None)
    assert not kb.switched
