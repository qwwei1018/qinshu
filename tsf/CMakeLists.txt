cmake_minimum_required(VERSION 3.20)
project(QinShuTSF LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# 静态链接 CRT (/MT) + 体积优化 (/Os)，零外部 VC++ 运行库依赖，DLL 仅 ~150KB
if(MSVC)
    set(CMAKE_MSVC_RUNTIME_LIBRARY "MultiThreaded")
    add_compile_options(/Os /GS- /W3 /utf-8)
    add_link_options(/OPT:REF /OPT:ICF)
endif()

add_library(QinShuTSF SHARED
    dllmain.cpp
    text_service.cpp
    pipe_client.cpp
    QinShuTSF.def
)

target_compile_definitions(QinShuTSF PRIVATE UNICODE _UNICODE WIN32_LEAN_AND_MEAN)
# 必须链接 uuid（包含全部标准 TSF IID/CLSID 符号）
target_link_libraries(QinShuTSF PRIVATE ole32 advapi32 shlwapi user32 uuid)
