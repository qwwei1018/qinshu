@echo off
chcp 65001 >nul
cd /d "%~dp0"
echo ============================================
echo   Qin书输入法 (Go 原生版) 嵌入图标编译并启动
echo ============================================

echo [1/3] 正在运行 Go 单元测试...
go test ./...
if %errorlevel% neq 0 (
    echo [错误] 单元测试未通过，请检查 Go 环境。
    pause
    exit /b 1
)

echo [2/3] 正在嵌入 Windows 11 程序图标 (assets\qinshu.ico)...
if exist "assets\qinshu.ico" (
    where rsrc >nul 2>nul
    if %errorlevel% neq 0 (
        echo 尝试安装 rsrc 图标资源工具（离线时将自动跳过，不影响编译）...
        go install github.com/akavel/rsrc@latest >nul 2>nul
    )
    where rsrc >nul 2>nul
    if %errorlevel%==0 (
        rsrc -arch amd64 -ico "assets\qinshu.ico" -o "cmd\qinshu\rsrc_windows_amd64.syso"
        echo 已生成图标资源 cmd\qinshu\rsrc_windows_amd64.syso
    ) else (
        where windres >nul 2>nul
        if %errorlevel%==0 (
            windres -i "cmd\qinshu\qinshu.rc" -o "cmd\qinshu\rsrc_windows_amd64.syso" -O coff
            echo 已通过 windres 生成图标资源。
        ) else (
            echo 未检测到 rsrc/windres，将跳过 EXE 静态图标嵌入（运行时仍自动加载 assets\qinshu.ico）。
        )
    )
)

echo [3/3] 正在编译 Qin书.exe ...
go build -ldflags="-H windowsgui -s -w" -o "Qin书.exe" ./cmd/qinshu
if %errorlevel% neq 0 (
    echo [错误] 编译失败。
    pause
    exit /b 1
)

echo 编译成功！正在启动 Qin书.exe ...
start "" "Qin书.exe"
