"""Loopback-only deterministic protocol stub for the isolated image trial.

This never evaluates image quality and must never be used as a paid/production
provider or as evidence that generated imagery meets real QA requirements.
"""

import base64
import json
import struct
import zlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


def white_png():
    def chunk(kind, data):
        payload = kind + data
        return struct.pack(">I", len(data)) + payload + struct.pack(">I", zlib.crc32(payload) & 0xFFFFFFFF)

    rows = (b"\x00" + b"\xff\xff\xff" * 2) * 2
    return b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", 2, 2, 8, 2, 0, 0, 0)) + chunk(b"IDAT", zlib.compress(rows)) + chunk(b"IEND", b"")


IMAGE_REPLY = json.dumps({"data": [{"b64_json": base64.b64encode(white_png()).decode("ascii")}]}, separators=(",", ":")).encode()
REVIEW_REPLY = json.dumps({
    "id": "isolated-controlled-review",
    "choices": [{"message": {"role": "assistant", "content": json.dumps({"score": 0.9, "needs_human_review": False, "reasons": []}, separators=(",", ":"))}}],
    "usage": {"prompt_tokens": 3, "completion_tokens": 4, "total_tokens": 7},
}, separators=(",", ":")).encode()


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *_args):
        pass

    def do_GET(self):
        if self.path != "/healthz":
            self.send_error(404)
            return
        self.send_response(200)
        self.end_headers()

    def do_POST(self):
        try:
            size = int(self.headers.get("Content-Length", "-1"))
        except ValueError:
            size = -1
        if size < 0 or size > 8 * 1024 * 1024 or self.path not in ("/v1/images/edits", "/v1/chat/completions"):
            self.send_error(404 if self.path not in ("/v1/images/edits", "/v1/chat/completions") else 413)
            return
        self.rfile.read(size)
        body = IMAGE_REPLY if self.path == "/v1/images/edits" else REVIEW_REPLY
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


ThreadingHTTPServer(("127.0.0.1", 18080), Handler).serve_forever()
