"""Thân yêu cầu và phản hồi JSON của HTTP API /v1 (docs/protocol/http-api.md,
schema/http-api.schema.json).

Tên lớp trùng tên $defs của schema. Model chỉ giữ hình dạng; ràng buộc giá trị do schema mô tả
và server kiểm ở nơi nhận. Thời gian là Unix mili-giây UTC.
"""

from pydantic import JsonValue

from rterm_server.protocol.sync import LocalRevocation, PolicyKey
from rterm_server.protocol.wire_message import WireMessage


class Meta(WireMessage):
    """Phản hồi của GET /v1/meta (không cần đăng nhập)."""

    server_version: str
    api: int
    capabilities: list[str] = []
    setup_required: bool


class APIError(WireMessage):
    """Lỗi HTTP: code dạng <miền>.<lý_do> ổn định cho client; message cho người vận hành."""

    code: str
    message: str
    # Nội dung tuỳ từng mã lỗi (như totp_enrollment_token, interval) nên là JSON tự do.
    details: dict[str, JsonValue] = {}


class ErrorResponse(WireMessage):
    """Thân của mọi phản hồi lỗi HTTP."""

    error: APIError


class SetupRequest(WireMessage):
    """Thân của POST /v1/setup."""

    setup_token: str
    email: str
    password: str


class SetupResult(WireMessage):
    """Phản hồi của POST /v1/setup: token tạm chỉ dùng để đăng ký TOTP."""

    totp_enrollment_token: str


class LoginRequest(WireMessage):
    """Thân của POST /v1/auth/login: đúng một trong totp hoặc recovery_code."""

    email: str
    password: str
    totp: str | None = None
    recovery_code: str | None = None
    device_name: str


class TokenPair(WireMessage):
    """Cặp token sau khi xác thực; token là chuỗi không trong suốt."""

    access_token: str
    refresh_token: str
    access_expires_at: int


class RefreshRequest(WireMessage):
    """Thân của POST /v1/auth/refresh."""

    refresh_token: str


class StepUpRequest(WireMessage):
    """Thân của POST /v1/auth/step-up."""

    totp: str


class TOTPEnrollment(WireMessage):
    """Phản hồi của POST /v1/auth/totp/enroll."""

    secret: str
    otpauth_uri: str


class TOTPConfirmRequest(WireMessage):
    """Thân của POST /v1/auth/totp/confirm."""

    code: str


class TOTPConfirmResult(TokenPair):
    """TokenPair cộng mã khôi phục, chỉ hiện đúng một lần."""

    recovery_codes: list[str]


class Me(WireMessage):
    """Phản hồi của GET /v1/me."""

    id: str
    email: str
    totp_enabled: bool
    soft_locked_until: int | None = None
    created_at: int


class ChangePasswordRequest(WireMessage):
    """Thân của POST /v1/me/password."""

    current_password: str
    new_password: str


class Device(WireMessage):
    """Một thiết bị (phiên đăng nhập có tên) của tài khoản."""

    id: str
    name: str
    created_at: int
    last_used_at: int
    current: bool


class DeviceList(WireMessage):
    """Phản hồi của GET /v1/me/devices."""

    devices: list[Device] = []


class Alert(WireMessage):
    """Cảnh báo hiện trong app; action là tập mở."""

    id: str
    action: str
    ts: int
    machine_id: str | None = None
    target: str | None = None


class AlertList(WireMessage):
    """Phản hồi của GET /v1/me/alerts."""

    alerts: list[Alert] = []


class KeyChallenge(WireMessage):
    """Phản hồi của POST /v1/me/keys/challenge: nonce dùng một lần."""

    nonce: str


class RegisterKeyRequest(WireMessage):
    """Thân của POST /v1/me/keys; khoá thiết bị phải có nonce và sig (device-key-pop/1)."""

    kind: str
    name: str
    public_key: str
    nonce: str | None = None
    sig: str | None = None
    hardware_backed: bool | None = None
    security_level: str | None = None


class AccountKey(WireMessage):
    """Một khoá công khai của tài khoản."""

    id: str
    kind: str
    name: str
    fp: str
    public_key: str | None = None
    hardware_backed: bool | None = None
    created_at: int
    device_id: str | None = None


class RegisterKeyResult(WireMessage):
    """Phản hồi của POST /v1/me/keys."""

    key: AccountKey
    affected_machine_ids: list[str] = []


class KeyList(WireMessage):
    """Phản hồi của GET /v1/me/keys."""

    keys: list[AccountKey] = []


class AffectedMachines(WireMessage):
    """Các máy có snapshot quyền thay đổi sau một thao tác; vắng nghĩa là không máy nào."""

    affected_machine_ids: list[str] = []


class EnrollStartRequest(WireMessage):
    """Thân của POST /v1/enroll/start do agent gửi."""

    host_key: str
    hostname: str
    os: str
    arch: str
    os_user: str
    agent_version: str
    allowed_cidrs: list[str] = []


class EnrollStartResult(WireMessage):
    """Phản hồi của POST /v1/enroll/start (RFC 8628 §3.2)."""

    device_code: str
    user_code: str
    verification_uri: str
    expires_in: int
    interval: int


class EnrollPollRequest(WireMessage):
    """Thân của POST /v1/enroll/poll: sig chứng minh agent giữ host key (enroll-pop/1)."""

    device_code: str
    sig: str


class EnrollPollResult(WireMessage):
    """Phản hồi khi máy đã được duyệt: mã máy và các khoá ký chính sách agent sẽ tin."""

    machine_id: str
    policy_keys: list[PolicyKey]


class EnrollInfo(WireMessage):
    """Phản hồi của GET /v1/enroll/{user_code} cho người đang duyệt."""

    user_code: str
    hostname: str
    os: str
    arch: str
    os_user: str
    requested_ip: str | None = None
    allowed_cidrs: list[str] = []
    host_key_fp: str
    requested_at: int
    expires_at: int


class EnrollApproveRequest(WireMessage):
    """Thân của POST /v1/enroll/{user_code}/approve."""

    name: str | None = None


class Machine(WireMessage):
    """Một máy mà tài khoản có quyền; policy_state và role là tập mở."""

    id: str
    name: str
    hostname: str
    os: str
    arch: str
    os_user: str
    addresses: list[str] = []
    host_key_fp: str
    online: bool
    session_count: int
    last_seen_at: int | None = None
    policy_state: str
    agent_version: str | None = None
    agent_capabilities: list[str] = []
    allowed_cidrs: list[str] = []
    role: str
    local_revocations: list[LocalRevocation] = []


class MachineList(WireMessage):
    """Phản hồi của GET /v1/machines."""

    machines: list[Machine] = []


class MachinePatch(WireMessage):
    """Thân của PATCH /v1/machines/{id}."""

    name: str | None = None
