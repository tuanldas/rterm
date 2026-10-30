/// Model Dart của RPC agent qua subsystem SSH `agent-rpc`
/// (`docs/protocol/rpc.md`, `docs/protocol/schema/rpc.schema.json`).
///
/// `fromJson` kiểm ràng buộc của schema và ném [ProtocolFormatException] khi
/// dữ liệu sai; trường lạ bị bỏ qua, chủ đề sự kiện lạ không phải lỗi.
/// `toJson` ghi dạng chuẩn của người gửi: trường tuỳ chọn không có giá trị thì
/// vắng mặt, nên đọc rồi ghi lại một ví dụ của hợp đồng cho ra đúng ví dụ đó.
///
/// Các hằng ở đây là định danh trên dây, viết thẳng và không lấy từ
/// `core/product.dart`: chúng phải giữ nguyên khi sản phẩm đổi tên.
library;

import 'json_reader.dart';

/// Phiên bản giao thức RPC của agent. Đổi `proto` là thay đổi phá vỡ, cấm
/// trong v1; tính năng mới đi qua capability.
const rpcProto = 1;

final _idPattern = RegExp(r'^[A-Za-z0-9_-]{1,64}$');
final _capabilityPattern = RegExp(r'^[a-z][a-z0-9_]*\.v[1-9][0-9]*$');
final _topicFilterPattern = RegExp(
  r'^[a-z][a-z0-9_]*(\.([a-z][a-z0-9_]*|\*))*$',
);

/// Đọc mã đối tượng (`$defs/ID`). Bộ ký tự hẹp vì mã phiên đi vào biến môi
/// trường, tên tệp và tên pipe phía agent.
String readId(Object? value, String path) =>
    readString(value, path, pattern: _idPattern);

/// Đọc thời điểm Unix mili-giây UTC (`$defs/Timestamp`).
int readTimestamp(Object? value, String path) => readInt(value, path, min: 0);

/// Đọc tên capability dạng `<miền>.v<n>` (`$defs/Capability`).
String readCapability(Object? value, String path) =>
    readString(value, path, pattern: _capabilityPattern);

/// Đọc số cột hoặc số hàng của terminal (`$defs/Dimension`).
int readDimension(Object? value, String path) =>
    readInt(value, path, min: 1, max: 65535);

String _readNonEmpty(Object? value, String path) =>
    readString(value, path, minLength: 1);

String _readOwnerClient(Object? value, String path) =>
    readString(value, path, minLength: 1, maxLength: 128);

int _readProto(Object? value, String path) {
  final proto = readInt(value, path);
  if (proto != rpcProto) {
    throw ProtocolFormatException(
      '$path: proto $proto không được hỗ trợ, chỉ có $rpcProto',
    );
  }
  return proto;
}

/// Một đầu của kết nối.
final class PeerInfo {
  const PeerInfo({required this.name, required this.version});

  factory PeerInfo.fromJson(Object? json, [String path = 'PeerInfo']) {
    final r = JsonReader(json, path);
    return PeerInfo(
      name: r.value('name', _readNonEmpty),
      version: r.value('version', _readNonEmpty),
    );
  }

  final String name;
  final String version;

  Map<String, Object?> toJson() => {'name': name, 'version': version};
}

/// Tham số của `hello`, yêu cầu đầu tiên của client.
final class HelloParams {
  const HelloParams({required this.client, this.capabilities = const []});

  factory HelloParams.fromJson(Object? json, [String path = 'HelloParams']) {
    final r = JsonReader(json, path);
    r.value('proto', _readProto);
    return HelloParams(
      client: r.value('client', PeerInfo.fromJson),
      capabilities: r.list('capabilities', readCapability, uniqueItems: true),
    );
  }

  int get proto => rpcProto;
  final PeerInfo client;

  /// Mọi capability mà client hiểu.
  final List<String> capabilities;

  Map<String, Object?> toJson() => {
    'proto': proto,
    'client': client.toJson(),
    if (capabilities.isNotEmpty) 'capabilities': capabilities,
  };
}

/// Kết quả của `hello`.
final class HelloResult {
  const HelloResult({
    required this.agent,
    required this.machineId,
    required this.osUser,
    this.capabilities = const [],
  });

  factory HelloResult.fromJson(Object? json, [String path = 'HelloResult']) {
    final r = JsonReader(json, path);
    r.value('proto', _readProto);
    return HelloResult(
      agent: r.value('agent', PeerInfo.fromJson),
      machineId: r.value('machine_id', _readMachineId),
      osUser: r.value('os_user', _readNonEmpty),
      capabilities: r.list('capabilities', readCapability, uniqueItems: true),
    );
  }

  int get proto => rpcProto;
  final PeerInfo agent;

  /// Rỗng khi agent chưa được thêm vào server nào.
  final String machineId;

  /// Tên đăng nhập SSH phải dùng: user hệ điều hành chạy agent.
  final String osUser;

  /// Mọi capability mà agent hiểu; app chỉ gọi method của nhóm có ở đây.
  final List<String> capabilities;

  static String _readMachineId(Object? value, String path) {
    final id = readString(value, path);
    return id.isEmpty ? id : readId(id, path);
  }

  Map<String, Object?> toJson() => {
    'proto': proto,
    'agent': agent.toJson(),
    'machine_id': machineId,
    'os_user': osUser,
    if (capabilities.isNotEmpty) 'capabilities': capabilities,
  };
}

/// Một phiên terminal đang sống trên máy.
final class SessionInfo {
  const SessionInfo({
    required this.id,
    this.title,
    required this.accountId,
    required this.createdAt,
    required this.lastActiveAt,
    required this.cols,
    required this.rows,
    required this.clients,
    required this.durable,
  });

  factory SessionInfo.fromJson(Object? json, [String path = 'SessionInfo']) {
    final r = JsonReader(json, path);
    return SessionInfo(
      id: r.value('id', readId),
      title: r.optional('title', readString),
      accountId: r.value('account_id', readId),
      createdAt: r.value('created_at', readTimestamp),
      lastActiveAt: r.value('last_active_at', readTimestamp),
      cols: r.value('cols', readDimension),
      rows: r.value('rows', readDimension),
      clients: r.value('clients', (v, p) => readInt(v, p, min: 0)),
      durable: r.value('durable', readBool),
    );
  }

  final String id;
  final String? title;

  /// Tài khoản sở hữu phiên.
  final String accountId;
  final int createdAt;
  final int lastActiveAt;
  final int cols;
  final int rows;

  /// Số client đang attach.
  final int clients;

  /// `false` khi môi trường không bảo đảm phiên sống tiếp qua lúc đăng xuất
  /// hoặc khởi động lại daemon; app nên cảnh báo người dùng.
  final bool durable;

  Map<String, Object?> toJson() => {
    'id': id,
    if (title != null) 'title': title,
    'account_id': accountId,
    'created_at': createdAt,
    'last_active_at': lastActiveAt,
    'cols': cols,
    'rows': rows,
    'clients': clients,
    'durable': durable,
  };
}

/// Kích thước PTY của phiên và client đang giữ quyền đặt kích thước; kết quả
/// của `session.size`.
final class SizeInfo {
  const SizeInfo({required this.cols, required this.rows, this.ownerClient});

  factory SizeInfo.fromJson(Object? json, [String path = 'SizeInfo']) {
    final r = JsonReader(json, path);
    return SizeInfo(
      cols: r.value('cols', readDimension),
      rows: r.value('rows', readDimension),
      ownerClient: r.optional('owner_client', _readOwnerClient),
    );
  }

  final int cols;
  final int rows;

  /// Mã không trong suốt của client giữ quyền chủ cỡ; vắng khi phiên không có
  /// client.
  final String? ownerClient;

  Map<String, Object?> toJson() => {
    'cols': cols,
    'rows': rows,
    if (ownerClient != null) 'owner_client': ownerClient,
  };
}

/// Kết quả của `session.list`.
final class SessionListResult {
  const SessionListResult({this.sessions = const []});

  factory SessionListResult.fromJson(
    Object? json, [
    String path = 'SessionListResult',
  ]) {
    final r = JsonReader(json, path);
    return SessionListResult(
      sessions: r.list('sessions', SessionInfo.fromJson),
    );
  }

  final List<SessionInfo> sessions;

  Map<String, Object?> toJson() => {
    if (sessions.isNotEmpty) 'sessions': [for (final s in sessions) s.toJson()],
  };
}

/// Tham số của `session.create`; kết quả là [SessionInfo].
final class SessionCreateParams {
  const SessionCreateParams({required this.cols, required this.rows});

  factory SessionCreateParams.fromJson(
    Object? json, [
    String path = 'SessionCreateParams',
  ]) {
    final r = JsonReader(json, path);
    return SessionCreateParams(
      cols: r.value('cols', readDimension),
      rows: r.value('rows', readDimension),
    );
  }

  final int cols;
  final int rows;

  Map<String, Object?> toJson() => {'cols': cols, 'rows': rows};
}

/// Tham số của `session.close`; kết quả là đối tượng rỗng.
final class SessionCloseParams {
  const SessionCloseParams({required this.sessionId});

  factory SessionCloseParams.fromJson(
    Object? json, [
    String path = 'SessionCloseParams',
  ]) {
    final r = JsonReader(json, path);
    return SessionCloseParams(sessionId: r.value('session_id', readId));
  }

  final String sessionId;

  Map<String, Object?> toJson() => {'session_id': sessionId};
}

/// Tham số của `session.size`; kết quả là [SizeInfo].
final class SessionSizeParams {
  const SessionSizeParams({required this.sessionId, this.take = false});

  factory SessionSizeParams.fromJson(
    Object? json, [
    String path = 'SessionSizeParams',
  ]) {
    final r = JsonReader(json, path);
    return SessionSizeParams(
      sessionId: r.value('session_id', readId),
      take: r.optional('take', readBool) ?? false,
    );
  }

  final String sessionId;

  /// Đòi quyền chủ cỡ cho kết nối đang gọi.
  final bool take;

  Map<String, Object?> toJson() => {
    'session_id': sessionId,
    if (take) 'take': true,
  };
}

/// Tham số của `events.subscribe`. Mỗi phần tử là một chủ đề, hoặc mẫu
/// `<tiền tố>.*` khớp mọi chủ đề bắt đầu bằng `<tiền tố>.`.
final class EventsSubscribeParams {
  const EventsSubscribeParams({required this.topics});

  factory EventsSubscribeParams.fromJson(
    Object? json, [
    String path = 'EventsSubscribeParams',
  ]) {
    final r = JsonReader(json, path);
    return EventsSubscribeParams(
      topics: r.list(
        'topics',
        (v, p) => readString(v, p, pattern: _topicFilterPattern),
        required: true,
        minItems: 1,
      ),
    );
  }

  final List<String> topics;

  Map<String, Object?> toJson() => {'topics': topics};
}

/// Kết quả của `events.subscribe`: các mẫu agent chấp nhận. Mẫu agent không
/// biết bị bỏ qua và không có mặt ở đây.
final class EventsSubscribeResult {
  const EventsSubscribeResult({this.topics = const []});

  factory EventsSubscribeResult.fromJson(
    Object? json, [
    String path = 'EventsSubscribeResult',
  ]) {
    final r = JsonReader(json, path);
    return EventsSubscribeResult(topics: r.list('topics', _readNonEmpty));
  }

  final List<String> topics;

  Map<String, Object?> toJson() => {if (topics.isNotEmpty) 'topics': topics};
}

/// Data của sự kiện `session.resized`.
final class SessionResizedData {
  const SessionResizedData({
    required this.id,
    required this.cols,
    required this.rows,
    this.ownerClient,
  });

  factory SessionResizedData.fromJson(
    Object? json, [
    String path = 'SessionResizedData',
  ]) {
    final r = JsonReader(json, path);
    return SessionResizedData(
      id: r.value('id', readId),
      cols: r.value('cols', readDimension),
      rows: r.value('rows', readDimension),
      ownerClient: r.optional('owner_client', _readOwnerClient),
    );
  }

  final String id;
  final int cols;
  final int rows;
  final String? ownerClient;

  Map<String, Object?> toJson() => {
    'id': id,
    'cols': cols,
    'rows': rows,
    if (ownerClient != null) 'owner_client': ownerClient,
  };
}

/// Chủ đề sự kiện mà app hiểu. Tập mở: chủ đề lạ đến dưới dạng
/// [UnknownEventData] để bỏ qua, không phải lỗi.
abstract final class EventTopic {
  static const sessionCreated = 'session.created';
  static const sessionClosed = 'session.closed';
  static const sessionAttached = 'session.attached';
  static const sessionDetached = 'session.detached';
  static const sessionResized = 'session.resized';
}

/// Tham số của notification `event`. Hình dạng [data] phụ thuộc [topic].
final class Event {
  const Event({required this.topic, required this.ts, required this.data});

  factory Event.fromJson(Object? json, [String path = 'Event']) {
    final r = JsonReader(json, path);
    final topic = r.value('topic', _readNonEmpty);
    return Event(
      topic: topic,
      ts: r.value('ts', readTimestamp),
      data: r.value('data', (v, p) => EventData._read(topic, v, p)),
    );
  }

  final String topic;
  final int ts;
  final EventData data;

  Map<String, Object?> toJson() => {
    'topic': topic,
    'ts': ts,
    'data': data.toJson(),
  };
}

/// Data của một sự kiện, theo chủ đề.
sealed class EventData {
  const EventData();

  static EventData _read(String topic, Object? json, String path) {
    switch (topic) {
      case EventTopic.sessionCreated ||
          EventTopic.sessionClosed ||
          EventTopic.sessionAttached ||
          EventTopic.sessionDetached:
        return SessionEventData(SessionInfo.fromJson(json, path));
      case EventTopic.sessionResized:
        return ResizedEventData(SessionResizedData.fromJson(json, path));
      default:
        return UnknownEventData(readObject(json, path));
    }
  }

  Map<String, Object?> toJson();
}

/// Data của `session.created`, `session.closed`, `session.attached`,
/// `session.detached`: trạng thái phiên tại thời điểm phát.
final class SessionEventData extends EventData {
  const SessionEventData(this.session);

  final SessionInfo session;

  @override
  Map<String, Object?> toJson() => session.toJson();
}

/// Data của `session.resized`.
final class ResizedEventData extends EventData {
  const ResizedEventData(this.resized);

  final SessionResizedData resized;

  @override
  Map<String, Object?> toJson() => resized.toJson();
}

/// Data của chủ đề app chưa biết, giữ nguyên để không mất thông tin; người
/// nhận bỏ qua sự kiện này.
final class UnknownEventData extends EventData {
  UnknownEventData(Map<String, Object?> data) : data = Map.unmodifiable(data);

  final Map<String, Object?> data;

  @override
  Map<String, Object?> toJson() => data;
}

/// Nội dung `data` của lỗi JSON-RPC riêng -32001..-32004 (registry trong
/// `docs/protocol/README.md`). Cả hai trường đều tuỳ chọn.
final class ErrorData {
  const ErrorData({this.reason, this.retryAfter});

  factory ErrorData.fromJson(Object? json, [String path = 'ErrorData']) {
    final r = JsonReader(json, path);
    return ErrorData(
      reason: r.optional('reason', _readNonEmpty),
      retryAfter: r.optional('retry_after', (v, p) => readInt(v, p, min: 0)),
    );
  }

  /// Mã lý do ổn định, máy đọc được; tập mở nên lý do lạ không phải lỗi.
  final String? reason;

  /// Số giây nên chờ trước khi thử lại.
  final int? retryAfter;

  Map<String, Object?> toJson() => {
    if (reason != null) 'reason': reason,
    if (retryAfter != null) 'retry_after': retryAfter,
  };
}
