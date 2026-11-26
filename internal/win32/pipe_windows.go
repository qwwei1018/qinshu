//go:build windows

package win32

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

var (
	modAdvapi32 = syscall.NewLazyDLL("advapi32.dll")

	procCreateNamedPipeW     = modKernel32.NewProc("CreateNamedPipeW")
	procConnectNamedPipe     = modKernel32.NewProc("ConnectNamedPipe")
	procDisconnectNamedPipe  = modKernel32.NewProc("DisconnectNamedPipe")
	procFlushFileBuffers     = modKernel32.NewProc("FlushFileBuffers")
	procLocalFree            = modKernel32.NewProc("LocalFree")
	procConvertStringSDToSDW = modAdvapi32.NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW")
)

const (
	pipeAccessDuplex       = 0x00000003
	pipeTypeByte           = 0x00000000
	pipeReadModeByte       = 0x00000000
	pipeWait               = 0x00000000
	pipeUnlimitedInstances = 255
	sddlRevision1          = 1
	errPipeConnected       = 535
)

type securityAttributes struct {
	Length             uint32
	SecurityDescriptor uintptr
	InheritHandle      uint32
}

// PipeServerHandler 定义 TSF DLL 经由命名管道发来的按键、Shift 单击与焦点事件处理回调。
type PipeServerHandler struct {
	HasBuffer  func() bool
	OnKey      func(keyName string, ch rune, ctrl, alt, shift bool, x, y int32) (handled bool, commit string)
	OnShiftTap func(x, y int32) (commit string)
	OnFocus    func(focused bool)
}

// StartTSFPipeServer 启动兼容 Windows 11 UWP/AppContainer 的高并发轻量命名管道服务端（\\.\pipe\QinShuIME）。
func StartTSFPipeServer(handler PipeServerHandler) chan struct{} {
	stopCh := make(chan struct{})
	go runPipeAcceptLoop(handler, stopCh)
	return stopCh
}

func buildAppContainerSA() (*securityAttributes, uintptr) {
	// 允许所有桌面进程 (WD) 与 Win11 UWP/Edge/终端 AppContainer 沙箱进程 (AC) 连接管道，
	// 并显式设置低完整性标签 (LW)，防止以管理员身份启动时普通权限应用 (Medium/Low) 连接被拒绝 (ERROR_ACCESS_DENIED)。
	sddl, _ := syscall.UTF16PtrFromString("D:(A;;GA;;;WD)(A;;GA;;;AC)S:(ML;;NW;;;LW)")
	var sd uintptr
	ok, _, _ := procConvertStringSDToSDW.Call(
		uintptr(unsafe.Pointer(sddl)),
		sddlRevision1,
		uintptr(unsafe.Pointer(&sd)),
		0,
	)
	if ok == 0 || sd == 0 {
		return nil, 0
	}
	sa := &securityAttributes{
		Length:             uint32(unsafe.Sizeof(securityAttributes{})),
		SecurityDescriptor: sd,
		InheritHandle:      0,
	}
	return sa, sd
}

func runPipeAcceptLoop(handler PipeServerHandler, stopCh <-chan struct{}) {
	pipeName, _ := syscall.UTF16PtrFromString(`\\.\pipe\QinShuIME`)
	sa, sd := buildAppContainerSA()
	if sd != 0 {
		defer procLocalFree.Call(sd)
	}

	var saPtr uintptr
	if sa != nil {
		saPtr = uintptr(unsafe.Pointer(sa))
	}

	for {
		select {
		case <-stopCh:
			return
		default:
		}

		hPipe, _, _ := procCreateNamedPipeW.Call(
			uintptr(unsafe.Pointer(pipeName)),
			pipeAccessDuplex,
			pipeTypeByte|pipeReadModeByte|pipeWait,
			pipeUnlimitedInstances,
			4096,
			4096,
			0,
			saPtr,
		)
		if hPipe == uintptr(syscall.InvalidHandle) || hPipe == 0 {
			return
		}

		connected, _, err := procConnectNamedPipe.Call(hPipe, 0)
		if connected == 0 {
			if errno, ok := err.(syscall.Errno); !ok || errno != errPipeConnected {
				_, _, _ = procCloseHandle.Call(hPipe)
				continue
			}
		}

		// 立即并发处理当前连接并立刻循环创建下一个监听实例，杜绝高频打字或窗口切换时的 ERROR_FILE_NOT_FOUND 丢键
		go func(h uintptr) {
			handlePipeClient(syscall.Handle(h), handler)
			_, _, _ = procFlushFileBuffers.Call(h)
			_, _, _ = procDisconnectNamedPipe.Call(h)
			_, _, _ = procCloseHandle.Call(h)
		}(hPipe)
	}
}

func handlePipeClient(h syscall.Handle, handler PipeServerHandler) {
	var acc []byte
	var chunk [256]byte
	for len(acc) < 512 {
		var n uint32
		err := syscall.ReadFile(h, chunk[:], &n, nil)
		if n > 0 {
			acc = append(acc, chunk[:n]...)
			if strings.ContainsRune(string(chunk[:n]), '\n') {
				break
			}
		}
		if err != nil || n == 0 {
			break
		}
	}
	if len(acc) == 0 {
		return
	}
	line := strings.TrimSpace(string(acc))
	if line == "" {
		return
	}
	reply := dispatchPipeLine(line, handler)
	var written uint32
	_ = syscall.WriteFile(h, []byte(reply), &written, nil)
}

func dispatchPipeLine(line string, handler PipeServerHandler) string {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return "P\n"
	}

	switch parts[0] {
	case "K":
		// 格式: K <vk> <ch_hex> <ctrl> <alt> <shift> <x> <y>
		if len(parts) < 8 || handler.OnKey == nil {
			return "P\n"
		}
		vk64, _ := strconv.ParseUint(parts[1], 10, 32)
		ch64, _ := strconv.ParseUint(parts[2], 16, 32)
		ctrl := parts[3] == "1"
		alt := parts[4] == "1"
		shift := parts[5] == "1"
		x64, _ := strconv.ParseInt(parts[6], 10, 32)
		y64, _ := strconv.ParseInt(parts[7], 10, 32)

		vk := uint32(vk64)
		// 修复：TSF 模式下 CapsLock 开启且当前无编码缓冲时，直接放行字母输出大写英文
		if isCapsLock() && !ctrl && !alt && vk >= 0x41 && vk <= 0x5A {
			if handler.HasBuffer == nil || !handler.HasBuffer() {
				return "P\n"
			}
		}

		keyName, mappedRune := vkToKeyAndRune(vk, shift)
		ch := rune(ch64)
		if ch == 0 {
			ch = mappedRune
		}
		handled, commit := handler.OnKey(keyName, ch, ctrl, alt, shift, int32(x64), int32(y64))
		if !handled {
			return "P\n"
		}
		if commit != "" {
			return fmt.Sprintf("C %s\n", hex.EncodeToString([]byte(commit)))
		}
		return "E\n"

	case "S":
		var x, y int32
		if len(parts) >= 3 {
			x64, _ := strconv.ParseInt(parts[1], 10, 32)
			y64, _ := strconv.ParseInt(parts[2], 10, 32)
			x, y = int32(x64), int32(y64)
		}
		if handler.OnShiftTap != nil {
			commit := handler.OnShiftTap(x, y)
			if commit != "" {
				return fmt.Sprintf("C %s\n", hex.EncodeToString([]byte(commit)))
			}
		}
		return "E\n"

	case "F":
		if len(parts) >= 2 && handler.OnFocus != nil {
			handler.OnFocus(parts[1] == "1")
		}
		return "E\n"
	}

	return "P\n"
}
