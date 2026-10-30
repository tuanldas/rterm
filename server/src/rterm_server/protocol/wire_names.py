"""Định danh trên dây mà server dùng: số proto, đường dẫn, method đồng bộ, capability, ngữ cảnh
chữ ký và mã lỗi JSON-RPC riêng.

Đây là hợp đồng đóng băng (docs/protocol/README.md): chỉ được thêm, không xoá, không đổi giá
trị. Giá trị viết thẳng và module không import mã nào khác của server, nhất là module tên sản
phẩm: agent và app đã phát hành vẫn gửi đúng các chuỗi này sau khi sản phẩm đổi tên. Test so
module này với bản đóng băng internal/protocol/testdata/wire_names.golden.json mà phía Go cũng
kiểm, nên hai bên không thể lệch nhau.

Chỉ khai báo định danh server dùng; phần riêng của agent (RPC qua SSH, biến môi trường phiên,
mã thoát) không có ở đây.
"""

from collections.abc import Mapping
from types import MappingProxyType
from typing import Final

SYNC_PROTO: Final = 1
"""Phiên bản giao thức đồng bộ agent–server. Đổi là thay đổi phá vỡ, bị cấm trong v1."""

API_PROTO: Final = 1
"""Phiên bản HTTP API, khớp API_PREFIX và trường api của GET /v1/meta."""

API_PREFIX: Final = "/v1"
"""Tiền tố mọi endpoint HTTP (trừ /health/*). Thêm endpoint không đổi tiền tố."""

AGENT_SYNC_PATH: Final = API_PREFIX + "/agent/sync"
"""Endpoint WebSocket mà agent giữ kết nối ra."""

# Method của kênh đồng bộ (JSON-RPC 2.0, mỗi khung text một thông điệp).
SYNC_METHOD_HELLO: Final = "hello"
SYNC_METHOD_AUTH: Final = "auth"
SYNC_METHOD_POLICY_APPLY: Final = "policy.apply"
SYNC_METHOD_POLICY_KEYS: Final = "policy.keys"
SYNC_METHOD_MACHINE_STATUS: Final = "machine.status"
SYNC_METHOD_AUDIT_REPORT: Final = "audit.report"
SYNC_METHOD_EMERGENCY_VERIFY: Final = "emergency.verify"

SYNC_METHODS: Final[tuple[str, ...]] = (
    SYNC_METHOD_HELLO,
    SYNC_METHOD_AUTH,
    SYNC_METHOD_POLICY_APPLY,
    SYNC_METHOD_POLICY_KEYS,
    SYNC_METHOD_MACHINE_STATUS,
    SYNC_METHOD_AUDIT_REPORT,
    SYNC_METHOD_EMERGENCY_VERIFY,
)

# Capability của kênh đồng bộ (trao đổi trong hello) và của HTTP (GET /v1/meta). Đây là tên mà
# server biết, không phải danh sách server quảng bá: mỗi tính năng tự khai capability của mình
# khi đã hiện thực.
CAP_POLICY_V1: Final = "policy.v1"
CAP_STATUS_V1: Final = "status.v1"
CAP_AUDIT_V1: Final = "audit.v1"
CAP_EMERGENCY_V1: Final = "emergency.v1"
CAP_DEVICES_V1: Final = "devices.v1"
CAP_GROUPS_V1: Final = "groups.v1"
CAP_SHARING_V1: Final = "sharing.v1"
CAP_SSH_KEYS_V1: Final = "ssh_keys.v1"

CAPABILITIES: Final[tuple[str, ...]] = (
    CAP_POLICY_V1,
    CAP_STATUS_V1,
    CAP_AUDIT_V1,
    CAP_EMERGENCY_V1,
    CAP_DEVICES_V1,
    CAP_GROUPS_V1,
    CAP_SHARING_V1,
    CAP_SSH_KEYS_V1,
)

# Ngữ cảnh chữ ký mà server ký hoặc kiểm (dữ liệu ký = ngữ cảnh, byte 0x00, thông điệp).
CONTEXT_POLICY_SNAPSHOT: Final = "policy-snapshot/1"
CONTEXT_POLICY_KEY_ROLLOVER: Final = "policy-key-rollover/1"
CONTEXT_SYNC_AUTH: Final = "sync-auth/1"
CONTEXT_ENROLL_POP: Final = "enroll-pop/1"
CONTEXT_DEVICE_KEY_POP: Final = "device-key-pop/1"

SIGNING_CONTEXTS: Final[tuple[str, ...]] = (
    CONTEXT_POLICY_SNAPSHOT,
    CONTEXT_POLICY_KEY_ROLLOVER,
    CONTEXT_SYNC_AUTH,
    CONTEXT_ENROLL_POP,
    CONTEXT_DEVICE_KEY_POP,
)

# Mã lỗi JSON-RPC riêng. -32000 là mã quá tải mà thư viện JSON-RPC trả khi có quá nhiều yêu cầu
# đang xử lý đồng thời, không kèm error.data; peer của server phải dùng đúng mã agent Go dùng.
# Bốn mã còn lại mang error.data là ErrorData {reason, retry_after}.
CODE_SERVER_BUSY: Final = -32000
CODE_UNAUTHORIZED: Final = -32001
CODE_NOT_FOUND: Final = -32002
CODE_LIMIT_EXCEEDED: Final = -32003
CODE_CONFLICT: Final = -32004

JSONRPC_ERRORS: Final[Mapping[str, int]] = MappingProxyType(
    {
        "server_busy": CODE_SERVER_BUSY,
        "unauthorized": CODE_UNAUTHORIZED,
        "not_found": CODE_NOT_FOUND,
        "limit_exceeded": CODE_LIMIT_EXCEEDED,
        "conflict": CODE_CONFLICT,
    }
)
"""Tên ổn định của từng mã lỗi riêng, đúng như bản đóng băng; chỉ đọc."""
