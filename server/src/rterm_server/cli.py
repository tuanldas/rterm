"""Dòng lệnh của server. Hiện chỉ có `version`; các lệnh chạy server thêm sau."""

import argparse

from rterm_server import product
from rterm_server._version import VERSION


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog=product.SERVER_IMAGE,
        description="Server tài khoản: danh tính và quyền truy cập máy.",
    )
    commands = parser.add_subparsers(dest="command", metavar="<lệnh>", required=True)
    commands.add_parser("version", help="in phiên bản rồi thoát")
    return parser


def main(argv: list[str] | None = None) -> int:
    """Chạy một lệnh và trả mã thoát: 0 thành công, 2 dùng sai."""
    parser = _build_parser()
    try:
        args = parser.parse_args(argv)
    except SystemExit as exc:
        # argparse đã in hướng dẫn hoặc lỗi ra stderr rồi gọi exit; đổi thành giá trị trả về
        # để main gọi được từ test mà không kết thúc tiến trình.
        return exc.code if isinstance(exc.code, int) else 2
    if args.command == "version":
        print(VERSION)
        return 0
    # Lệnh con là bắt buộc và argparse chỉ nhận lệnh đã đăng ký, nên tới đây là lỗi lập trình:
    # đăng ký lệnh mới mà quên xử lý.
    raise AssertionError(f"lệnh {args.command!r} đã đăng ký nhưng chưa được xử lý")
