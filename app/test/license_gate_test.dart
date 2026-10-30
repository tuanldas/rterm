import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

import '../../scripts/check_dart_licenses.dart' as gate;

// Logic của cổng giấy phép pub (scripts/check_dart_licenses.dart). Cổng chạy
// trong CI của app; test nằm ở đây để chạy cùng `flutter test`.

const _mit = '''
Copyright (c) 2024 Ai đó

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.
''';

const _bsd3 = '''
Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:
    * Redistributions of source code must retain the above copyright notice.
    * Neither the name of Google LLC nor the names of its contributors may
      be used to endorse or promote products derived from this software
      without specific prior written permission.
''';

const _bsd2 = '''
Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:
1. Redistributions of source code must retain the above copyright notice.
2. Redistributions in binary form must reproduce the above copyright notice.
''';

const _apache = '''
                                 Apache License
                           Version 2.0, January 2004
''';

const _isc = '''
Permission to use, copy, modify, and/or distribute this software for any
purpose with or without fee is hereby granted, provided that the above
copyright notice and this permission notice appear in all copies.
''';

const _zlib = '''
This software is provided ‘as-is’, without any express or implied
warranty. Permission is granted to anyone to use this software for any purpose,
including commercial applications, and to alter it and redistribute it freely.
''';

// Trích đúng các câu của GPL-3.0 nhắc tới AGPL và LGPL.
const _gpl3 = '''
                    GNU GENERAL PUBLIC LICENSE
                       Version 3, 29 June 2007
  13. Use with the GNU Affero General Public License.
  ... combine any covered work with a work licensed under version 3
of the GNU Affero General Public License into a single combined work ...
use the GNU Lesser General Public License instead of this License.
''';

// LGPL-3.0 nói rõ nó dựa trên GPL-3.0.
const _lgpl3 = '''
                   GNU LESSER GENERAL PUBLIC LICENSE
                       Version 3, 29 June 2007
  This version of the GNU Lesser General Public License incorporates
the terms and conditions of version 3 of the GNU General Public
License, supplemented by the additional permissions listed below.
''';

// MPL-2.0 liệt kê cả ba giấy phép GNU làm "Secondary License".
const _mpl2 = '''
Mozilla Public License Version 2.0
1.12. "Secondary License"
    means either the GNU General Public License, Version 2.0, the GNU Lesser
    General Public License, Version 2.1, the GNU Affero General Public
    License, Version 3.0, or any later versions of those licenses.
''';

const _gplNotice = '''
This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
''';

const _policy = '''
# chính sách thử
allow MIT
allow BSD-3-Clause
deny GPL-*
deny AGPL-*
exception pub dev_only MPL-2.0 dev Chỉ dùng khi kiểm thử.
exception pub any_scope LGPL-3.0 runtime Đã hỏi người dùng.
exception pub unknown_text MIT runtime Văn bản MIT sửa lời, đã đọc tay.
exception pub gpl_pkg GPL-3.0 runtime Thử ghi đè lệnh cấm.
exception go dev_only MIT runtime Ngoại lệ của hệ khác không áp cho pub.
''';

void main() {
  test('nhan dien giay phep tu van ban chuan', () {
    expect(gate.detectLicenses(_mit), {'MIT'});
    expect(gate.detectLicenses(_bsd3), {'BSD-3-Clause'});
    expect(gate.detectLicenses(_bsd2), {'BSD-2-Clause'});
    expect(gate.detectLicenses(_apache), {'Apache-2.0'});
    expect(gate.detectLicenses(_isc), {'ISC'});
    expect(gate.detectLicenses(_zlib), {'Zlib'});
    expect(gate.detectLicenses(_gplNotice), {'GPL-3.0'});
    expect(gate.detectLicenses('Bản quyền thuộc về tác giả.'), isEmpty);
    // Hai tệp giấy phép của cùng một gói được nối lại: cả hai đều phải đạt.
    expect(gate.detectLicenses('$_mit\n$_gpl3'), {'MIT', 'GPL-3.0'});
  });

  test('van ban GNU va MPL nhac ten nhau khong lam nhan nham', () {
    expect(gate.detectLicenses(_gpl3), {'GPL-3.0'});
    expect(gate.detectLicenses(_lgpl3), {'LGPL-3.0'});
    expect(gate.detectLicenses(_mpl2), {'MPL-2.0'});
  });

  group('xep loai theo chinh sach', () {
    final policy = gate.parsePolicy(_policy);
    gate.Verdict verdict(String pkg, Set<String> licenses, gate.Scope scope) =>
        gate.classify(policy, pkg, licenses, scope).verdict;

    test('cong chan giay phep cam va giay phep ngoai danh sach', () {
      expect(verdict('a', {'MIT'}, gate.Scope.runtime), gate.Verdict.allow);
      expect(verdict('a', {'GPL-3.0'}, gate.Scope.dev), gate.Verdict.deny);
      expect(verdict('a', {'AGPL-3.0'}, gate.Scope.dev), gate.Verdict.deny);
      expect(verdict('a', {'LGPL-2.1'}, gate.Scope.dev), gate.Verdict.ask);
      expect(verdict('a', {}, gate.Scope.dev), gate.Verdict.unknown);
      // Nhiều giấy phép là AND: một cái bị cấm thì cả gói bị cấm.
      expect(
        verdict('a', {'MIT', 'GPL-3.0'}, gate.Scope.dev),
        gate.Verdict.deny,
      );
      expect(
        verdict('a', {'MIT', 'BSD-3-Clause'}, gate.Scope.dev),
        gate.Verdict.allow,
      );
    });

    test('ngoai le chi ap dung dung goi, dung ma va dung pham vi', () {
      expect(
        verdict('dev_only', {'MPL-2.0'}, gate.Scope.dev),
        gate.Verdict.exception,
      );
      expect(
        verdict('dev_only', {'MPL-2.0'}, gate.Scope.runtime),
        gate.Verdict.ask,
      );
      expect(
        verdict('any_scope', {'LGPL-3.0'}, gate.Scope.runtime),
        gate.Verdict.exception,
      );
      expect(
        verdict('any_scope', {'LGPL-3.0'}, gate.Scope.dev),
        gate.Verdict.exception,
      );
      // Gói đổi sang giấy phép khác thì ngoại lệ cũ không còn che.
      expect(
        verdict('any_scope', {'MPL-2.0'}, gate.Scope.dev),
        gate.Verdict.ask,
      );
      expect(verdict('khac', {'MPL-2.0'}, gate.Scope.dev), gate.Verdict.ask);
      // Văn bản không nhận diện được: ngoại lệ ghi tay là kết luận đã đọc.
      expect(
        verdict('unknown_text', {}, gate.Scope.runtime),
        gate.Verdict.exception,
      );
      expect(
        gate
            .classify(policy, 'dev_only', {'MPL-2.0'}, gate.Scope.runtime)
            .notes
            .join(),
        contains('phạm vi dev'),
      );
    });

    test('ngoai le khong go duoc lenh cam', () {
      expect(
        verdict('gpl_pkg', {'GPL-3.0'}, gate.Scope.runtime),
        gate.Verdict.deny,
      );
    });

    test('lenh cam khong phan biet hoa thuong', () {
      final p = gate.parsePolicy(
        '${_policy}deny sspl-1.0\n'
        'exception pub lower_case gpl-3.0-only runtime Mã cấm viết thường.\n',
      );
      gate.Verdict of(String pkg, Set<String> licenses) =>
          gate.classify(p, pkg, licenses, gate.Scope.runtime).verdict;
      // Mã viết thường khớp mục cấm theo tiền tố.
      expect(of('a', {'gpl-3.0'}), gate.Verdict.deny);
      // Mục cấm viết thường khớp mã viết hoa.
      expect(of('a', {'SSPL-1.0'}), gate.Verdict.deny);
      // Ngoại lệ ghi mã cấm viết thường không gỡ được gói không rõ giấy phép.
      expect(of('lower_case', {}), gate.Verdict.deny);
    });

    test('ngoai le ghi ma cam thang, khong tuy thu tu dong', () {
      const allowed = 'exception pub two MIT runtime Đã đọc văn bản.\n';
      const banned = 'exception pub two GPL-3.0 runtime Thử ghi đè lệnh cấm.\n';
      for (final extra in ['$allowed$banned', '$banned$allowed']) {
        final p = gate.parsePolicy('$_policy$extra');
        for (final scope in gate.Scope.values) {
          expect(
            gate.classify(p, 'two', {}, scope).verdict,
            gate.Verdict.deny,
            reason: '$scope:\n$extra',
          );
        }
      }
    });

    test('giay phep cam nhan ra duoc thang ngoai le cho phan khong ro', () {
      final p = gate.parsePolicy(
        '${_policy}exception pub dual MIT runtime Phần không rõ đã đọc tay.\n',
      );
      // Tệp GPL nhận ra được nối với một tệp văn bản riêng không nhận ra.
      final detected = gate.detectLicenses(
        '$_gpl3\nBản quyền riêng của tác giả.',
      );
      for (final scope in gate.Scope.values) {
        for (final licenses in [
          detected,
          {'GPL-3.0', gate.unknownLicense},
        ]) {
          expect(
            gate.classify(p, 'dual', licenses, scope).verdict,
            gate.Verdict.deny,
            reason: '$scope: $licenses',
          );
        }
      }
      // Đối chứng: cùng ngoại lệ đó gỡ được phần không rõ khi phần nhận ra
      // được là dễ dãi.
      expect(
        gate.classify(p, 'dual', {
          'MIT',
          gate.unknownLicense,
        }, gate.Scope.runtime).verdict,
        gate.Verdict.exception,
      );
    });
  });

  test('chinh sach that doc duoc, dong sai cu phap bi bao loi', () {
    final real = gate.parsePolicy(
      File('../scripts/license_policy.txt').readAsStringSync(),
    );
    expect(real.verdictOf('MIT'), gate.Verdict.allow);
    expect(real.verdictOf('GPL-3.0'), gate.Verdict.deny);
    expect(real.verdictOf('AGPL-3.0'), gate.Verdict.deny);
    expect(real.verdictOf('LGPL-3.0'), gate.Verdict.ask);

    for (final bad in [
      'allow',
      'permit MIT',
      'deny GPL-* AGPL-*',
      'exception pub foo MIT runtime',
      'exception npm foo MIT runtime lý do',
      'exception pub foo MIT test lý do',
    ]) {
      expect(
        () => gate.parsePolicy('allow MIT\n$bad'),
        throwsA(isA<gate.GateException>()),
        reason: bad,
      );
    }
  });

  test('goi chi den qua dev_dependencies duoc xep pham vi dev', () {
    final runtime = gate.runtimeClosure({
      'roots': ['app'],
      'packages': [
        {
          'name': 'app',
          'dependencies': ['flutter', 'intl'],
          'devDependencies': ['flutter_test'],
        },
        {
          'name': 'flutter',
          'dependencies': ['collection'],
        },
        {
          'name': 'intl',
          'dependencies': ['clock'],
        },
        {
          'name': 'flutter_test',
          'dependencies': ['flutter', 'matcher'],
        },
        {'name': 'collection', 'dependencies': <String>[]},
        {'name': 'clock', 'dependencies': <String>[]},
        {'name': 'matcher', 'dependencies': <String>[]},
      ],
    });
    expect(runtime, {'flutter', 'intl', 'collection', 'clock'});
  });

  test('doc nguon va phien ban goi tu pubspec.lock', () {
    const lock = '''
# Generated by pub
packages:
  async:
    dependency: transitive
    description:
      name: async
      sha256: "abc"
      url: "https://pub.dev"
    source: hosted
    version: "2.13.1"
  flutter:
    dependency: "direct main"
    description: flutter
    source: sdk
    version: "0.0.0"
sdks:
  dart: ">=3.13.7 <4.0.0"
''';
    final parsed = gate.parsePubspecLock(lock);
    expect(parsed.keys, ['async', 'flutter']);
    expect(parsed['async'], (source: 'hosted', version: '2.13.1'));
    expect(parsed['flutter']!.source, 'sdk');
  });
}
