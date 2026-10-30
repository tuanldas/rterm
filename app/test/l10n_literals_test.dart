import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

// Mọi chuỗi hiển thị phải đi qua ARB (lib/l10n/app_vi.arb). Test quét mã
// nguồn trong lib/ tìm literal chuỗi đưa thẳng vào widget hiển thị chữ.

/// Widget nhận chuỗi hiển thị ở đối số vị trí đầu tiên.
final _positional = RegExp(r'\b(?:Text|SelectableText)\s*\(\s*');

/// Tham số có tên mang chuỗi hiển thị (tiêu đề, nhãn, gợi ý, chú giải…).
final _named = RegExp(
  r'\b(?:title|label|labelText|hintText|helperText|errorText|counterText|'
  r'prefixText|suffixText|semanticsLabel|tooltip|message|text)\s*:\s*',
);

/// Chuỗi và chú thích của Dart, khớp theo thứ tự xuất hiện để `//` nằm trong
/// chuỗi (như URL) không bị coi là chú thích.
final _stringOrComment = RegExp(
  r"""r?'''[\s\S]*?'''|r?\"\"\"[\s\S]*?\"\"\"|r?'(?:\\.|[^'\\\n])*'|"""
  r'''r?"(?:\\.|[^"\\\n])*"|//[^\n]*|/\*[\s\S]*?\*/''',
);

final _stringLiteral = RegExp(r"""^(r?)('''|\"\"\"|'|")([\s\S]*?)\2""");
final _interpolation = RegExp(r'\$(?:\{[^}]*\}|[A-Za-z_][A-Za-z0-9_]*)');
final _letter = RegExp(r'\p{L}', unicode: true);

/// Một literal chuỗi có chữ cái nằm ở chỗ hiển thị.
typedef Hardcoded = ({int line, String literal});

/// Thay chú thích bằng khoảng trắng (giữ số dòng), chuỗi giữ nguyên.
String _blankComments(String source) =>
    source.replaceAllMapped(_stringOrComment, (m) {
      final token = m[0]!;
      return token.startsWith('//') || token.startsWith('/*')
          ? token.replaceAll(RegExp(r'[^\n]'), ' ')
          : token;
    });

/// Tìm literal chuỗi viết thẳng vào chỗ hiển thị trong [source]. Chuỗi không
/// có chữ cái (dấu câu, số, chỉ có biến nội suy) không cần dịch nên bỏ qua.
List<Hardcoded> findHardcodedDisplayStrings(String source) {
  final code = _blankComments(source);
  final found = <Hardcoded>[];
  for (final pattern in [_positional, _named]) {
    for (final m in pattern.allMatches(code)) {
      final literal = _stringLiteral.firstMatch(code.substring(m.end));
      if (literal == null) {
        continue;
      }
      final raw = literal[1]!.isNotEmpty;
      final body = raw
          ? literal[3]!
          : literal[3]!.replaceAll(_interpolation, '');
      if (_letter.hasMatch(body)) {
        final line = '\n'.allMatches(code.substring(0, m.start)).length + 1;
        found.add((line: line, literal: literal[0]!));
      }
    }
  }
  return found;
}

void main() {
  test('chuoi hien thi khong viet cung', () {
    final files = Directory('lib')
        .listSync(recursive: true)
        .whereType<File>()
        .where((f) => f.path.endsWith('.dart'))
        // AppLocalizations sinh từ ARB chính là nơi chứa chuỗi.
        .where((f) => !f.uri.pathSegments.last.startsWith('app_localizations'))
        .toList();
    expect(files, isNotEmpty, reason: 'không thấy mã nguồn trong lib/');

    final violations = [
      for (final f in files)
        for (final v in findHardcodedDisplayStrings(f.readAsStringSync()))
          '${f.path}:${v.line}: ${v.literal}',
    ];
    expect(
      violations,
      isEmpty,
      reason:
          'chuỗi hiển thị viết cứng; thêm khoá vào lib/l10n/app_vi.arb rồi '
          'dùng AppLocalizations.of(context):\n${violations.join('\n')}',
    );
  });

  group('bo quet bat duoc chuoi hien thi viet cung', () {
    final cases = <String, (String, int)>{
      'Text thuong': ("Text('Xin chào')", 1),
      'const Text, nhay kep': ('const Text("Đăng nhập")', 1),
      'xuong dong sau ngoac': ("Text(\n  'Kết nối lại',\n)", 1),
      'chuoi tho': ("Text(r'Lỗi')", 1),
      'ba dau nhay': ("Text('''Nhiều\ndòng''')", 1),
      'SelectableText': ("SelectableText('Mã khôi phục')", 1),
      'tham so co ten': ("InputDecoration(hintText: 'Email')", 1),
      'TextSpan': ("TextSpan(text: 'Xem thêm')", 1),
      'noi suy kem chu': (r"Text('Xin chào $name')", 1),
      'chi co noi suy': (r"Text('$count')", 0),
      'chi co dau cau': ("Text(' · ')", 0),
      'lay tu ARB': ('Text(l10n.homeNoServer)', 0),
      'trong chu thich dong': ("// Text('Xin chào')\nfinal a = 1;", 0),
      'trong chu thich khoi': ("/* Text('Xin chào') */", 0),
      'URL truoc Text tren cung dong': ("f('http://x'); Text('Chào')", 1),
      'thong bao loi khong phai cho hien thi': ("throw StateError('hỏng')", 0),
    };
    cases.forEach((name, c) {
      test(name, () {
        expect(findHardcodedDisplayStrings(c.$1), hasLength(c.$2));
      });
    });
  });
}
