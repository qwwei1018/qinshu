@echo off
chcp 65001 >nul
cd /d "%~dp0"
echo ============================================================
echo   Qin书输入法 - 一键下载并导入十万级开源五笔86 + 拼音大词库
echo   数据源：开源 Rime (rime-wubi / rime-luna-pinyin)
echo ============================================================
if not exist "tmp_dict" mkdir "tmp_dict"

echo [1/4] 正在下载开源五笔86大词库 (wubi86.dict.yaml)...
powershell -NoProfile -Command ^
  "$urls = @('https://cdn.jsdelivr.net/gh/rime/rime-wubi@master/wubi86.dict.yaml','https://raw.githubusercontent.com/rime/rime-wubi/master/wubi86.dict.yaml');" ^
  "$ok = $false; foreach ($u in $urls) { try { Invoke-WebRequest -Uri $u -OutFile 'tmp_dict\wubi86.dict.yaml' -TimeoutSec 30; $ok = $true; break } catch {} };" ^
  "if (-not $ok) { exit 1 }"
if %errorlevel% neq 0 (
    echo [提示] 五笔大词库下载失败，请检查网络或代理后重试。
    pause
    exit /b 1
)

echo [2/4] 正在下载开源朙月拼音大词库 (luna_pinyin.dict.yaml)...
powershell -NoProfile -Command ^
  "$urls = @('https://cdn.jsdelivr.net/gh/rime/rime-luna-pinyin@master/luna_pinyin.dict.yaml','https://raw.githubusercontent.com/rime/rime-luna-pinyin/master/luna_pinyin.dict.yaml');" ^
  "$ok = $false; foreach ($u in $urls) { try { Invoke-WebRequest -Uri $u -OutFile 'tmp_dict\luna_pinyin.dict.yaml' -TimeoutSec 30; $ok = $true; break } catch {} };" ^
  "if (-not $ok) { exit 1 }"
if %errorlevel% neq 0 (
    echo [提示] 拼音大词库下载失败，请检查网络或代理后重试。
    pause
    exit /b 1
)

echo [3/4] 正在转换并导入五笔86大词库至 data\wubi86_extra.txt ...
del /f /q "data\wubi86_extra.txt" >nul 2>nul
go run ./cmd/qinshu-cli -import-kind wubi -import-file "tmp_dict\wubi86.dict.yaml"

echo [4/4] 正在转换并导入拼音大词库至 data\pinyin_extra.txt ...
del /f /q "data\pinyin_extra.txt" >nul 2>nul
go run ./cmd/qinshu-cli -import-kind pinyin -import-file "tmp_dict\luna_pinyin.dict.yaml"

rmdir /s /q "tmp_dict" >nul 2>nul
echo.
echo 导入完成！如 Qin书 正在运行，右键点击状态栏选择「重新加载词库与短语」即可立即生效。
pause
