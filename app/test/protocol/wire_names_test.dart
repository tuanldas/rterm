import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:rterm/protocol/rpc_models.dart';

// So định danh trên dây mà app khai báo với bản đóng băng dùng chung của hợp
// đồng (docs/protocol/README.md). Agent Go và server Python cũng so với đúng tệp
// này, nên một hằng gõ sai ở app (như chủ đề sự kiện) làm test đỏ, thay vì lặng
// lẽ biến sự kiện đó thành UnknownEventData.

/// flutter test chạy ở thư mục app/.
const _goldenPath = '../internal/protocol/testdata/wire_names.golden.json';

/// Mọi hằng của [EventTopic].
const _eventTopics = [
  EventTopic.sessionCreated,
  EventTopic.sessionClosed,
  EventTopic.sessionAttached,
  EventTopic.sessionDetached,
  EventTopic.sessionResized,
];

/// Mục của bản đóng băng được so với hằng của app trong tệp này.
const _compared = {'protos', 'event_topics'};

/// Mục của bản đóng băng mà app chưa khai báo hằng nào, kèm lý do. Khi app
/// thêm hằng cho một mục: so hằng đó ở đây và chuyển mục sang [_compared]. Mục
/// mới của bản đóng băng chưa được xếp vào đâu thì test đỏ để không bị bỏ sót.
const _notDeclared = {
  'api_prefix': 'chưa có client HTTP',
  'agent_sync_path': 'kênh đồng bộ chỉ nối agent với server',
  'sync_methods': 'kênh đồng bộ chỉ nối agent với server',
  'session_env_vars': 'chưa mở kênh terminal SSH',
  'session_env_new': 'chưa mở kênh terminal SSH',
  'exit_session_not_found': 'chưa mở kênh terminal SSH',
  'stderr_session_not_found': 'chưa mở kênh terminal SSH',
  'subsystem_agent_rpc': 'chưa có client RPC',
  'rpc_methods': 'chưa có client RPC',
  'rpc_notifications': 'chưa có client RPC',
  'capabilities': 'chưa đọc capability của agent hay server',
  'signing_contexts': 'chưa ký chứng minh sở hữu khoá thiết bị',
  'jsonrpc_error_codes': 'chưa có client RPC xử lý mã lỗi',
};

/// `proto` trong bản đóng băng mà app chưa khai báo, kèm lý do.
const _protosNotDeclared = {
  'api': 'chưa có client HTTP',
  'sync': 'kênh đồng bộ chỉ nối agent với server',
};

void main() {
  final golden =
      jsonDecode(File(_goldenPath).readAsStringSync()) as Map<String, Object?>;

  test(
    'moi muc cua ban dong bang duoc so hoac ghi ro ly do app chua khai bao',
    () {
      final frozen = golden.keys.toSet();
      final classified = {..._compared, ..._notDeclared.keys};
      expect(
        frozen.difference(classified),
        isEmpty,
        reason:
            'bản đóng băng có mục mới: so với hằng của app hoặc ghi vào '
            '_notDeclared kèm lý do',
      );
      expect(
        classified.difference(frozen),
        isEmpty,
        reason: 'mục không còn trong bản đóng băng',
      );
      expect(_compared.intersection(_notDeclared.keys.toSet()), isEmpty);
    },
  );

  test('chu de su kien app khai bao khop ban dong bang', () {
    final frozen = (golden['event_topics']! as List<Object?>).toSet();
    final declared = _eventTopics.toSet();
    expect(
      declared,
      hasLength(_eventTopics.length),
      reason: 'hai hằng EventTopic trùng giá trị',
    );
    expect(
      frozen.difference(declared),
      isEmpty,
      reason:
          'chủ đề đã đóng băng mà app thiếu hằng hoặc gõ sai: sự kiện đó sẽ '
          'thành UnknownEventData',
    );
    expect(
      declared.difference(frozen),
      isEmpty,
      reason:
          'hằng EventTopic không có trong bản đóng băng: gõ sai, hoặc chủ đề '
          'mới chưa được đóng băng',
    );
  });

  test('proto rpc app khai bao khop ban dong bang', () {
    final protos = golden['protos']! as Map<String, Object?>;
    expect(rpcProto, protos['rpc'], reason: 'đổi proto là thay đổi phá vỡ');
    expect(
      protos.keys.toSet(),
      {'rpc', ..._protosNotDeclared.keys},
      reason:
          'proto mới trong bản đóng băng: khai báo ở app hoặc ghi vào '
          '_protosNotDeclared kèm lý do',
    );
  });
}
