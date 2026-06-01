from __future__ import annotations

import argparse
import json
import os
import re
from pathlib import Path

from agent_runtime.mimo import _anthropic_base_url, _post_json


ALLOWED_ENV = {
    "ANTHROPIC_BASE_URL",
    "ANTHROPIC_AUTH_TOKEN",
    "ANTHROPIC_MODEL",
    "ANTHROPIC_DEFAULT_SONNET_MODEL",
    "ANTHROPIC_DEFAULT_OPUS_MODEL",
    "ANTHROPIC_DEFAULT_HAIKU_MODEL",
    "MIMO_DEFAULT_MODEL",
    "MIMO_VISION_MODEL",
}


def main() -> int:
    parser = argparse.ArgumentParser(description="Smoke test Mimo Anthropic-compatible API without printing secrets.")
    parser.add_argument("--config", default=r"C:\Users\10495\Desktop\mimo.txt")
    parser.add_argument("--model", default="")
    args = parser.parse_args()

    load_env_file(Path(args.config))
    model = args.model or os.getenv("MIMO_DEFAULT_MODEL") or os.getenv("ANTHROPIC_MODEL") or "mimo-v2.5-pro"
    token = os.getenv("ANTHROPIC_AUTH_TOKEN", "").strip()
    if not token:
        raise SystemExit("ANTHROPIC_AUTH_TOKEN is not configured")
    body = {
        "model": model,
        "max_tokens": 120,
        "temperature": 0,
        "messages": [{"role": "user", "content": '只回复 JSON: {"status":"ok"}'}],
    }
    raw = _post_json(_anthropic_base_url(), token, body)
    content = raw.get("content") or []
    first_text = ""
    if content and isinstance(content[0], dict):
        first_text = str(content[0].get("text") or "")
    summary = {
        "http_ok": True,
        "model": model,
        "top_keys": list(raw.keys()),
        "content_type": type(content).__name__,
        "content_len": len(content),
        "stop_reason": raw.get("stop_reason"),
        "usage": raw.get("usage"),
        "first_text_prefix_ascii": first_text[:200].encode("unicode_escape").decode("ascii"),
    }
    print(json.dumps(summary, ensure_ascii=True, indent=2))
    return 0


def load_env_file(path: Path) -> None:
    if not path.exists():
        raise SystemExit(f"Mimo config file not found: {path}")
    pattern_ps = re.compile(r'^\$env:([A-Za-z_][A-Za-z0-9_]*)\s*=\s*"(.*)"\s*$')
    pattern_kv = re.compile(r'^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*"?(.*?)"?\s*$')
    for line in path.read_text(encoding="utf-8-sig").splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        match = pattern_ps.match(line) or pattern_kv.match(line)
        if match and match.group(1) in ALLOWED_ENV:
            os.environ[match.group(1)] = match.group(2)


if __name__ == "__main__":
    raise SystemExit(main())
