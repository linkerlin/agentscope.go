#!/usr/bin/env python3
"""Validate Go-generated JSON fixtures against Python agentscope v2 Pydantic models."""

import json
import sys
from pathlib import Path

# Add the Python agentscope source to the path
sys.path.insert(0, str(Path("/c/GitHub/agentscope/src")))

from agentscope.message import Msg


def validate_msg(path: str) -> bool:
    with open(path, "r", encoding="utf-8") as f:
        data = json.load(f)

    print(f"Validating {path} ...")
    try:
        msg = Msg.model_validate(data)
        print(f"  -> OK: {len(msg.content)} block(s)")
        for i, block in enumerate(msg.content):
            print(f"     block {i}: type={block.type}")
        return True
    except Exception as e:
        print(f"  -> FAIL: {e}")
        return False


def main() -> int:
    fixtures_dir = Path(__file__).parent / "fixtures"
    ok = True

    go_msg = fixtures_dir / "go_msg.json"
    if go_msg.exists():
        if not validate_msg(str(go_msg)):
            ok = False
    else:
        print(f"Missing fixture: {go_msg}")
        ok = False

    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
