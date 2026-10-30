import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:rterm/core/product.dart';

/// Lấy nhóm 1 của [pattern] trong tệp [path]; tệp phải có đúng một chỗ khớp
/// để khai báo trùng lặp cũng lộ ra.
String _declaredIn(String path, RegExp pattern) {
  final matches = pattern.allMatches(File(path).readAsStringSync()).toList();
  if (matches.length != 1) {
    fail(
      '$path: cần đúng một chỗ khớp ${pattern.pattern}, thấy ${matches.length}',
    );
  }
  return matches.single.group(1)!;
}

void main() {
  test('hang san pham khop product.json', () {
    // flutter test chạy ở thư mục app/, product.json nằm ở gốc repo.
    final doc = jsonDecode(File('../product.json').readAsStringSync());
    if (doc is! Map<String, Object?>) {
      fail('product.json phải là một đối tượng JSON');
    }

    // Mỗi dòng: khoá của product.json, chỗ khai báo trong app, giá trị ở đó.
    // Android bắt buộc khai báo applicationId ở Gradle và nhãn app ở manifest,
    // nên hai chỗ này cũng phải khớp dù không phải mã Dart.
    final declared = <(String, String, Object?)>[
      ('name', 'lib/core/product.dart productName', productName),
      ('app_id', 'lib/core/product.dart appId', appId),
      ('url_scheme', 'lib/core/product.dart urlScheme', urlScheme),
      (
        'app_id',
        'android/app/build.gradle.kts applicationId',
        _declaredIn(
          'android/app/build.gradle.kts',
          RegExp(r'applicationId\s*=\s*"([^"]*)"'),
        ),
      ),
      (
        'name',
        'android/app/src/main/AndroidManifest.xml android:label',
        _declaredIn(
          'android/app/src/main/AndroidManifest.xml',
          RegExp(r'<application[^>]*\sandroid:label="([^"]*)"'),
        ),
      ),
    ];

    // Gom mọi chỗ lệch rồi báo một lần để người đổi tên thấy hết chỗ sót.
    final mismatches = [
      for (final (key, where, value) in declared)
        if (!doc.containsKey(key))
          'product.json thiếu khoá "$key" (app khai báo ở $where)'
        else if (doc[key] != value)
          '$where = ${jsonEncode(value)} nhưng product.json "$key" = '
              '${jsonEncode(doc[key])}',
    ];
    expect(mismatches, isEmpty, reason: mismatches.join('\n'));
  });
}
