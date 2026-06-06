#!/usr/bin/env python3
"""Generate JSON fixtures from Python agentscope v2 for Go validation."""

import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path("/c/GitHub/agentscope/src")))

from agentscope.message import Msg, TextBlock, DataBlock, ThinkingBlock, HintBlock
from agentscope.message import ToolCallBlock, ToolResultBlock
from agentscope.message import Base64Source, URLSource


def main() -> int:
    fixtures_dir = Path(__file__).parent / "fixtures"
    fixtures_dir.mkdir(parents=True, exist_ok=True)

    msg = Msg(
        id="msg-py-001",
        role="assistant",
        name="PyAgent",
        content=[
            TextBlock(text="Hello from Python"),
            DataBlock(
                source=URLSource(url="http://example.com/img.png", media_type="image/png")
            ),
            DataBlock(
                source=Base64Source(data="iVBORw0KGgo=", media_type="audio/mp3")
            ),
            ThinkingBlock(thinking="Thinking..."),
            HintBlock(hint="hint text"),
            ToolCallBlock(id="tc1", name="calc", input='{"x": 1}'),
            ToolResultBlock(
                id="tc1",
                name="calc",
                output=[TextBlock(text="result")],
                state="success",
            ),
        ],
        metadata={"key": "value"},
        created_at="2024-01-01T00:00:00Z",
    )

    data = msg.model_dump_json(indent=2)
    out_path = fixtures_dir / "py_msg.json"
    out_path.write_text(data, encoding="utf-8")
    print(f"Python fixture generated: {out_path}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
