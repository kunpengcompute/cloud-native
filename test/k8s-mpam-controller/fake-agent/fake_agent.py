#!/usr/bin/env python3
"""
Fake MPAM interference agent for local/e2e testing.

Implements:
  - POST /v1/online-pods
  - GET  /v1/interference?node_name=<node>

Behavior:
  - Receives and stores latest online pod cgroup info per node.
  - Returns random interference reason: l3 / mb / cpu.
"""

from __future__ import annotations

import argparse
import json
import random
import threading
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any, Dict, List
from urllib.parse import parse_qs, urlparse


class Store:
    def __init__(self) -> None:
        self._lock = threading.Lock()
        self._pods_by_node: Dict[str, List[Dict[str, Any]]] = {}

    def set_pods(self, node_name: str, pods: List[Dict[str, Any]]) -> None:
        with self._lock:
            self._pods_by_node[node_name] = pods

    def get_pods(self, node_name: str) -> List[Dict[str, Any]]:
        with self._lock:
            return list(self._pods_by_node.get(node_name, []))


class Handler(BaseHTTPRequestHandler):
    store = Store()

    def _write_json(self, status: int, payload: Dict[str, Any]) -> None:
        body = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_POST(self) -> None:
        parsed = urlparse(self.path)
        if parsed.path != "/v1/online-pods":
            self._write_json(HTTPStatus.NOT_FOUND, {"error": "not found"})
            return

        try:
            length = int(self.headers.get("Content-Length", "0"))
            raw = self.rfile.read(length) if length > 0 else b"{}"
            req = json.loads(raw.decode("utf-8"))
        except Exception as exc:  # pragma: no cover
            self._write_json(HTTPStatus.BAD_REQUEST, {"accepted": False, "message": f"invalid json: {exc}"})
            return

        node_name = str(req.get("node_name", "")).strip()
        pods = req.get("pods", [])
        if not node_name:
            self._write_json(HTTPStatus.BAD_REQUEST, {"accepted": False, "message": "node_name is required"})
            return
        if not isinstance(pods, list):
            self._write_json(HTTPStatus.BAD_REQUEST, {"accepted": False, "message": "pods must be a list"})
            return

        # only store data; no real analysis.
        self.store.set_pods(node_name, pods)
        print(f"[fake-agent] receive online pods: node={node_name}, count={len(pods)}")
        for idx, pod in enumerate(pods):
            namespace = str(pod.get("namespace", ""))
            name = str(pod.get("name", ""))
            uid = str(pod.get("uid", ""))
            cgroup_path = str(pod.get("cgroup_path", ""))
            print(
                f"[fake-agent]   pod[{idx}]: {namespace}/{name} uid={uid} cgroup_path={cgroup_path}",
                flush=True,
            )
        self._write_json(HTTPStatus.OK, {"accepted": True, "message": "ok"})

    def do_GET(self) -> None:
        parsed = urlparse(self.path)
        if parsed.path != "/v1/interference":
            self._write_json(HTTPStatus.NOT_FOUND, {"error": "not found"})
            return

        query = parse_qs(parsed.query)
        node_name = (query.get("node_name", [""])[0] or "").strip()
        if not node_name:
            self._write_json(HTTPStatus.BAD_REQUEST, {"error": "node_name is required"})
            return

        pods = self.store.get_pods(node_name)
        reason = random.choice(["l3", "mb", "cpu"])

        items: List[Dict[str, Any]] = []
        # Return up to 3 random pod scores for observability.
        for pod in pods[:3]:
            uid = pod.get("uid", "")
            if not uid:
                continue
            items.append(
                {
                    "pod_uid": uid,
                    "score": round(random.uniform(0.4, 0.99), 2),
                }
            )

        resp = {
            "version": "v1",
            "node_name": node_name,
            "reason": reason,
            "ttl_seconds": 10,
            "items": items,
        }
        print(f"[fake-agent] return interference: node={node_name}, reason={reason}, items={len(items)}")
        self._write_json(HTTPStatus.OK, resp)

    # Silence default access log; keep stdout focused on payload-level logs.
    def log_message(self, fmt: str, *args: Any) -> None:  # pragma: no cover
        return


def main() -> None:
    parser = argparse.ArgumentParser(description="Fake MPAM interference agent")
    parser.add_argument("--host", default="127.0.0.1", help="listen host")
    parser.add_argument("--port", type=int, default=18080, help="listen port")
    parser.add_argument("--seed", type=int, default=42, help="random seed")
    args = parser.parse_args()

    random.seed(args.seed)
    server = ThreadingHTTPServer((args.host, args.port), Handler)
    print(f"[fake-agent] listening on http://{args.host}:{args.port}")
    server.serve_forever()


if __name__ == "__main__":
    main()
