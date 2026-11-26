# Qin书输入法 (QinShu IME)

面向 Windows 11 的轻量高性能原生输入法，采用 **Win11 原生 TSF COM 前端（C++ x64/x86） + Go 核心引擎与 GDI 高 DPI 悬浮窗** 架构，支持五笔 86、全拼/简拼、五拼混输，兼容传统桌面应用、浏览器及 UWP/AppContainer 现代应用。

## 核心特性

- **标准 86 版五笔大码表**：内置基于开源 Rime `wubi86.dict.yaml` 校对转换的 **94,049 条** 标准 86 五笔码表（含 25 个一级简码、二/三级简码、常用单字与词组），支持四码唯一自动上屏、五码顶字上屏及 `z` 键万能通配反查。
- **十万级简体拼音词库**：内置基于开源 Rime-Ice 雾凇拼音（`8105` 通用规范汉字表 + `base` 高频简体词库）转换的 **128,548 条** 拼音词条，支持全拼、简拼、12 组模糊音、整句动态规划组词与用户选词动态调频/自动造词。
- **五拼混输与效率工具**：
  - 支持五笔 + 拼音混输、纯五笔 86、纯拼音三种模式切换；
  - 支持自定义短语（`data/phrases.txt`）与 `$year`/`$month`/`$day`/`$weekday` 动态变量；
  - 内置快捷指令（`rq` 日期、`sj` 时间、`xq` 星期、`fh` 符号面板）与智能邮箱/网址补全（如 `zxc@`、`http`、`www.`）。
- **原生 Win11 TSF 与低资源占用**：
  - 提供 `QinShuTSF_x64.dll` 与 `QinShuTSF_x86.dll` 静态编译无外部运行库依赖的原生 TSF COM 组件；
  - 命名管道支持 AppContainer 与低完整性标签权限互通，并支持按键自动拉起后端服务；
  - 支持「简约白」「万能蓝」「深邃暗」三套高 DPI 皮肤主题。

## 下载与安装（Releases）

1. 前往本仓库 [Releases](https://github.com/qwwei1018/qinshu/releases) 下载最新版本的压缩包（例如 `QinShu-IME-TSF-Win11-Full-v2.0.2.zip`）并解压。
2. 双击运行 **`安装_Qin书TSF.bat`**（自动复制文件至 `C:\Program Files\QinShu`，注册 x64/x86 TSF COM 组件，配置开机自启并启动后台服务）。
3. 按 **`Win + Space`** 切换到 **「Qin书」** 即可开始输入。
4. 如需卸载，双击运行 **`卸载_Qin书TSF.bat`**。

> [!TIP]
> 解压后也可直接双击 **`Qin书.exe`** 免安装体验（基于全局低级键盘钩子的便携模式）。

## 常用按键与操作

| 按键 / 操作 | 说明 |
| --- | --- |
| `Win + Space` | 在系统语言栏中切换「Qin书」输入法 |
| 单击 `Shift` / `Ctrl + Space` | 切换中文 / 英文输入状态 |
| `Space` / `1-9` | 选择第 1 项或指定序号候选词上屏 |
| `;` / `'` | `;` 选第 2 候选词；纯五笔模式下 `'` 选第 3 候选词 |
| `-` `=` / `,` `.` / `PageUp` `PageDown` | 上下翻页 |
| `Esc` / `Delete` | 清空当前编码缓冲 |
| 右键点击悬浮状态栏 | 切换输入模式、模糊音开关/预设、主题、重新加载词库或编辑自定义短语 |

## 本地编译

- **环境要求**：Go 1.22+；如需自行编译 TSF DLL，需安装 Visual Studio Build Tools（MSVC）与 CMake 3.20+。
- **运行单元测试**：
  ```powershell
  go test -v ./...
  ```
- **编译 Go 主程序**：
  - 直接运行 `仅编译EXE.bat` 或 `编译并运行_Qin书.bat`；
  - 或手动执行：
    ```powershell
    go build -ldflags="-H windowsgui -s -w" -o "Qin书.exe" ./cmd/qinshu
    ```
- **编译原生 TSF DLL（x64 / x86）**：
  ```powershell
  cmake -S tsf -B build-tsf-x64 -A x64
  cmake --build build-tsf-x64 --config Release
  cmake -S tsf -B build-tsf-x86 -A Win32
  cmake --build build-tsf-x86 --config Release
  ```

## 版本记录

- **v2.0.2**
  - 修复 Win11 TSF 模式下在 Chromium/Electron 等应用中单击 `Shift` 无法切换中英文的问题（TSF 激活/焦点同步 + 只读被动 Shift 观察器兜底）；
  - 全面升级英文模式（`En`）：内置 5,019 词高频英文词库支持智能单词补全（`1-9` / `Tab` / 方向键选词，空格流畅连打），并完整支持智能邮箱（`zxc@`）与智能网址（`http:` / `https:` / `www.`）补全；
- **v2.0.1**
  - 替换内置码表为标准 Rime 五笔 86 大码表（94,049 条）并扩充 Rime-Ice 标准简体拼音大词库（128,548 条）；
  - 修复 Windows 11 TSF `OPENCLOSE` / `CONVERSION` 隔间作用域问题、命名管道低完整性权限及 GC 悬空指针问题；
  - 新增 TSF 后端自动拉起机制与安装脚本占用 DLL 替换兜底。

