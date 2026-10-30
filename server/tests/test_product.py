"""Tên sản phẩm phía server: khớp product.json và chỉ được viết ở một chỗ.

Bảo đảm:
- Mỗi hằng của rterm_server.product bằng giá trị tương ứng trong product.json; tên gói
  ([project].name) và tên lệnh console trong pyproject.toml bằng server_image. Thêm khoá mới vào
  product.json mà server chưa quyết dùng hay không thì test đỏ.
- Không string literal nào trong server/src chứa tên sản phẩm (hoặc tên cũ), không phân biệt
  hoa thường, ngoài product.py, protocol/wire_names.py và chuỗi đường dẫn gói Python.
"""

import ast
import json
import re
import tomllib
from pathlib import Path
from typing import Any

import pytest

import rterm_server
from rterm_server import product

# Khoá product.json → hằng tương ứng phía server.
PRODUCT_CONSTANTS = {
    "name": "NAME",
    "server_image": "SERVER_IMAGE",
    "app_id": "APP_ID",
    "default_server_port": "DEFAULT_SERVER_PORT",
    "env_prefix": "ENV_PREFIX",
    "token_prefix": "TOKEN_PREFIX",
    "url_scheme": "URL_SCHEME",
}

# Khoá product.json mà server không dùng: chúng thuộc agent (tệp chạy, cổng SSH, thư mục trạng
# thái) hoặc chỉ là chú thích. legacy_names được bộ quét literal bên dưới đọc thẳng từ đây.
NOT_USED_BY_SERVER = frozenset(
    {"$comment", "agent_binary", "default_agent_port", "state_dir_name", "legacy_names"}
)

# Tệp được phép chứa tên sản phẩm, tính từ thư mục gói: nơi khai báo duy nhất, và định danh trên
# dây (đóng băng qua lần đổi tên nên không được suy ra từ product).
EXEMPT_FILES = frozenset({"product.py", "protocol/wire_names.py"})


@pytest.fixture(scope="module")
def product_json(repo_root: Path) -> dict[str, Any]:
    return json.loads((repo_root / "product.json").read_bytes())


def test_hang_san_pham_khop_product_json(product_json: dict[str, Any], repo_root: Path):
    unclassified = set(product_json) - set(PRODUCT_CONSTANTS) - NOT_USED_BY_SERVER
    assert not unclassified, (
        f"product.json có khoá mới {sorted(unclassified)}: thêm hằng vào product.py hoặc ghi vào"
        " NOT_USED_BY_SERVER nếu server không dùng"
    )
    mismatches = [
        f"product.{const} = {getattr(product, const)!r},"
        f" product.json[{key!r}] = {product_json[key]!r}"
        for key, const in PRODUCT_CONSTANTS.items()
        if getattr(product, const) != product_json[key]
    ]
    assert not mismatches, "hằng sản phẩm lệch product.json:\n" + "\n".join(mismatches)

    pyproject = tomllib.loads((repo_root / "server" / "pyproject.toml").read_text(encoding="utf-8"))
    image = product_json["server_image"]
    assert pyproject["project"]["name"] == image, "[project].name phải bằng server_image"
    assert list(pyproject["project"]["scripts"]) == [image], (
        "lệnh console duy nhất phải mang tên server_image (tên lệnh trong image)"
    )


def find_product_name_literals(
    source: str, names: list[str], package: str
) -> list[tuple[int, str]]:
    """String literal (kể cả docstring, phần cố định của f-string, bytes) chứa một trong các tên.

    Chuỗi đúng dạng đường dẫn gói (như "<gói>.migrations" cho importlib.resources) được miễn vì
    đó là tên module Python, không phải chuỗi chương trình tự đặt ra.
    """
    package_path = re.compile(rf"{re.escape(package)}(\.[A-Za-z_][A-Za-z0-9_]*)*")
    lowered = [n.lower() for n in names if n]
    found = []
    for node in ast.walk(ast.parse(source)):
        if not isinstance(node, ast.Constant) or not isinstance(node.value, str | bytes):
            continue
        text = node.value.decode("latin-1") if isinstance(node.value, bytes) else node.value
        if package_path.fullmatch(text):
            continue
        if any(n in text.lower() for n in lowered):
            found.append((node.lineno, text))
    return found


@pytest.mark.parametrize(
    ("source", "expected"),
    [
        pytest.param('a = "sanpham"\n', 1, id="literal thường"),
        pytest.param('a = "SANPHAM_SERVER_LISTEN"\n', 1, id="viết hoa vẫn bị bắt"),
        pytest.param('a = f"{x}-sanpham"\n', 1, id="phần cố định của f-string"),
        pytest.param('a = b"sanpham"\n', 1, id="bytes"),
        pytest.param('"""Mô tả sanpham."""\n', 1, id="docstring"),
        pytest.param('a = "ten-cu"\n', 1, id="tên cũ"),
        pytest.param('f("sanpham_server.migrations")\n', 0, id="đường dẫn gói được miễn"),
        pytest.param('a = "lỗi ở sanpham_server.cli"\n', 1, id="đường dẫn gói trong câu"),
        pytest.param("# sanpham\na = 1\n", 0, id="chú thích không phải literal"),
        pytest.param('a = "khac"\n', 0, id="không chứa tên"),
    ],
)
def test_bo_quet_literal_bat_dung_vi_pham(source: str, expected: int):
    found = find_product_name_literals(source, ["sanpham", "ten-cu"], "sanpham_server")
    assert len(found) == expected, f"tìm thấy {found}, cần {expected} vi phạm"


def test_ten_san_pham_chi_khai_bao_mot_cho(product_json: dict[str, Any], repo_root: Path):
    src_dir = repo_root / "server" / "src"
    exempt = {f"{rterm_server.__name__}/{name}" for name in EXEMPT_FILES}
    names = [product_json["name"], *product_json["legacy_names"]]
    scanned = 0
    violations = []
    for path in sorted(src_dir.rglob("*.py")):
        rel = path.relative_to(src_dir).as_posix()
        if rel in exempt:
            continue
        scanned += 1
        source = path.read_text(encoding="utf-8")
        for line, text in find_product_name_literals(source, names, rterm_server.__name__):
            violations.append(f"{rel}:{line}: {text!r}")
    assert scanned, f"không quét được tệp nào trong {src_dir}"
    assert not violations, (
        "literal chứa tên sản phẩm; dùng rterm_server.product (hoặc protocol.wire_names nếu là"
        " định danh trên dây):\n" + "\n".join(violations)
    )
