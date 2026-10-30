# App Android

App Flutter (chỉ Android, minSdk 26). Quy ước chung ở [docs/code-standards.md](../docs/code-standards.md).

| Thư mục | Nội dung |
|---|---|
| `lib/core/product.dart` | tên sản phẩm, app id, URL scheme; khớp `product.json` ở gốc repo |
| `lib/protocol/` | model Dart của hợp đồng ở [docs/protocol](../docs/protocol/README.md) |
| `lib/l10n/app_vi.arb` | mọi chuỗi hiển thị; `AppLocalizations` sinh ra khi `flutter pub get` |

Kiểm tra như CI (`.github/workflows/app.yml`), chạy trong thư mục `app/`:

```sh
flutter pub get --enforce-lockfile
dart format --output=none --set-exit-if-changed .
flutter analyze
flutter test
dart run ../scripts/check_dart_licenses.dart   # cổng giấy phép pub
flutter build apk --debug
```

Thêm phụ thuộc pub thì chạy lại cổng giấy phép; giấy phép ngoài nhóm cho phép phải hỏi người sở hữu dự án trước khi ghi ngoại lệ vào `scripts/license_policy.txt`.
