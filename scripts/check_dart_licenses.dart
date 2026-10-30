// Cổng giấy phép cho phụ thuộc pub của app Flutter, trực tiếp lẫn gián tiếp.
//
// Đọc trong thư mục app sau `flutter pub get`:
//   - .dart_tool/package_config.json: thư mục của từng gói;
//   - .dart_tool/package_graph.json: đồ thị phụ thuộc, để biết gói nào vào bản
//     phát hành (runtime) và gói nào chỉ dùng khi phát triển (dev);
//   - pubspec.lock: nguồn của gói. Gói nguồn `sdk` thuộc SDK Flutter, không
//     phải gói pub, nên nằm ngoài phạm vi; phụ thuộc pub của chúng vẫn được kiểm.
// Giấy phép nhận diện từ văn bản các tệp LICENSE/COPYING ở gốc mỗi gói rồi xếp
// loại theo scripts/license_policy.txt, tệp dùng chung với cổng Go và PyPI.
//
// Chạy:   dart run scripts/check_dart_licenses.dart [--app <thư mục>] [--policy <tệp>]
// Mã thoát: 0 mọi gói đạt; 1 có gói bị cấm, cần hỏi hoặc không rõ giấy phép;
// 2 dùng sai hoặc thiếu dữ liệu (chưa chạy `flutter pub get`, chính sách hỏng).

import 'dart:convert';
import 'dart:io';

/// Hệ gói của cổng này trong dòng `exception` của chính sách.
const ecosystem = 'pub';

/// Mã giả cho gói không nhận diện được giấy phép nào.
const unknownLicense = 'UNKNOWN';

/// Phạm vi của một gói: vào bản phát hành hay chỉ dùng khi phát triển.
enum Scope { runtime, dev }

/// Kết luận cho một giấy phép (hoặc cả gói), xếp từ tốt tới tệ.
enum Verdict {
  allow('đạt'),
  exception('đạt nhờ ngoại lệ'),
  ask('cần hỏi người dùng'),
  unknown('không rõ giấy phép'),
  deny('bị cấm');

  const Verdict(this.label);

  final String label;

  bool get passes => this == allow || this == exception;
}

/// Lỗi dữ liệu đầu vào: chính sách sai cú pháp, thiếu tệp của pub.
class GateException implements Exception {
  GateException(this.message);

  final String message;

  @override
  String toString() => message;
}

/// Một dòng `exception <hệ> <gói> <mã-SPDX> <phạm-vi> <lý do>` của chính sách.
class PolicyException {
  PolicyException(
    this.ecosystem,
    this.package,
    this.license,
    this.scope,
    this.reason,
  );

  final String ecosystem;
  final String package;
  final String license;
  final Scope scope;
  final String reason;

  /// Ngoại lệ cho phạm vi runtime bao cả dev; ngoại lệ dev không áp cho gói
  /// vào bản phát hành.
  bool coversScope(Scope s) => scope == Scope.runtime || s == Scope.dev;
}

/// Nội dung của scripts/license_policy.txt.
class Policy {
  Policy(this.allow, this.deny, this.exceptions);

  final Set<String> allow;

  /// Mã SPDX hoặc tiền tố kết thúc bằng `*`.
  final List<String> deny;
  final List<PolicyException> exceptions;

  /// Mã SPDX không phân biệt hoa thường nên so cấm cũng vậy: so phân biệt thì
  /// một mã cấm viết thường, như `gpl-3.0-only` ghi trong ngoại lệ, sẽ lọt qua.
  bool isDenied(String license) {
    final id = license.toLowerCase();
    return deny.any((d) {
      final entry = d.toLowerCase();
      return entry.endsWith('*')
          ? id.startsWith(entry.substring(0, entry.length - 1))
          : id == entry;
    });
  }

  /// Kết luận theo nhóm allow/deny; cấm thắng nếu một mã nằm ở cả hai.
  Verdict verdictOf(String license) {
    if (license == unknownLicense) {
      return Verdict.unknown;
    }
    if (isDenied(license)) {
      return Verdict.deny;
    }
    return allow.contains(license) ? Verdict.allow : Verdict.ask;
  }
}

/// Đọc chính sách. Dòng sai cú pháp là lỗi chứ không bị bỏ qua: một ngoại lệ
/// gõ sai mà lặng lẽ mất thì cổng đỏ không rõ lý do, còn một dòng deny bị bỏ
/// qua thì cổng xanh sai.
Policy parsePolicy(String text) {
  final allow = <String>{};
  final deny = <String>[];
  final exceptions = <PolicyException>[];
  final lines = const LineSplitter().convert(text);
  for (var i = 0; i < lines.length; i++) {
    final line = lines[i].trim();
    if (line.isEmpty || line.startsWith('#')) {
      continue;
    }
    final parts = line.split(RegExp(r'\s+'));
    Never fail(String why) =>
        throw GateException('chính sách dòng ${i + 1}: $why: "$line"');
    switch (parts.first) {
      case 'allow':
        if (parts.length != 2) fail('cần "allow <mã-SPDX>"');
        allow.add(parts[1]);
      case 'deny':
        if (parts.length != 2) fail('cần "deny <mã-SPDX | tiền-tố*>"');
        deny.add(parts[1]);
      case 'exception':
        if (parts.length < 6) {
          fail('cần "exception <hệ> <gói> <mã-SPDX> <phạm-vi> <lý do>"');
        }
        if (!const {'go', 'pub', 'pypi'}.contains(parts[1])) {
          fail('hệ "${parts[1]}" không thuộc go, pub, pypi');
        }
        final scope = Scope.values.where((s) => s.name == parts[4]);
        if (scope.isEmpty)
          fail('phạm vi "${parts[4]}" không thuộc runtime, dev');
        exceptions.add(
          PolicyException(
            parts[1],
            parts[2],
            parts[3],
            scope.single,
            parts.sublist(5).join(' '),
          ),
        );
      default:
        fail('chỉ thị lạ "${parts.first}"');
    }
  }
  if (allow.isEmpty) {
    throw GateException('chính sách không có dòng allow nào');
  }
  return Policy(allow, deny, exceptions);
}

/// Đưa văn bản giấy phép về dạng so được: chữ thường, dấu nháy cong thành
/// nháy thẳng, mọi khoảng trắng gộp thành một dấu cách.
String _normalize(String text) => text
    .toLowerCase()
    .replaceAll(RegExp('[‘’]'), "'")
    .replaceAll(RegExp('[“”]'), '"')
    .replaceAll(RegExp(r'\s+'), ' ');

// Văn bản GPL, LGPL, AGPL và MPL nhắc tên nhau trong nội dung (GPL-3.0 bảo
// "hãy dùng GNU Lesser General Public License", MPL-2.0 liệt kê cả ba giấy
// phép GNU làm "Secondary License"), nên đếm mọi lần nhắc tên sẽ ra sai. Chỉ
// tin tiêu đề kèm ngày phát hành, hoặc câu thông báo áp dụng giấy phép.
final _gnuHeader = RegExp(
  r'gnu (affero |lesser |library )?general public license version '
  r'(\d+(?:\.\d+)?), (?:\d{1,2} )?[a-z]+ (?:19|20)\d\d',
);
final _gnuNotice = RegExp(
  r'terms of the gnu (affero |lesser |library )?general public license,? '
  r'as published by the free software foundation',
);
final _version = RegExp(r'version (\d+(?:\.\d+)?)');
final _mpl = RegExp(r'mozilla public license,? (?:version |v\. ?)(\d\.\d)');
final _epl = RegExp(
  r'eclipse public license,? (?:- )?v(?:ersion|\.)? ?(\d\.\d)',
);

/// Mã SPDX của một giấy phép họ GNU. Không rõ phiên bản thì theo quy ước SPDX
/// là "bản nào cũng được" (`-or-later` từ bản đầu tiên).
String _gnuId(String? kind, String? version) {
  final family = switch (kind) {
    'affero ' => 'AGPL',
    'lesser ' || 'library ' => 'LGPL',
    _ => 'GPL',
  };
  if (version == null) {
    return '$family-${family == 'LGPL' ? '2.0' : '1.0'}-or-later';
  }
  return '$family-${version.contains('.') ? version : '$version.0'}';
}

/// Nhận diện các giấy phép có trong văn bản [text] (nối từ mọi tệp giấy phép
/// của một gói). Chỉ nhận khi gặp câu đặc trưng của văn bản chuẩn; không nhận
/// ra gì thì trả tập rỗng để gói bị coi là không rõ giấy phép.
///
/// Họ copyleft được dò độc lập với họ dễ dãi, nên một tệp ghép cả MIT lẫn GPL
/// ra cả hai và gói bị chặn: cổng thà đỏ nhầm còn hơn xanh nhầm.
Set<String> detectLicenses(String text) {
  final t = _normalize(text);
  final found = <String>{};

  for (final m in _gnuHeader.allMatches(t)) {
    found.add(_gnuId(m[1], m[2]));
  }
  for (final m in _gnuNotice.allMatches(t)) {
    final tail = t.substring(m.end, (m.end + 80).clamp(0, t.length));
    found.add(_gnuId(m[1], _version.firstMatch(tail)?[1]));
  }
  for (final m in _mpl.allMatches(t)) {
    found.add('MPL-${m[1]}');
  }
  for (final m in _epl.allMatches(t)) {
    found.add('EPL-${m[1]}');
  }
  if (t.contains('common development and distribution license')) {
    found.add('CDDL-1.0');
  }

  if (t.contains('apache license') && t.contains('version 2.0')) {
    found.add('Apache-2.0');
  }
  if (t.contains(
    'permission is hereby granted, free of charge, to any person obtaining a copy',
  )) {
    found.add(
      t.contains(
            'the above copyright notice and this permission notice shall be included',
          )
          ? 'MIT'
          : 'MIT-0',
    );
  }
  if (t.contains(
    'redistribution and use in source and binary forms, with or without modification, are permitted',
  )) {
    if (t.contains(
      'all advertising materials mentioning features or use of this software',
    )) {
      found.add('BSD-4-Clause');
    } else if (t.contains(
      'to endorse or promote products derived from this software',
    )) {
      found.add('BSD-3-Clause');
    } else {
      found.add('BSD-2-Clause');
    }
  }
  if (RegExp(
    r'permission to use, copy, modify, and(/or)? distribute this software for any purpose with or without fee is hereby granted',
  ).hasMatch(t)) {
    found.add(
      t.contains(
            'provided that the above copyright notice and this permission notice appear in all copies',
          )
          ? 'ISC'
          : '0BSD',
    );
  }
  if (t.contains(
        "this software is provided 'as-is', without any express or implied warranty",
      ) &&
      t.contains(
        'permission is granted to anyone to use this software for any purpose',
      )) {
    found.add('Zlib');
  }
  if (t.contains('boost software license - version 1.0')) {
    found.add('BSL-1.0');
  }
  if (t.contains(
    'this is free and unencumbered software released into the public domain',
  )) {
    found.add('Unlicense');
  }
  if (t.contains('cc0 1.0 universal')) {
    found.add('CC0-1.0');
  }
  return found;
}

/// Kết luận cho một gói có các giấy phép [licenses] (rỗng là không rõ). Nhiều
/// giấy phép được coi như AND: mọi giấy phép đều phải đạt, vì từ văn bản không
/// phân biệt được "chọn một trong hai" với "mỗi phần một giấy phép".
///
/// Ngoại lệ chỉ gỡ được nhóm "cần hỏi" và "không rõ"; giấy phép bị cấm vẫn bị
/// cấm, muốn đổi phải sửa chính dòng deny cho thấy rõ là đổi chính sách.
({Verdict verdict, List<String> notes}) classify(
  Policy policy,
  String package,
  Set<String> licenses,
  Scope scope,
) {
  final own = [
    for (final e in policy.exceptions)
      if (e.ecosystem == ecosystem && e.package == package) e,
  ];
  final notes = <String>[];
  var worst = Verdict.allow;
  for (final license in licenses.isEmpty ? {unknownLicense} : licenses) {
    var v = policy.verdictOf(license);
    if (v == Verdict.ask || v == Verdict.unknown) {
      // Văn bản không nhận diện được thì ngoại lệ ghi tay là kết luận của người
      // đã đọc giấy phép; nhận diện được thì ngoại lệ phải đúng mã đó, để gói
      // đổi sang giấy phép khác vẫn bị chặn lại.
      final matching = own.where(
        (e) => e.license == license || license == unknownLicense,
      );
      final inScope = matching.where((e) => e.coversScope(scope));
      // Chỉ cần một ngoại lệ khớp ghi mã bị cấm là cấm, để kết quả không tuỳ
      // thứ tự dòng trong tệp chính sách.
      final banned = inScope.where((e) => policy.isDenied(e.license));
      if (banned.isNotEmpty) {
        v = Verdict.deny;
        notes.add(
          '$license: ngoại lệ ghi ${banned.first.license}, giấy phép bị cấm',
        );
      } else if (inScope.isNotEmpty) {
        v = Verdict.exception;
        notes.add('$license: ngoại lệ (${inScope.first.reason})');
      } else if (matching.isNotEmpty) {
        notes.add(
          '$license: ${v.label}; ngoại lệ chỉ cho phạm vi dev nhưng gói vào bản phát hành',
        );
      } else {
        notes.add('$license: ${v.label}');
      }
    } else if (v == Verdict.deny) {
      notes.add('$license: ${v.label}');
    }
    if (v.index > worst.index) {
      worst = v;
    }
  }
  return (verdict: worst, notes: notes);
}

/// Gói đã được phân giải bởi pub.
class ResolvedPackage {
  ResolvedPackage(this.name, this.version, this.root, this.source);

  final String name;
  final String version;
  final Directory root;
  final String source;
}

/// Đọc nguồn và phiên bản từng gói trong pubspec.lock. Tệp do pub sinh với cấu
/// trúc cố định nên đọc theo dòng là đủ, khỏi kéo thêm bộ đọc YAML.
Map<String, ({String source, String version})> parsePubspecLock(String text) {
  final out = <String, ({String source, String version})>{};
  String? current;
  String? source;
  String? version;
  void flush() {
    if (current != null) {
      if (source == null) {
        throw GateException('pubspec.lock: gói "$current" không có source');
      }
      out[current] = (source: source, version: version ?? '?');
    }
  }

  var inPackages = false;
  for (final line in const LineSplitter().convert(text)) {
    if (!line.startsWith(' ')) {
      flush();
      current = null;
      inPackages = line.trimRight() == 'packages:';
      continue;
    }
    if (!inPackages) {
      continue;
    }
    final pkg = RegExp(r'^  ([A-Za-z0-9_]+):\s*$').firstMatch(line);
    if (pkg != null) {
      flush();
      current = pkg[1];
      source = null;
      version = null;
      continue;
    }
    final field = RegExp(r'^    (source|version): "?([^"]*)"?\s*$')
        .firstMatch(line);
    if (field != null && field[1] == 'source') {
      source = field[2];
    } else if (field != null) {
      version = field[2];
    }
  }
  flush();
  return out;
}

/// Tìm gói vào bản phát hành: mọi gói đi tới được từ `dependencies` của gói
/// gốc. Gói còn lại chỉ đến qua `dev_dependencies`.
Set<String> runtimeClosure(Map<String, Object?> graph) {
  final roots = (graph['roots'] as List<Object?>?)?.cast<String>() ?? const [];
  final deps = <String, List<String>>{};
  final rootDeps = <String>[];
  for (final p in (graph['packages'] as List<Object?>? ?? const [])) {
    final m = p as Map<String, Object?>;
    final name = m['name'] as String;
    deps[name] = (m['dependencies'] as List<Object?>? ?? const [])
        .cast<String>();
    if (roots.contains(name)) {
      rootDeps.addAll(deps[name]!);
    }
  }
  if (roots.isEmpty || deps.isEmpty) {
    throw GateException(
      'package_graph.json không có gói gốc hoặc danh sách gói',
    );
  }
  final seen = <String>{};
  final queue = [...rootDeps];
  while (queue.isNotEmpty) {
    final name = queue.removeLast();
    if (seen.add(name)) {
      queue.addAll(deps[name] ?? const []);
    }
  }
  return seen;
}

/// Nối mọi tệp giấy phép ở gốc gói (LICENSE, LICENSE.md, LICENSE-MIT,
/// COPYING, COPYING.LESSER, UNLICENSE…); rỗng nếu không có tệp nào.
String readLicenseText(Directory root) {
  final files =
      root
          .listSync()
          .whereType<File>()
          .where(
            (f) => RegExp(
              r'^(licen[cs]e|copying|unlicense)',
              caseSensitive: false,
            ).hasMatch(f.uri.pathSegments.last),
          )
          .toList()
        ..sort((a, b) => a.path.compareTo(b.path));
  return [for (final f in files) f.readAsStringSync()].join('\n');
}

Map<String, Object?> _readJson(File file) {
  if (!file.existsSync()) {
    throw GateException(
      'thiếu ${file.path}; chạy `flutter pub get` trong thư mục app trước',
    );
  }
  final value = jsonDecode(file.readAsStringSync());
  if (value is! Map<String, Object?>) {
    throw GateException('${file.path} không phải đối tượng JSON');
  }
  return value;
}

/// Liệt kê các gói pub đã phân giải của app ở [app], trừ chính app.
List<ResolvedPackage> resolvedPackages(Directory app) {
  final configFile = File('${app.path}/.dart_tool/package_config.json');
  final config = _readJson(configFile);
  final lockFile = File('${app.path}/pubspec.lock');
  if (!lockFile.existsSync()) {
    throw GateException('thiếu ${lockFile.path}');
  }
  final lock = parsePubspecLock(lockFile.readAsStringSync());
  final out = <ResolvedPackage>[];
  for (final p in config['packages'] as List<Object?>) {
    final m = p as Map<String, Object?>;
    final name = m['name'] as String;
    final locked = lock[name];
    if (locked == null) {
      // Chỉ chính app có trong package_config mà không có trong pubspec.lock.
      continue;
    }
    // rootUri tương đối được tính từ thư mục chứa package_config.json.
    final root = configFile.absolute.uri.resolve(m['rootUri'] as String);
    out.add(
      ResolvedPackage(
        name,
        locked.version,
        Directory.fromUri(root),
        locked.source,
      ),
    );
  }
  return out;
}

Never _usage(String message) {
  stderr.writeln(message);
  stderr.writeln(
    'cách dùng: dart run scripts/check_dart_licenses.dart [--app <thư mục app>] [--policy <tệp chính sách>]',
  );
  exit(2);
}

void main(List<String> args) {
  final repoRoot = File.fromUri(Platform.script).parent.parent.path;
  var appPath = '$repoRoot/app';
  var policyPath = '$repoRoot/scripts/license_policy.txt';
  for (var i = 0; i < args.length; i++) {
    final value = i + 1 < args.length ? args[i + 1] : null;
    switch (args[i]) {
      case '--app' when value != null:
        appPath = value;
        i++;
      case '--policy' when value != null:
        policyPath = value;
        i++;
      default:
        _usage('đối số không hợp lệ: ${args[i]}');
    }
  }

  try {
    exit(run(Directory(appPath), File(policyPath)));
  } on GateException catch (e) {
    stderr.writeln('cổng giấy phép pub: $e');
    exit(2);
  } on FileSystemException catch (e) {
    stderr.writeln(
      'cổng giấy phép pub: không đọc được ${e.path}: ${e.message}',
    );
    exit(2);
  } on FormatException catch (e) {
    stderr.writeln(
      'cổng giấy phép pub: tệp của pub không phải JSON hợp lệ: ${e.message}',
    );
    exit(2);
  } on TypeError catch (e) {
    // Tệp của pub đổi cấu trúc: dừng với mã 2 thay vì đoán, kẻo cổng xanh sai.
    stderr.writeln('cổng giấy phép pub: tệp của pub có cấu trúc lạ: $e');
    exit(2);
  }
}

/// Chạy cổng và in báo cáo; trả mã thoát 0 hoặc 1.
int run(Directory app, File policyFile) {
  if (!policyFile.existsSync()) {
    throw GateException('không thấy tệp chính sách ${policyFile.path}');
  }
  final policy = parsePolicy(policyFile.readAsStringSync());
  final runtime = runtimeClosure(
    _readJson(File('${app.path}/.dart_tool/package_graph.json')),
  );
  final packages = resolvedPackages(app)
    ..sort((a, b) => a.name.compareTo(b.name));

  final inScope = packages.where((p) => p.source != 'sdk').toList();
  final sdk = packages.where((p) => p.source == 'sdk').map((p) => p.name);
  var failed = 0;
  for (final p in inScope) {
    final scope = runtime.contains(p.name) ? Scope.runtime : Scope.dev;
    final licenses = detectLicenses(readLicenseText(p.root));
    final result = classify(policy, p.name, licenses, scope);
    final shown = licenses.isEmpty
        ? unknownLicense
        : (licenses.toList()..sort()).join(' AND ');
    final mark = result.verdict.passes ? 'OK ' : 'ĐỎ ';
    stdout.writeln(
      '$mark ${p.name} ${p.version} [${scope.name}] $shown'
      '${result.notes.isEmpty ? '' : ' — ${result.notes.join('; ')}'}',
    );
    if (!result.verdict.passes) {
      failed++;
    }
  }

  final names = inScope.map((p) => p.name).toSet();
  for (final e in policy.exceptions) {
    if (e.ecosystem == ecosystem && !names.contains(e.package)) {
      stdout.writeln(
        'Cảnh báo: ngoại lệ cho gói ${e.package} không còn dùng, nên xoá khỏi chính sách.',
      );
    }
  }
  stdout.writeln(
    'Đã kiểm ${inScope.length} gói pub; bỏ qua gói của SDK Flutter: ${sdk.join(', ')}.',
  );
  if (failed > 0) {
    stdout.writeln(
      'ĐỎ: $failed gói không đạt chính sách giấy phép. Gỡ gói, hoặc hỏi người sở hữu dự án '
      'rồi ghi ngoại lệ có lý do vào ${policyFile.path}.',
    );
    return 1;
  }
  stdout.writeln('XANH: mọi gói pub đạt chính sách giấy phép.');
  return 0;
}
