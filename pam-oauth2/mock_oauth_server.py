#!/usr/bin/env python3
import os, time, json, uuid, hashlib
from datetime import datetime, timezone
from flask import Flask, request, jsonify, make_response, g

HOST = os.getenv("HOST", "0.0.0.0")
PORT = int(os.getenv("PORT", "8000"))
# If True, log full bodies; otherwise only length + sha256
LOG_BODIES = os.getenv("LOG_BODIES", "false").lower() in {"1","true","yes"}

app = Flask(__name__)

def now_iso():
    return datetime.now(timezone.utc).isoformat()

def client_ip():
    # Prefer X-Forwarded-For if you front this with a proxy
    xff = request.headers.get("X-Forwarded-For")
    return (xff.split(",")[0].strip() if xff else request.remote_addr)

def sanitize_headers(hdrs):
    redacts = {"authorization", "cookie", "x-api-key"}
    out = {}
    for k, v in hdrs.items():
        out[k] = "<redacted>" if k.lower() in redacts else v
    return out

@app.before_request
def _start_timer():
    g._t0 = time.perf_counter()
    g._rid = uuid.uuid4().hex  # request id
    # Read body early so we can log it once
    g._raw = request.get_data(cache=True) or b""

@app.after_request
def _log(resp):
    dur_ms = int((time.perf_counter() - g._t0) * 1000)
    body_len = len(g._raw)
    body_sha = hashlib.sha256(g._raw).hexdigest() if body_len else None
    log = {
        "ts": now_iso(),
        "rid": g._rid,
        "remote": client_ip(),
        "method": request.method,
        "path": request.path,
        "query": request.query_string.decode("utf-8", "replace"),
        "status": resp.status_code,
        "duration_ms": dur_ms,
        "headers": sanitize_headers(request.headers),
        "body_len": body_len,
        "body_sha256": body_sha,
    }
    if LOG_BODIES and body_len:
        # Best effort decode
        try:
            log["body_preview"] = g._raw.decode("utf-8", "replace")
        except Exception:
            log["body_preview"] = "<binary>"
    print(json.dumps(log, ensure_ascii=False), flush=True)
    return resp

def token_payload(uid: str):
    # The exact object your PAM tests expect
    return {
        "access_token": "bar",
        "expires_in": 3598,
        "grp": "tester",
        "scope": ["uid"],
        "token_type": "Bearer",
        "uid": uid,
    }

@app.get("/")
def tokeninfo_root():
    """Simulate a tokeninfo endpoint using query: ?access_token=..."""
    token = request.args.get("access_token", "")
    # === happy path ===
    if token == "bar":
        return jsonify(token_payload(uid="foo")), 200

    # === knobs for tests ===
    if token == "badstatus":

        return jsonify({"error": "unauthorized"}), 401
    if token == "wronguid":
        return jsonify(token_payload(uid="someoneelse")), 200
    if token == "wronggrp":
        obj = token_payload(uid="foo")
        obj["grp"] = "not-tester"
        return jsonify(obj), 200
    if token == "malformed":
        return jsonify({"access_token": "malformed"}), 200

    # default: unauthorized
    return jsonify({"error": "unauthorized"}), 401

@app.get("/__health")
def health():
    return jsonify({"ok": True, "ts": now_iso()}), 200

if __name__ == "__main__":
    # threaded=True is fine here; Flask dev server is sufficient for tests
    app.run(host=HOST, port=PORT, threaded=True)
