#pragma once

#include <windows.h>
#include <guiddef.h>

// Qin书 TSF COM 类唯一标识 (CLSID_QinShuTSF)
// {7D4A9C1E-5B2F-4E83-A1C6-8F3D2E9B4A10}
static const CLSID CLSID_QinShuTSF = {
    0x7d4a9c1e, 0x5b2f, 0x4e83, {0xa1, 0xc6, 0x8f, 0x3d, 0x2e, 0x9b, 0x4a, 0x10}
};

// Qin书 TSF 语言配置唯一标识 (GUID_QinShuProfile)
// {8E5B0D2F-6C3A-4F94-B2D7-9A4E3F0C5B21}
static const GUID GUID_QinShuProfile = {
    0x8e5b0d2f, 0x6c3a, 0x4f94, {0xb2, 0xd7, 0x9a, 0x4e, 0x3f, 0x0c, 0x5b, 0x21}
};

// 显式定义 Windows 10/11 现代应用 (UWP/Immersive) 与系统托盘能力类别 GUID，兼容所有版本 Windows SDK
// GUID_TFCAT_TIPCAP_IMMERSIVESUPPORT {13A016DF-560B-46CD-947A-4C3AF1E0E35D}
static const GUID QINSHU_GUID_TFCAT_TIPCAP_IMMERSIVESUPPORT = {
    0x13a016df, 0x560b, 0x46cd, {0x94, 0x7a, 0x4c, 0x3a, 0xf1, 0xe0, 0xe3, 0x5d}
};

// GUID_TFCAT_TIPCAP_SYSTRAYSUPPORT {25504FB4-7BAB-4BC1-9C69-CF81890F0EF5}
static const GUID QINSHU_GUID_TFCAT_TIPCAP_SYSTRAYSUPPORT = {
    0x25504fb4, 0x7bab, 0x4bc1, {0x9c, 0x69, 0xcf, 0x81, 0x89, 0x0f, 0x0e, 0xf5}
};

// 简体中文 (zh-CN) 语言 ID
static const LANGID QINSHU_LANGID = MAKELANGID(LANG_CHINESE, SUBLANG_CHINESE_SIMPLIFIED);

// 命名管道路径
static const wchar_t* QINSHU_PIPE_NAME = L"\\\\.\\pipe\\QinShuIME";
