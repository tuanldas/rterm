"""Cấu hình dùng chung của bộ test server.

Test đọc tài sản ở gốc repo (docs/protocol, product.json, bản đóng băng định danh, scripts/)
chứ không chép vào server/, để server kiểm đúng thứ mà agent Go và app Dart cũng kiểm.
"""

from pathlib import Path

import pytest


@pytest.fixture(scope="session")
def repo_root() -> Path:
    """Gốc repo; báo lỗi rõ khi test chạy ngoài repo đầy đủ (ví dụ từ bản sao chỉ có server/)."""
    root = Path(__file__).resolve().parents[2]
    if not (root / "product.json").is_file():
        pytest.fail(f"không thấy product.json ở {root}: bộ test server phải chạy trong repo đầy đủ")
    return root
