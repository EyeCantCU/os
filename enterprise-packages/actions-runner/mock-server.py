from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json, threading

TENANT_PREFIX = "/fake/_actions"
RESOURCE_ID = "a8c47e17-4d56-4a56-92bb-de7ea7dc65be"

def host_base(h):
    return f"http://{h.headers.get('Host','localhost')}"

class H(BaseHTTPRequestHandler):
    def log_message(self, *a): pass

    def _json(self, code, body):
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(body).encode())

    def do_OPTIONS(self):
        self.send_response(204)
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "*")
        self.end_headers()

    def do_POST(self):
        if self.path == "/api/v3/actions/runner-registration":
            self._json(200, {
                "url": host_base(self) + TENANT_PREFIX,
                "token_schema": "OAuthAccessToken",
                "token": "dummy"
            })
            return
        if self.path.startswith(f"{TENANT_PREFIX}/_apis/actions/"):
            self._json(200, {"id": 123, "name": "local", "status": "online"})
            return
        self.send_error(404)

    def do_GET(self):
        if self.path.startswith(f"{TENANT_PREFIX}/_apis/connectionData"):
            base = host_base(self) + TENANT_PREFIX
            self._json(200, {
                "locationServiceData": {
                    "serviceDefinitions": [
                        {
                            "identifier": RESOURCE_ID,
                            "serviceType": "Actions",
                            "displayName": "Mock Actions",
                            "locationMappings": [
                                {"accessMappingMoniker": "Host", "location": base}
                            ]
                        }
                    ]
                }
            })
            return
        if self.path == f"{TENANT_PREFIX}/_apis/resourceAreas":
            base = host_base(self) + TENANT_PREFIX
            self._json(200, {
                "count": 1,
                "value": [{"id": RESOURCE_ID, "name": "Actions", "locationUrl": base}]
            })
            return
        if self.path == f"{TENANT_PREFIX}/_apis/resourceAreas/{RESOURCE_ID}":
            base = host_base(self) + TENANT_PREFIX
            self._json(200, {"id": RESOURCE_ID, "name": "Actions", "locationUrl": base})
            return
        if self.path.startswith(f"{TENANT_PREFIX}/_apis/"):
            self._json(200, {})
            return
        self.send_error(404)

def serve(port):
    ThreadingHTTPServer(("0.0.0.0", port), H).serve_forever()

if __name__ == "__main__":
    threading.Thread(target=serve, args=(8080,), daemon=True).start()
    print("mock actions service on :8080")
    threading.Event().wait()
