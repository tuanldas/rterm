"""Lớp gốc cho mọi thông điệp JSON của hợp đồng."""

from pydantic import BaseModel, ConfigDict, JsonValue


class WireMessage(BaseModel):
    """Thông điệp JSON trên dây, đọc và ghi theo quy tắc chung của hợp đồng.

    - Trường lạ bị bỏ qua: server phải đọc được thông điệp của agent và app bản mới hơn.
    - Kiểu chặt: không đổi "1" thành 1 hay 1.0 thành 1, để server hiểu thông điệp đúng như
      agent Go và app Dart hiểu.
    - Danh sách tuỳ chọn mặc định rỗng, vì trên dây "vắng" nghĩa là "rỗng"; người nhận không
      phải phân biệt hai trường hợp.
    """

    model_config = ConfigDict(extra="ignore", strict=True)

    def to_wire(self) -> dict[str, JsonValue]:
        """Dạng chuẩn của người gửi: trường tuỳ chọn không có giá trị (None, danh sách rỗng) vắng
        mặt, còn trường bắt buộc luôn có mặt kể cả khi rỗng (như grants của snapshot)."""
        return self.model_dump(mode="json", exclude_defaults=True)
