import 'package:flutter_test/flutter_test.dart';
import 'package:rterm/protocol/json_reader.dart';
import 'package:rterm/protocol/rpc_models.dart';

// Các nhánh của bộ đọc mà bộ ví dụ trong docs/protocol không phủ tới.

Map<String, Object?> _session([Map<String, Object?> override = const {}]) => {
  'id': 'k3x9mqz7ab',
  'account_id': 'acc_1',
  'created_at': 1,
  'last_active_at': 2,
  'cols': 80,
  'rows': 24,
  'clients': 0,
  'durable': true,
  ...override,
};

Matcher _rejectsAt(String path) => throwsA(
  isA<ProtocolFormatException>().having(
    (e) => e.message,
    'message',
    contains(path),
  ),
);

void main() {
  test('sai kieu bi tu choi va thong bao chi dung truong hong', () {
    expect(
      () => SessionInfo.fromJson(_session({'cols': '80'})),
      _rejectsAt('SessionInfo.cols'),
    );
    expect(
      () => SessionListResult.fromJson({
        'sessions': [
          _session(),
          _session({'durable': 'yes'}),
        ],
      }),
      _rejectsAt('SessionListResult.sessions[1].durable'),
    );
    expect(() => SessionInfo.fromJson([1]), _rejectsAt('SessionInfo'));
  });

  test('null o truong da biet bi tu choi ke ca truong tuy chon', () {
    expect(
      () => SessionInfo.fromJson(_session({'title': null})),
      _rejectsAt('SessionInfo.title'),
    );
    expect(
      () => SessionListResult.fromJson({'sessions': null}),
      _rejectsAt('SessionListResult.sessions'),
    );
  });

  test('so co phan thap phan khong duoc nhan la so nguyen', () {
    expect(
      () => SessionCreateParams.fromJson({'cols': 80.0, 'rows': 24}),
      _rejectsAt('SessionCreateParams.cols'),
    );
  });

  test('capability lap lai hoac sai dang bi tu choi', () {
    final client = {'name': 'a', 'version': '1'};
    expect(
      () => HelloParams.fromJson({
        'proto': 1,
        'client': client,
        'capabilities': ['session.v1', 'session.v1'],
      }),
      _rejectsAt('HelloParams.capabilities'),
    );
    expect(
      () => HelloParams.fromJson({
        'proto': 1,
        'client': client,
        'capabilities': ['Session'],
      }),
      _rejectsAt('HelloParams.capabilities[0]'),
    );
  });

  test('do dai chuoi tinh theo ky tu Unicode nhu JSON Schema', () {
    // 128 ký tự ngoài BMP là 256 đơn vị UTF-16 nhưng vẫn đúng giới hạn 128.
    final owner = '\u{1F600}' * 128;
    expect(
      SizeInfo.fromJson({'cols': 1, 'rows': 1, 'owner_client': owner})
          .ownerClient,
      owner,
    );
    expect(
      () => SizeInfo.fromJson({
        'cols': 1,
        'rows': 1,
        'owner_client': '${owner}x',
      }),
      _rejectsAt('SizeInfo.owner_client'),
    );
  });

  test(
    'su kien chu de la duoc giu nguyen de bo qua, chu de da biet co kieu',
    () {
      final unknown = Event.fromJson({
        'topic': 'metrics.cpu',
        'ts': 1,
        'data': {'cpu': 0.5},
      });
      expect(unknown.data, isA<UnknownEventData>());

      final created = Event.fromJson({
        'topic': EventTopic.sessionCreated,
        'ts': 1,
        'data': _session(),
      });
      expect(
        created.data,
        isA<SessionEventData>().having(
          (d) => d.session.id,
          'session.id',
          'k3x9mqz7ab',
        ),
      );
      expect(
        () => Event.fromJson({
          'topic': 'metrics.cpu',
          'ts': 1,
          'data': <Object?>[],
        }),
        _rejectsAt('Event.data'),
      );
    },
  );

  test('may chua them vao server co machine_id rong', () {
    final hello = HelloResult.fromJson({
      'proto': 1,
      'agent': {'name': 'a', 'version': '1'},
      'machine_id': '',
      'os_user': 'deploy',
    });
    expect(hello.machineId, isEmpty);
    expect(
      () => HelloResult.fromJson({
        'proto': 1,
        'agent': {'name': 'a', 'version': '1'},
        'machine_id': 'co dau cach',
        'os_user': 'deploy',
      }),
      _rejectsAt('HelloResult.machine_id'),
    );
  });
}
