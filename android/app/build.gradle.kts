import java.io.File

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "com.qinshu.ime"
    compileSdk = 35

    defaultConfig {
        applicationId = "com.qinshu.ime"
        minSdk = 24
        targetSdk = 35
        versionCode = (System.getenv("GITHUB_RUN_NUMBER")?.toIntOrNull() ?: 3)
        versionName = System.getenv("APP_VERSION") ?: "1.1.1"
    }

    // 关键低内存配置：禁止压缩 qdict，确保 Android 直接零解压 FileChannel.map(READ_ONLY)
    androidResources {
        noCompress += listOf("qdict")
    }

    signingConfigs {
        create("release") {
            val ksPath = System.getenv("KEYSTORE_PATH")
            if (!ksPath.isNullOrBlank() && File(ksPath).exists()) {
                storeFile = File(ksPath)
                storePassword = System.getenv("KEYSTORE_PASSWORD") ?: ""
                keyAlias = System.getenv("KEY_ALIAS") ?: ""
                keyPassword = System.getenv("KEY_PASSWORD") ?: ""
            }
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro",
            )
            val ksPath = System.getenv("KEYSTORE_PATH")
            signingConfig = if (!ksPath.isNullOrBlank() && File(ksPath).exists()) {
                signingConfigs.getByName("release")
            } else {
                signingConfigs.getByName("debug")
            }
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions {
        jvmTarget = "17"
    }
}

dependencies {
    // 零重型 UI/框架依赖：仅保留 Kotlin 标准库与单元测试依赖，实现极致低内存与小体积 APK
    testImplementation("junit:junit:4.13.2")
}
