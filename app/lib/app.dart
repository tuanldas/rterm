import 'package:flutter/material.dart';

import 'core/product.dart';
import 'l10n/app_localizations.dart';

/// Gốc widget của app: nạp bản địa hoá rồi mở màn hình đầu tiên.
class App extends StatelessWidget {
  const App({super.key});

  @override
  Widget build(BuildContext context) {
    return const MaterialApp(
      // Tên sản phẩm không dịch nên lấy từ product.dart, không qua ARB.
      title: productName,
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: _HomeScreen(),
    );
  }
}

/// Màn hình đầu tiên khi app chưa có máy chủ nào.
class _HomeScreen extends StatelessWidget {
  const _HomeScreen();

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context);
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: const Text(productName)),
      body: Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                l10n.homeWelcome(productName),
                style: theme.textTheme.headlineSmall,
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 12),
              Text(l10n.homeNoServer, textAlign: TextAlign.center),
            ],
          ),
        ),
      ),
    );
  }
}
