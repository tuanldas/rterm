plugins {
    id("com.android.application")
    // The Flutter Gradle Plugin must be applied after the Android and Kotlin Gradle plugins.
    id("dev.flutter.flutter-gradle-plugin")
}

android {
    namespace = "com.tuanldas.rterm"
    compileSdk = flutter.compileSdkVersion
    ndkVersion = flutter.ndkVersion

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    defaultConfig {
        // Phải khớp appId trong lib/core/product.dart và app_id trong product.json;
        // test/core/product_test.dart đối chiếu.
        applicationId = "com.tuanldas.rterm"
        // Android 8.0 (API 26) là mức thấp nhất app hỗ trợ. Mã đọc mức bảo mật
        // của kho khoá phải rẽ nhánh theo API vì API 26–30 chưa có getSecurityLevel().
        minSdk = 26
        targetSdk = flutter.targetSdkVersion
        versionCode = flutter.versionCode
        versionName = flutter.versionName
    }

    buildTypes {
        release {
            // Bản phát hành thật ký bằng khoá riêng ở bước đóng gói; khoá debug ở
            // đây chỉ để `flutter run --release` chạy được trên máy phát triển.
            signingConfig = signingConfigs.getByName("debug")
        }
    }
}

kotlin {
    compilerOptions {
        jvmTarget = org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17
    }
}

flutter {
    source = "../.."
}
