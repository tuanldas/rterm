"""Thông điệp của kênh đồng bộ agent–server (docs/protocol/sync.md, schema/sync.schema.json).

Tên lớp trùng tên $defs của schema. Model chỉ giữ hình dạng (trường, kiểu, bắt buộc hay
không); ràng buộc giá trị (mẫu mã, độ dài, const) do schema mô tả và server kiểm ở nơi nhận.
Thời gian là Unix mili-giây UTC.
"""

import base64

from rterm_server.protocol.wire_message import WireMessage


class Applied(WireMessage):
    """Phiên bản snapshot quyền agent đang áp dụng, so theo thứ tự từ điển (epoch, rev)."""

    epoch: int
    rev: int


class SyncHello(WireMessage):
    """Tham số của hello (agent → server), yêu cầu đầu tiên của kênh."""

    proto: int
    machine_id: str
    agent_version: str
    capabilities: list[str] = []
    applied: Applied


class SyncHelloResult(WireMessage):
    """Kết quả của hello: nonce 16 byte (base64) để agent ký trong auth."""

    proto: int
    server_version: str
    nonce: str
    capabilities: list[str] = []


class SyncAuth(WireMessage):
    """Tham số của auth: chữ ký host key trên thử thách, ngữ cảnh sync-auth/1."""

    sig: str


class GrantKey(WireMessage):
    """Khoá đăng nhập của một tài khoản: vân tay và dòng authorized_keys chuẩn hoá."""

    fp: str
    public_key: str


class Grant(WireMessage):
    """Quyền của một tài khoản trên máy. Vai trò lạ phải được coi là member."""

    account_id: str
    role: str
    keys: list[GrantKey] = []


class Emergency(WireMessage):
    """Trạng thái đăng nhập khẩn cấp của máy."""

    enabled: bool
    account_ids: list[str] = []


class PolicySnapshot(WireMessage):
    """Toàn bộ quyền của một máy. grants luôn có mặt: rỗng là "không ai được vào"."""

    proto: int
    machine_id: str
    os_user: str
    epoch: int
    rev: int
    issued_at: int
    full: bool
    # Bắt buộc, không có mặc định: snapshot không quyền nào vẫn phải phát "grants": [].
    grants: list[Grant]
    emergency: Emergency
    clear_local_revocations: list[str] = []


def encode_payload(raw: bytes) -> str:
    """Mã hoá đúng các byte JSON đã ký thành payload (base64 chuẩn có đệm)."""
    return base64.b64encode(raw).decode("ascii")


class SignedEnvelope(WireMessage):
    """Tham số của policy.apply: payload là base64 của đúng các byte JSON đã ký.

    Chữ ký (ngữ cảnh policy-snapshot/1) phủ lên các byte đó chứ không phủ lên JSON đã phân
    tích, nên hai bên không phải thống nhất cách chuẩn hoá JSON.
    """

    payload: str
    sig: str
    key_id: str

    def payload_bytes(self) -> bytes:
        """Giải payload ra đúng các byte đã ký; kiểm chữ ký trên chúng trước khi phân tích JSON."""
        try:
            # validate=True từ chối ký tự ngoài bảng chuẩn (kể cả bảng URL-safe) thay vì bỏ qua;
            # thiếu đệm cũng lỗi. binascii.Error là một ValueError.
            return base64.b64decode(self.payload, validate=True)
        except ValueError as exc:
            raise ValueError(f"payload không phải base64 chuẩn có đệm: {exc}") from exc


class PolicyKey(WireMessage):
    """Khoá công khai ký snapshot quyền mà agent tin."""

    key_id: str
    public_key: str


class KeyEndorsement(WireMessage):
    """Tham số của policy.keys: khoá đang tin chứng thực khoá ký mới (policy-key-rollover/1)."""

    key_id: str
    public_key: str
    not_before: int
    signer_key_id: str
    sig: str


class Reject(WireMessage):
    """Lần agent từ chối snapshot gần nhất; reason là tập mở."""

    epoch: int
    rev: int
    reason: str
    at: int


class LocalRevocation(WireMessage):
    """Thu hồi đặt ngay trên máy, theo khoá (key_fp) hoặc theo tài khoản (account_id)."""

    id: str
    key_fp: str | None = None
    account_id: str | None = None
    expires_at: int


class MachineStatus(WireMessage):
    """Tham số của notification machine.status (agent → server)."""

    session_count: int
    session_accounts: list[str] = []
    addresses: list[str] = []
    holder_versions: list[str] = []
    agent_version: str
    os: str
    arch: str
    applied: Applied
    last_reject: Reject | None = None
    local_revocations: list[LocalRevocation] = []


class AuditEvent(WireMessage):
    """Sự kiện kiểm toán do máy báo; server không tin tuyệt đối."""

    event_id: str
    ts: int
    action: str
    account_id: str | None = None
    target: str | None = None
    result: str | None = None
    ip: str | None = None
    count: int | None = None
    meta: dict[str, str] = {}


class AuditReport(WireMessage):
    """Tham số của audit.report: một lô sự kiện, ít nhất một."""

    events: list[AuditEvent]


class AuditAck(WireMessage):
    """Kết quả của audit.report: mọi sự kiện server đã xử lý xong."""

    acked: list[str]


class EmergencyVerifyParams(WireMessage):
    """Tham số của emergency.verify. Không có trường mật khẩu tài khoản, và không được có."""

    email: str
    emergency_secret: str
    totp: str
    os_user: str
    remote_ip: str


class EmergencyVerifyResult(WireMessage):
    """Kết quả khi xác minh khẩn cấp thành công."""

    account_id: str
