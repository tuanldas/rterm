"""Phiên bản của bản build server.

Trong repo luôn là "dev". Lúc dựng image, Dockerfile ghi đè tệp này bằng tag phát hành, nên
lệnh `version` in đúng bản đang chạy mà không phải đọc metadata của gói đã cài.
"""

from typing import Final

VERSION: Final = "dev"
