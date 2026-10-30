"""Định danh sản phẩm phía server: nơi duy nhất trong mã Python được viết tên sản phẩm.

Nguồn trung lập là product.json ở gốc repo; test chéo so từng hằng ở đây với tệp đó, nên đổi
tên sản phẩm là sửa product.json rồi sửa đúng chỗ test chỉ ra. Mã khác của server lấy giá trị
từ module này, không viết lại chuỗi (test quét string literal chặn việc đó).

Định danh trên dây (method, capability, ngữ cảnh chữ ký) KHÔNG lấy từ đây mà ở
protocol.wire_names: chúng phải sống qua lần đổi tên vì agent và app đã phát hành vẫn chạy.
"""

from typing import Final

NAME: Final = "rterm"
"""Tên sản phẩm hiển thị cho người dùng."""

SERVER_IMAGE: Final = "rterm-server"
"""Tên image Docker của server, cũng là tên lệnh trong image."""

APP_ID: Final = "com.tuanldas.rterm"
"""Định danh gói của app Android."""

DEFAULT_SERVER_PORT: Final = 7443
"""Cổng HTTPS mặc định của server; đổi được bằng cấu hình."""

ENV_PREFIX: Final = "RTERM_"
"""Tiền tố biến môi trường cấu hình của server."""

TOKEN_PREFIX: Final = "rt_"  # noqa: S105 - tiền tố công khai để công cụ quét nhận ra token bị lộ
"""Tiền tố token do server cấp; client coi token là chuỗi không trong suốt."""

URL_SCHEME: Final = "rterm"
"""Scheme của deep link (thiết lập server, lời mời)."""
