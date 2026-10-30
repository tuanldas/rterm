import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:rterm/protocol/api_error.dart';
import 'package:rterm/protocol/json_reader.dart';
import 'package:rterm/protocol/rpc_models.dart';

// Hợp đồng nằm ở docs/protocol: examples/<miền>/<Def>.<biến thể>.json ứng với
// $defs/<Def> của schema/<miền>.schema.json. Kiểm bằng JSON Schema chạy ở phía
// Go và Python; ở đây kiểm model Dart trên cùng bộ ví dụ: đọc được ví dụ hợp
// lệ, ghi lại đúng như cũ, bỏ qua trường lạ, và từ chối ví dụ không hợp lệ.

/// flutter test chạy ở thư mục app/.
const _examplesDir = '../docs/protocol/examples';

/// Miền app dùng: RPC với agent, HTTP với server. Kênh đồng bộ chỉ nối agent
/// với server nên app không có model cho nó.
const _domains = ['rpc', 'http-api'];

typedef _RoundTrip = Map<String, Object?> Function(Object? json);

/// Đọc rồi ghi lại bằng model Dart, theo `<miền>/<Def>`. Thêm model cho một
/// thông điệp nghĩa là thêm dòng ở đây.
final Map<String, _RoundTrip> _models = {
  'rpc/HelloParams': (j) => HelloParams.fromJson(j).toJson(),
  'rpc/HelloResult': (j) => HelloResult.fromJson(j).toJson(),
  'rpc/SessionInfo': (j) => SessionInfo.fromJson(j).toJson(),
  'rpc/SizeInfo': (j) => SizeInfo.fromJson(j).toJson(),
  'rpc/SessionListResult': (j) => SessionListResult.fromJson(j).toJson(),
  'rpc/SessionCreateParams': (j) => SessionCreateParams.fromJson(j).toJson(),
  'rpc/SessionCloseParams': (j) => SessionCloseParams.fromJson(j).toJson(),
  'rpc/SessionSizeParams': (j) => SessionSizeParams.fromJson(j).toJson(),
  'rpc/EventsSubscribeParams': (j) =>
      EventsSubscribeParams.fromJson(j).toJson(),
  'rpc/EventsSubscribeResult': (j) =>
      EventsSubscribeResult.fromJson(j).toJson(),
  'rpc/Event': (j) => Event.fromJson(j).toJson(),
  'rpc/SessionResizedData': (j) => SessionResizedData.fromJson(j).toJson(),
  'rpc/ErrorData': (j) => ErrorData.fromJson(j).toJson(),
  'http-api/ErrorResponse': (j) => ErrorResponse.fromJson(j).toJson(),
};

/// Thân HTTP chỉ agent gửi hoặc nhận (agent tự thêm mình vào server); app
/// không bao giờ cần model cho chúng.
const _agentOnly = {
  'http-api/EnrollStartRequest',
  'http-api/EnrollStartResult',
  'http-api/EnrollPollRequest',
  'http-api/EnrollPollResult',
};

/// Thân HTTP app sẽ dùng nhưng chưa có model. Khi app gọi endpoint tương ứng:
/// viết model, thêm vào [_models], xoá khỏi đây. Thông điệp mới của hợp đồng
/// không nằm ở bảng nào làm test đỏ để không bị bỏ sót.
const _notModelledYet = {
  'http-api/AffectedMachines',
  'http-api/AlertList',
  'http-api/ChangePasswordRequest',
  'http-api/DeviceList',
  'http-api/EnrollApproveRequest',
  'http-api/EnrollInfo',
  'http-api/KeyChallenge',
  'http-api/KeyList',
  'http-api/LoginRequest',
  'http-api/Machine',
  'http-api/MachineList',
  'http-api/MachinePatch',
  'http-api/Me',
  'http-api/Meta',
  'http-api/RefreshRequest',
  'http-api/RegisterKeyRequest',
  'http-api/RegisterKeyResult',
  'http-api/SetupRequest',
  'http-api/SetupResult',
  'http-api/StepUpRequest',
  'http-api/TOTPConfirmRequest',
  'http-api/TOTPConfirmResult',
  'http-api/TOTPEnrollment',
  'http-api/TokenPair',
};

/// Một tệp `examples/<miền>/<Def>.<biến thể>[.invalid].json`.
final class _Example {
  _Example(this.domain, this.fileName, this.json);

  final String domain;
  final String fileName;
  final Object? json;

  List<String> get _parts =>
      fileName.substring(0, fileName.length - '.json'.length).split('.');
  String get def => _parts.first;
  String get variant => _parts.skip(1).join('.');
  bool get invalid => _parts.last == 'invalid';
  String get key => '$domain/$def';
  String get name => '$domain/$fileName';
}

List<_Example> _loadExamples() {
  final out = <_Example>[];
  for (final domain in _domains) {
    final files =
        Directory('$_examplesDir/$domain')
            .listSync()
            .whereType<File>()
            .where((f) => f.path.endsWith('.json'))
            .toList()
          ..sort((a, b) => a.path.compareTo(b.path));
    for (final f in files) {
      final fileName = f.uri.pathSegments.last;
      out.add(_Example(domain, fileName, jsonDecode(f.readAsStringSync())));
    }
  }
  return out;
}

/// Quy ước của bộ ví dụ: trường lạ đặt tên bắt đầu bằng `x_`.
Object? _stripUnknown(Object? value) => switch (value) {
  Map<String, Object?>() => {
    for (final e in value.entries)
      if (!e.key.startsWith('x_')) e.key: _stripUnknown(e.value),
  },
  List<Object?>() => [for (final v in value) _stripUnknown(v)],
  _ => value,
};

/// Thêm trường lạ vào mọi đối tượng JSON ở mọi tầng, kể cả trường lạ mang giá
/// trị null (null chỉ bị cấm ở trường đã biết).
Object? _injectUnknown(Object? value) => switch (value) {
  Map<String, Object?>() => {
    for (final e in value.entries) e.key: _injectUnknown(e.value),
    'x_truong_la': {
      'long': [1, 'hai', 3.5],
    },
    'x_rong': null,
  },
  List<Object?>() => [for (final v in value) _injectUnknown(v)],
  _ => value,
};

void main() {
  final examples = _loadExamples();
  final valid = [
    for (final e in examples)
      if (!e.invalid && _models.containsKey(e.key)) e,
  ];
  final invalid = [
    for (final e in examples)
      if (e.invalid && _models.containsKey(e.key)) e,
  ];

  test(
    'moi vi du cua mien app dung deu co model hoac ghi ro ly do chua co',
    () {
      final keys = {for (final e in examples) e.key};
      final unaccounted = keys.difference({
        ..._models.keys,
        ..._agentOnly,
        ..._notModelledYet,
      });
      expect(
        unaccounted,
        isEmpty,
        reason:
            'thông điệp chưa có model Dart: viết model hoặc ghi vào '
            '_agentOnly/_notModelledYet',
      );

      final listed = {..._agentOnly, ..._notModelledYet};
      expect(
        listed.intersection(_models.keys.toSet()),
        isEmpty,
        reason: 'đã có model thì xoá khỏi bảng chưa có model',
      );
      expect(
        listed.difference(keys),
        isEmpty,
        reason: 'mục không còn ví dụ nào trong hợp đồng',
      );
      expect(
        _models.keys.toSet().difference({for (final e in valid) e.key}),
        isEmpty,
        reason: 'model đã đăng ký nhưng không có ví dụ hợp lệ nào',
      );
      // Không có ví dụ không hợp lệ thì nhóm test từ chối bên dưới rỗng và
      // xanh mà không chứng minh gì.
      expect(invalid, isNotEmpty);
    },
  );

  group('vi du giao thuc parse duoc o Dart', () {
    for (final e in valid) {
      test(e.name, () {
        expect(() => _models[e.key]!(e.json), returnsNormally);
      });
    }
  });

  group('doc roi ghi lai giu nguyen truong da biet', () {
    for (final e in valid) {
      test(e.name, () {
        // Ví dụ viết ở dạng chuẩn của người gửi nên đọc rồi ghi lại phải ra
        // đúng nó; biến thể unknown-fields thì ra đúng phần đã biết.
        final expected = e.variant == 'unknown-fields'
            ? _stripUnknown(e.json)
            : e.json;
        expect(_models[e.key]!(e.json), equals(expected));
      });
    }
  });

  group('truong la bi bo qua khi parse', () {
    for (final e in valid) {
      test(e.name, () {
        final roundTrip = _models[e.key]!;
        final withUnknown = roundTrip(_injectUnknown(e.json));
        expect(
          _stripUnknown(withUnknown),
          equals(_stripUnknown(roundTrip(e.json))),
        );
      });
    }
  });

  group('vi du khong hop le bi model tu choi', () {
    for (final e in invalid) {
      test(e.name, () {
        expect(
          () => _models[e.key]!(e.json),
          throwsA(isA<ProtocolFormatException>()),
        );
      });
    }
  });
}
