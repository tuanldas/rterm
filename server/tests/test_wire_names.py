"""Định danh trên dây phía server khớp bản đóng băng dùng chung với agent Go.

Bảo đảm:
- Mọi định danh server khai báo có trong internal/protocol/testdata/wire_names.golden.json với
  đúng giá trị (đổi tên, xoá hay thêm mà không đóng băng đều đỏ).
- Mỗi mục của bản đóng băng hoặc được server khai báo, hoặc được ghi rõ là không dùng: thêm
  định danh mới vào registry mà quên phía server thì test này đỏ.
- wire_names.py không phụ thuộc mã nào khác của server, nhất là module tên sản phẩm.
"""

import ast
import inspect
import json
from pathlib import Path
from typing import Any

import pytest

from rterm_server.protocol import wire_names

# Khoá của bản đóng băng chỉ thuộc kênh SSH và RPC của agent, server không dùng.
AGENT_ONLY_KEYS = frozenset(
    {
        "session_env_vars",
        "session_env_new",
        "exit_session_not_found",
        "stderr_session_not_found",
        "subsystem_agent_rpc",
        "rpc_methods",
        "rpc_notifications",
        "event_topics",
    }
)

# Phần tử của danh sách đóng băng mà server không dùng, kèm lý do.
NOT_USED_BY_SERVER: dict[str, frozenset[str]] = {
    # Capability của RPC agent: server chỉ lưu và chuyển nguyên văn agent_capabilities cho app.
    "capabilities": frozenset({"session.v1", "events.v1"}),
    # Ký bộ khoá và manifest phát hành: làm ngoại tuyến, không qua server.
    "signing_contexts": frozenset({"release-keyset/1", "release-manifest/1"}),
}

# proto của RPC agent không thuộc server.
NOT_USED_PROTOS = frozenset({"rpc"})

# Thư viện chuẩn duy nhất wire_names.py được import: chỉ để khai báo kiểu và map chỉ đọc.
ALLOWED_IMPORTS = frozenset({"collections.abc", "types", "typing"})


@pytest.fixture(scope="module")
def golden(repo_root: Path) -> dict[str, Any]:
    path = repo_root / "internal" / "protocol" / "testdata" / "wire_names.golden.json"
    return json.loads(path.read_bytes())


def _diff(name: str, frozen: set[str], declared: tuple[str, ...]) -> list[str]:
    problems = [f"{name}: {v!r} khai báo hai lần" for v in set(declared) if declared.count(v) > 1]
    problems += [
        f"{name}: {v!r} đã đóng băng nhưng thiếu hoặc bị đổi trong wire_names.py"
        for v in sorted(frozen - set(declared))
    ]
    problems += [
        f"{name}: {v!r} chưa có trong bản đóng băng (thêm vào đó và docs/protocol/README.md"
        " nếu là chủ ý)"
        for v in sorted(set(declared) - frozen)
    ]
    return problems


def test_dinh_danh_tren_day_khop_ban_dong_bang(golden: dict[str, Any]):
    handled = {"protos", "api_prefix", "agent_sync_path", "sync_methods"}
    handled |= {"capabilities", "signing_contexts", "jsonrpc_error_codes"}
    unclassified = set(golden) - handled - AGENT_ONLY_KEYS
    assert not unclassified, (
        f"bản đóng băng có mục mới {sorted(unclassified)}: khai báo ở wire_names.py hoặc ghi vào"
        " AGENT_ONLY_KEYS nếu server không dùng"
    )

    protos = golden["protos"]
    assert set(protos) - NOT_USED_PROTOS == {"sync", "api"}, (
        f"proto lạ trong bản đóng băng: {protos}"
    )
    assert wire_names.SYNC_PROTO == protos["sync"], "đổi proto đồng bộ là thay đổi phá vỡ"
    assert wire_names.API_PROTO == protos["api"], "đổi proto HTTP là thay đổi phá vỡ"
    assert wire_names.API_PREFIX == golden["api_prefix"]
    assert wire_names.AGENT_SYNC_PATH == golden["agent_sync_path"]

    problems = []
    for key, declared in (
        ("sync_methods", wire_names.SYNC_METHODS),
        ("capabilities", wire_names.CAPABILITIES),
        ("signing_contexts", wire_names.SIGNING_CONTEXTS),
    ):
        not_used = NOT_USED_BY_SERVER.get(key, frozenset())
        stale = not_used - set(golden[key])
        problems += [
            f"{key}: {v!r} ghi là không dùng nhưng không có trong bản đóng băng" for v in stale
        ]
        problems += _diff(key, set(golden[key]) - not_used, declared)
    assert not problems, "\n".join(problems)

    assert dict(wire_names.JSONRPC_ERRORS) == golden["jsonrpc_error_codes"], (
        "mã lỗi JSON-RPC riêng lệch bản đóng băng"
    )


def test_dinh_danh_tren_day_khong_phu_thuoc_ten_san_pham():
    tree = ast.parse(inspect.getsource(wire_names))
    imported = []
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            imported += [alias.name for alias in node.names]
        elif isinstance(node, ast.ImportFrom):
            imported.append("." * node.level + (node.module or ""))
    assert imported, "không thấy import nào: bộ phân tích đọc sai tệp"
    outside = sorted(set(imported) - ALLOWED_IMPORTS)
    assert not outside, (
        f"wire_names.py import {outside}: định danh trên dây phải viết thẳng, không suy ra từ mã"
        " khác (nhất là tên sản phẩm) để sống qua lần đổi tên"
    )
