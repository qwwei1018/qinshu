@echo off
chcp 65001 >nul
cd /d "%~dp0"

:: 自动请求管理员权限（注册 Win11 系统语言栏 TSF COM 组件所需）
net session >nul 2>&1
if %errorlevel% neq 0 (
    powershell -NoProfile -Command "Start-Process '%~f0' -Verb RunAs"
    exit /b 0
)

set "INSTALL_DIR=C:\Program Files\QinShu"
echo ============================================================
echo   Qin书输入法 (Win11 原生 TSF 语言栏版) 一键安装与注册
echo ============================================================

if not exist "QinShuTSF_x64.dll" (
    echo [提示] 当前目录未找到 QinShuTSF_x64.dll。
    echo 请先通过 GitHub Actions 下载编译包，或在本机装有 MSVC/CMake 时编译 tsf\ 目录。
    pause
    exit /b 1
)
if not exist "Qin书.exe" (
    echo [提示] 当前目录未找到 Qin书.exe，正在尝试使用本地 Go 编译...
    go build -ldflags="-H windowsgui -s -w" -o "Qin书.exe" ./cmd/qinshu
    if %errorlevel% neq 0 (
        echo [错误] 编译 Qin书.exe 失败。
        pause
        exit /b 1
    )
)

echo [1/4] 停止旧进程并复制文件至 %INSTALL_DIR% ...
taskkill /f /im "Qin书.exe" >nul 2>&1
if not exist "%INSTALL_DIR%" mkdir "%INSTALL_DIR%"
copy /y "Qin书.exe" "%INSTALL_DIR%\" >nul
copy /y "QinShuTSF_x64.dll" "%INSTALL_DIR%\" >nul
if exist "QinShuTSF_x86.dll" copy /y "QinShuTSF_x86.dll" "%INSTALL_DIR%\" >nul
xcopy /e /i /y "data" "%INSTALL_DIR%\data" >nul
if exist "assets" xcopy /e /i /y "assets" "%INSTALL_DIR%\assets" >nul

echo [2/4] 注册 Windows 11 原生 TSF 输入法 (x64 + 可选 x86)...
"%SystemRoot%\System32\regsvr32.exe" /s "%INSTALL_DIR%\QinShuTSF_x64.dll"
if exist "%INSTALL_DIR%\QinShuTSF_x86.dll" (
    "%SystemRoot%\SysWOW64\regsvr32.exe" /s "%INSTALL_DIR%\QinShuTSF_x86.dll"
)

echo [3/4] 配置轻量 Go 后端开机自启 (Qin书.exe --tsf)...
powershell -NoProfile -Command ^
  "$ws = New-Object -ComObject WScript.Shell;" ^
  "$s = $ws.CreateShortcut([Environment]::GetFolderPath('Startup') + '\Qin书输入法.lnk');" ^
  "$s.TargetPath = '%INSTALL_DIR%\Qin书.exe';" ^
  "$s.Arguments = '--tsf';" ^
  "$s.WorkingDirectory = '%INSTALL_DIR%';" ^
  "if (Test-Path '%INSTALL_DIR%\assets\qinshu.ico') { $s.IconLocation = '%INSTALL_DIR%\assets\qinshu.ico' };" ^
  "$s.Save()"

echo [4/4] 启动 Qin书 轻量 Go 后端服务...
start "" "%INSTALL_DIR%\Qin书.exe" --tsf

echo.
echo 安装完成！请按 Win + Space 在系统语言栏切换到「Qin书」开始使用。
pause
