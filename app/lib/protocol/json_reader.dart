/// Đọc JSON nhận qua dây theo hợp đồng ở `docs/protocol`.
///
/// Dữ liệu từ agent và server được kiểm kiểu và ràng buộc của schema ngay ở
/// biên, để dữ liệu hỏng dừng ở đây thay vì đi sâu vào app. Quy tắc tương thích
/// tiến của hợp đồng nằm ở đây cho mọi model: khoá lạ bị bỏ qua vì bộ đọc chỉ
/// hỏi khoá nó biết, trường danh sách vắng mặt nghĩa là rỗng.
library;

/// JSON nhận được không khớp hợp đồng giao thức: sai kiểu, thiếu trường bắt
/// buộc hoặc trái ràng buộc của schema. Thông báo nêu đường dẫn tới trường hỏng.
class ProtocolFormatException extends FormatException {
  const ProtocolFormatException(super.message);
}

/// Hàm đọc một giá trị JSON tại [path] (dùng cho thông báo lỗi).
typedef ValueReader<T> = T Function(Object? value, String path);

/// Một đối tượng JSON đang được đọc. [path] là vị trí của nó trong thông điệp,
/// ví dụ `SessionListResult.sessions[1]`.
final class JsonReader {
  /// Ném [ProtocolFormatException] nếu [value] không phải đối tượng JSON.
  JsonReader(Object? value, this.path) : _map = readObject(value, path);

  final String path;
  final Map<String, Object?> _map;

  /// Trường bắt buộc.
  T value<T>(String key, ValueReader<T> read) {
    if (!_map.containsKey(key)) {
      throw ProtocolFormatException('$path: thiếu trường bắt buộc "$key"');
    }
    return read(_map[key], '$path.$key');
  }

  /// Trường tuỳ chọn; vắng mặt trả `null`. Có mặt với giá trị `null` vẫn là
  /// lỗi vì hợp đồng không dùng `null`: trường không có giá trị thì vắng hẳn.
  T? optional<T extends Object>(String key, ValueReader<T> read) =>
      _map.containsKey(key) ? read(_map[key], '$path.$key') : null;

  /// Trường danh sách. Vắng mặt nghĩa là rỗng, trừ khi [required].
  List<T> list<T>(
    String key,
    ValueReader<T> readItem, {
    bool required = false,
    int minItems = 0,
    bool uniqueItems = false,
  }) {
    final itemsPath = '$path.$key';
    final List<Object?> raw;
    if (_map.containsKey(key)) {
      final value = _map[key];
      if (value is! List<Object?>) {
        throw ProtocolFormatException(
          '$itemsPath: cần mảng, nhận ${describeJson(value)}',
        );
      }
      raw = value;
    } else if (required) {
      throw ProtocolFormatException('$path: thiếu trường bắt buộc "$key"');
    } else {
      raw = const [];
    }
    if (raw.length < minItems) {
      throw ProtocolFormatException(
        '$itemsPath: cần ít nhất $minItems phần tử, có ${raw.length}',
      );
    }
    final items = [
      for (var i = 0; i < raw.length; i++) readItem(raw[i], '$itemsPath[$i]'),
    ];
    if (uniqueItems && items.toSet().length != items.length) {
      throw ProtocolFormatException('$itemsPath: có phần tử lặp lại');
    }
    return List.unmodifiable(items);
  }
}

/// Đọc một đối tượng JSON. Giữ nguyên nội dung để ghi lại được; dùng cho
/// trường có hình dạng tự do như `data` của sự kiện lạ hay `details` của lỗi.
Map<String, Object?> readObject(Object? value, String path) {
  if (value is Map<String, Object?>) {
    return value;
  }
  throw ProtocolFormatException(
    '$path: cần đối tượng JSON, nhận ${describeJson(value)}',
  );
}

/// Đọc một chuỗi. Độ dài tính theo ký tự Unicode như JSON Schema, không theo
/// đơn vị UTF-16 của Dart.
String readString(
  Object? value,
  String path, {
  int minLength = 0,
  int? maxLength,
  RegExp? pattern,
}) {
  if (value is! String) {
    throw ProtocolFormatException(
      '$path: cần chuỗi, nhận ${describeJson(value)}',
    );
  }
  final length = value.runes.length;
  if (length < minLength) {
    throw ProtocolFormatException(
      '$path: chuỗi cần ít nhất $minLength ký tự, có $length',
    );
  }
  if (maxLength != null && length > maxLength) {
    throw ProtocolFormatException(
      '$path: chuỗi dài quá $maxLength ký tự ($length)',
    );
  }
  if (pattern != null && !pattern.hasMatch(value)) {
    throw ProtocolFormatException(
      '$path: "$value" không khớp mẫu ${pattern.pattern}',
    );
  }
  return value;
}

/// Đọc một số nguyên. Số có phần thập phân (kể cả `1.0`) bị từ chối như các
/// cài đặt khác của hợp đồng.
int readInt(Object? value, String path, {int? min, int? max}) {
  if (value is! int) {
    throw ProtocolFormatException(
      '$path: cần số nguyên, nhận ${describeJson(value)}',
    );
  }
  if (min != null && value < min) {
    throw ProtocolFormatException('$path: $value nhỏ hơn mức tối thiểu $min');
  }
  if (max != null && value > max) {
    throw ProtocolFormatException('$path: $value lớn hơn mức tối đa $max');
  }
  return value;
}

/// Đọc một giá trị đúng/sai.
bool readBool(Object? value, String path) {
  if (value is bool) {
    return value;
  }
  throw ProtocolFormatException(
    '$path: cần true hoặc false, nhận ${describeJson(value)}',
  );
}

/// Tên loại của một giá trị JSON, dùng trong thông báo lỗi.
String describeJson(Object? value) => switch (value) {
  null => 'null',
  String() => 'chuỗi',
  int() => 'số nguyên',
  double() => 'số thực',
  bool() => 'giá trị đúng/sai',
  List<Object?>() => 'mảng',
  Map<Object?, Object?>() => 'đối tượng',
  _ => value.runtimeType.toString(),
};
