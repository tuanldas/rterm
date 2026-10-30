/// Lỗi của HTTP API (`docs/protocol/http-api.md`, `$defs/ErrorResponse` trong
/// `docs/protocol/schema/http-api.schema.json`).
library;

import 'json_reader.dart';

final _codePattern = RegExp(r'^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$');

/// Một lỗi do server trả về. Tập mã mở: app gặp mã lạ hiện thông báo chung.
final class ApiError implements Exception {
  const ApiError({required this.code, required this.message, this.details});

  factory ApiError.fromJson(Object? json, [String path = 'APIError']) {
    final r = JsonReader(json, path);
    return ApiError(
      code: r.value('code', (v, p) => readString(v, p, pattern: _codePattern)),
      message: r.value('message', readString),
      details: r.optional('details', _readDetails),
    );
  }

  /// Mã ổn định dạng `<miền>.<lý_do>`, để app ánh xạ sang thông báo trong ARB.
  final String code;

  /// Mô tả cho người vận hành, không hiển thị thẳng cho người dùng cuối.
  final String message;

  /// Dữ liệu riêng của từng mã lỗi (ví dụ `interval` của `enroll.slow_down`);
  /// giữ nguyên như server gửi.
  final Map<String, Object?>? details;

  static Map<String, Object?> _readDetails(Object? value, String path) =>
      Map.unmodifiable(readObject(value, path));

  Map<String, Object?> toJson() => {
    'code': code,
    'message': message,
    if (details != null) 'details': details,
  };

  @override
  String toString() => 'ApiError($code): $message';
}

/// Thân JSON của mọi phản hồi lỗi HTTP.
final class ErrorResponse {
  const ErrorResponse({required this.error});

  factory ErrorResponse.fromJson(
    Object? json, [
    String path = 'ErrorResponse',
  ]) {
    final r = JsonReader(json, path);
    return ErrorResponse(error: r.value('error', ApiError.fromJson));
  }

  final ApiError error;

  Map<String, Object?> toJson() => {'error': error.toJson()};
}
