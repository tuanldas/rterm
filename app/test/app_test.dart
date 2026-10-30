import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:rterm/app.dart';
import 'package:rterm/core/product.dart';
import 'package:rterm/l10n/app_localizations.dart';

void main() {
  testWidgets('man hinh dau hien chuoi tieng Viet lay tu ARB', (tester) async {
    await tester.pumpWidget(const App());
    await tester.pumpAndSettle();

    final context = tester.element(find.byType(Scaffold));
    expect(Localizations.localeOf(context).languageCode, 'vi');

    final l10n = lookupAppLocalizations(const Locale('vi'));
    expect(find.text(l10n.homeWelcome(productName)), findsOneWidget);
    expect(find.text(l10n.homeNoServer), findsOneWidget);
  });
}
