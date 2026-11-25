#pragma once

#include <windows.h>
#include <string>

enum class PipeAction {
    PassThrough, // 放行给目标应用
    Eaten,       // 吞掉按键（候选框已由 Go 服务端刷新）
    Commit       // 吞掉按键并通过 ITfEditSession 原生上屏
};

struct PipeResponse {
    PipeAction action = PipeAction::PassThrough;
    std::wstring commitText;
};

// 向后台轻量 Go 引擎发送单行指令并获取响应（严格 15ms 超时，绝不卡死任何宿主程序）。
PipeResponse SendPipeCommand(const std::string& line);

// 将单个按键事件格式化为轻量文本指令并发送。
PipeResponse SendPipeKeyEvent(UINT vk, wchar_t ch, bool ctrl, bool alt, bool shift, LONG x, LONG y);

// 通知 Shift 单击切换中英文。
PipeResponse SendPipeShiftTap(LONG x, LONG y);

// 通知焦点变化（切出窗口时立即隐藏候选窗）。
void SendPipeFocus(bool focused);
