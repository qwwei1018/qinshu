#include "pipe_client.h"
#include "guids.h"

#include <cstdio>
#include <vector>

namespace {

int HexVal(char c) {
    if (c >= '0' && c <= '9') return c - '0';
    if (c >= 'a' && c <= 'f') return c - 'a' + 10;
    if (c >= 'A' && c <= 'F') return c - 'A' + 10;
    return -1;
}

std::wstring DecodeHexUtf8(const std::string& hex) {
    if (hex.size() < 2 || (hex.size() % 2) != 0) {
        return L"";
    }
    std::string utf8;
    utf8.reserve(hex.size() / 2);
    for (size_t i = 0; i < hex.size(); i += 2) {
        int hi = HexVal(hex[i]);
        int lo = HexVal(hex[i + 1]);
        if (hi < 0 || lo < 0) {
            return L"";
        }
        utf8.push_back(static_cast<char>((hi << 4) | lo));
    }
    int wlen = MultiByteToWideChar(CP_UTF8, 0, utf8.data(), static_cast<int>(utf8.size()), nullptr, 0);
    if (wlen <= 0) {
        return L"";
    }
    std::wstring out(static_cast<size_t>(wlen), L'\0');
    MultiByteToWideChar(CP_UTF8, 0, utf8.data(), static_cast<int>(utf8.size()), &out[0], wlen);
    return out;
}

} // namespace

PipeResponse SendPipeCommand(const std::string& line) {
    PipeResponse resp;

    // 快速等待管道就绪（最多 15ms），后台服务未启动时立即放行不阻塞
    HANDLE hPipe = CreateFileW(
        QINSHU_PIPE_NAME,
        GENERIC_READ | GENERIC_WRITE,
        0,
        nullptr,
        OPEN_EXISTING,
        0,
        nullptr
    );

    if (hPipe == INVALID_HANDLE_VALUE) {
        if (GetLastError() == ERROR_PIPE_BUSY) {
            if (!WaitNamedPipeW(QINSHU_PIPE_NAME, 15)) {
                return resp;
            }
            hPipe = CreateFileW(
                QINSHU_PIPE_NAME,
                GENERIC_READ | GENERIC_WRITE,
                0,
                nullptr,
                OPEN_EXISTING,
                0,
                nullptr
            );
        }
    }
    if (hPipe == INVALID_HANDLE_VALUE) {
        return resp;
    }

    DWORD written = 0;
    if (!WriteFile(hPipe, line.data(), static_cast<DWORD>(line.size()), &written, nullptr)) {
        CloseHandle(hPipe);
        return resp;
    }

    std::string reply;
    char buf[1024];
    DWORD readBytes = 0;
    while (reply.size() < 4096 && ReadFile(hPipe, buf, sizeof(buf), &readBytes, nullptr) && readBytes > 0) {
        reply.append(buf, readBytes);
        if (reply.find('\n') != std::string::npos) {
            break;
        }
    }
    if (!reply.empty()) {
        while (!reply.empty() && (reply.back() == '\n' || reply.back() == '\r')) {
            reply.pop_back();
        }
        if (reply == "E") {
            resp.action = PipeAction::Eaten;
        } else if (reply.size() >= 2 && reply[0] == 'C' && reply[1] == ' ') {
            resp.action = PipeAction::Commit;
            resp.commitText = DecodeHexUtf8(reply.substr(2));
        } else {
            resp.action = PipeAction::PassThrough;
        }
    }

    CloseHandle(hPipe);
    return resp;
}

PipeResponse SendPipeKeyEvent(UINT vk, wchar_t ch, bool ctrl, bool alt, bool shift, LONG x, LONG y) {
    char cmd[128];
    std::snprintf(
        cmd,
        sizeof(cmd),
        "K %u %x %d %d %d %ld %ld\n",
        vk,
        static_cast<unsigned int>(ch),
        ctrl ? 1 : 0,
        alt ? 1 : 0,
        shift ? 1 : 0,
        x,
        y
    );
    return SendPipeCommand(cmd);
}

PipeResponse SendPipeShiftTap(LONG x, LONG y) {
    char cmd[64];
    std::snprintf(cmd, sizeof(cmd), "S %ld %ld\n", x, y);
    return SendPipeCommand(cmd);
}

void SendPipeFocus(bool focused) {
    SendPipeCommand(focused ? "F 1\n" : "F 0\n");
}
