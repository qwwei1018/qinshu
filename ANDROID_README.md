# Qin书输入法（安卓端）集成与云端打包说明

本交付包可直接解压覆盖至你的桌面版 `Qin书` 仓库根目录（不会覆盖桌面版现有源码与 `build-win11.yml`）。

## 目录结构

- `cmd/dictbuild/`：零依赖 Go 构建期二进制词库编译器，将桌面版 `data/pinyin.txt`、`data/english.txt` 与新增 `data/emoji_pinyin.txt` 编译为只读 `mmap` 二进制词库 `qinshu.qdict`（内置一级常用字/高频词强置顶与生僻字降权）。
- `data/emoji_pinyin.txt`：纯 Unicode Emoji 拼音联想表（无颜文字）。
- `android/`：原生 Kotlin + 单视图 Canvas 自绘低内存安卓输入法工程（零 Compose、零 AppCompat，包含九宫格 T9 / 26 键全键盘 / 数字算式页、实时算式计算 `11+12`→`23`/`11+12=23`、短信通知与剪贴板智能提取验证码/网址/Email、纯 Emoji 面板与桌面 `user_dict.json` 导入）。
- `.github/workflows/build-android.yml`：GitHub Actions 自动构建脚本（`ubuntu-latest`）。

## GitHub 自动打包与发布步骤

1. 提交安卓端代码并推送到 GitHub `main` 分支（如需指定正式发布版本号，可同步打 `android-v1.1.1` 标签）：
   ```bash
   git add cmd/dictbuild data/emoji_pinyin.txt android .github/workflows/build-android.yml ANDROID_README.md .gitignore
   git commit -m "feat(android): add QinShu Android IME v1.1.1 & CI release workflow"
   git tag -a android-v1.1.1 -m "Qin书输入法 Android v1.1.1"
   git push origin main --tags
   ```
2. GitHub Actions 将自动触发 **Build QinShu IME (Android APK)**，约 2–3 分钟后在仓库 **Releases** 与 **Actions Artifacts** 中生成可安装的 APK（如 `QinShu-IME-Android-android-v1.1.1.apk`）。
3. （可选）如需使用自己的正式签名证书，在 GitHub 仓库 `Settings -> Secrets and variables -> Actions` 中添加以下 4 个 Secrets（未配置时默认使用调试签名，仍可直接安装体验）：
   - `ANDROID_KEYSTORE_BASE64`（`.jks` 文件的 Base64 编码）
   - `ANDROID_KEYSTORE_PASSWORD`
   - `ANDROID_KEY_ALIAS`
   - `ANDROID_KEY_PASSWORD`
