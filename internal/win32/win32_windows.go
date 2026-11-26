//go:build windows

package win32

import (
	"fmt"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	modUser32   = syscall.NewLazyDLL("user32.dll")
	modKernel32 = syscall.NewLazyDLL("kernel32.dll")
	modGdi32    = syscall.NewLazyDLL("gdi32.dll")
	modShcore   = syscall.NewLazyDLL("shcore.dll")
	modShell32  = syscall.NewLazyDLL("shell32.dll")

	procSetWindowsHookExW    = modUser32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx  = modUser32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx       = modUser32.NewProc("CallNextHookEx")
	procGetMessageW          = modUser32.NewProc("GetMessageW")
	procTranslateMessage     = modUser32.NewProc("TranslateMessage")
	procDispatchMessageW     = modUser32.NewProc("DispatchMessageW")
	procPostQuitMessage      = modUser32.NewProc("PostQuitMessage")
	procPostMessageW         = modUser32.NewProc("PostMessageW")
	procSendInput            = modUser32.NewProc("SendInput")
	procGetAsyncKeyState     = modUser32.NewProc("GetAsyncKeyState")
	procGetKeyState          = modUser32.NewProc("GetKeyState")
	procGetForegroundWindow  = modUser32.NewProc("GetForegroundWindow")
	procGetWindowThreadPID   = modUser32.NewProc("GetWindowThreadProcessId")
	procGetGUIThreadInfo     = modUser32.NewProc("GetGUIThreadInfo")
	procClientToScreen       = modUser32.NewProc("ClientToScreen")
	procGetCursorPos         = modUser32.NewProc("GetCursorPos")
	procGetSystemMetrics     = modUser32.NewProc("GetSystemMetrics")
	procGetDpiForSystem      = modUser32.NewProc("GetDpiForSystem")
	procSetProcessDPIAware   = modUser32.NewProc("SetProcessDPIAware")
	procSetProcessDpiAwareEx = modShcore.NewProc("SetProcessDpiAwareness")

	procRegisterClassExW    = modUser32.NewProc("RegisterClassExW")
	procCreateWindowExW     = modUser32.NewProc("CreateWindowExW")
	procDestroyWindow       = modUser32.NewProc("DestroyWindow")
	procDefWindowProcW      = modUser32.NewProc("DefWindowProcW")
	procShowWindow          = modUser32.NewProc("ShowWindow")
	procSetWindowPos        = modUser32.NewProc("SetWindowPos")
	procInvalidateRect      = modUser32.NewProc("InvalidateRect")
	procBeginPaint          = modUser32.NewProc("BeginPaint")
	procEndPaint            = modUser32.NewProc("EndPaint")
	procFillRect            = modUser32.NewProc("FillRect")
	procFrameRect           = modUser32.NewProc("FrameRect")
	procDrawTextW           = modUser32.NewProc("DrawTextW")
	procLoadCursorW         = modUser32.NewProc("LoadCursorW")
	procLoadImageW          = modUser32.NewProc("LoadImageW")
	procDestroyIcon         = modUser32.NewProc("DestroyIcon")
	procSetCapture          = modUser32.NewProc("SetCapture")
	procReleaseCapture      = modUser32.NewProc("ReleaseCapture")
	procGetWindowRect       = modUser32.NewProc("GetWindowRect")
	procCreatePopupMenu     = modUser32.NewProc("CreatePopupMenu")
	procAppendMenuW         = modUser32.NewProc("AppendMenuW")
	procTrackPopupMenu      = modUser32.NewProc("TrackPopupMenu")
	procDestroyMenu         = modUser32.NewProc("DestroyMenu")
	procSetForegroundWindow = modUser32.NewProc("SetForegroundWindow")

	procGetModuleHandleW = modKernel32.NewProc("GetModuleHandleW")
	procCreateMutexW     = modKernel32.NewProc("CreateMutexW")
	procCloseHandle      = modKernel32.NewProc("CloseHandle")

	procCreateSolidBrush       = modGdi32.NewProc("CreateSolidBrush")
	procCreateFontW            = modGdi32.NewProc("CreateFontW")
	procSelectObject           = modGdi32.NewProc("SelectObject")
	procDeleteObject           = modGdi32.NewProc("DeleteObject")
	procSetBkMode              = modGdi32.NewProc("SetBkMode")
	procSetTextColor           = modGdi32.NewProc("SetTextColor")
	procGetTextExtentW         = modGdi32.NewProc("GetTextExtentPoint32W")
	procCreateCompatibleDC     = modGdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = modGdi32.NewProc("CreateCompatibleBitmap")
	procDeleteDC               = modGdi32.NewProc("DeleteDC")
	procBitBlt                 = modGdi32.NewProc("BitBlt")

	procShellExecuteW = modShell32.NewProc("ShellExecuteW")
)

const (
	WH_KEYBOARD_LL = 13
	WM_KEYDOWN     = 0x0100
	WM_KEYUP       = 0x0101
	WM_SYSKEYDOWN  = 0x0104
	WM_SYSKEYUP    = 0x0105

	LLKHF_INJECTED = 0x00000010

	VK_BACK       = 0x08
	VK_TAB        = 0x09
	VK_RETURN     = 0x0D
	VK_SHIFT      = 0x10
	VK_CONTROL    = 0x11
	VK_MENU       = 0x12
	VK_CAPITAL    = 0x14
	VK_ESCAPE     = 0x1B
	VK_SPACE      = 0x20
	VK_PRIOR      = 0x21
	VK_NEXT       = 0x22
	VK_LEFT       = 0x25
	VK_UP         = 0x26
	VK_RIGHT      = 0x27
	VK_DOWN       = 0x28
	VK_DELETE     = 0x2E
	VK_LWIN       = 0x5B
	VK_RWIN       = 0x5C
	VK_LSHIFT     = 0xA0
	VK_RSHIFT     = 0xA1
	VK_LCONTROL   = 0xA2
	VK_RCONTROL   = 0xA3
	VK_LMENU      = 0xA4
	VK_RMENU      = 0xA5
	VK_OEM_1      = 0xBA
	VK_OEM_PLUS   = 0xBB
	VK_OEM_COMMA  = 0xBC
	VK_OEM_MINUS  = 0xBD
	VK_OEM_PERIOD = 0xBE
	VK_OEM_2      = 0xBF
	VK_OEM_3      = 0xC0
	VK_OEM_4      = 0xDB
	VK_OEM_5      = 0xDC
	VK_OEM_6      = 0xDD
	VK_OEM_7      = 0xDE

	INPUT_KEYBOARD    = 1
	KEYEVENTF_KEYUP   = 0x0002
	KEYEVENTF_UNICODE = 0x0004

	SM_CXSCREEN = 0
	SM_CYSCREEN = 1

	SRCCOPY         = 0x00CC0020
	IMAGE_ICON      = 1
	LR_LOADFROMFILE = 0x00000010
	LR_DEFAULTSIZE  = 0x00000040

	errorAlreadyExists = 183
)

type point struct {
	X int32
	Y int32
}

type rect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type sizeStruct struct {
	CX int32
	CY int32
}

type msgStruct struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

type kbdLLHookStruct struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

// keybdInputEvent 严格匹配 Windows amd64 平台 40 字节 INPUT 结构体内存布局。
type keybdInputEvent struct {
	Type        uint32
	pad0        uint32
	WVk         uint16
	WScan       uint16
	DwFlags     uint32
	Time        uint32
	pad1        uint32
	DwExtraInfo uintptr
	tailPad     [8]byte
}

type guiThreadInfo struct {
	CbSize        uint32
	Flags         uint32
	HwndActive    uintptr
	HwndFocus     uintptr
	HwndCapture   uintptr
	HwndMenuOwner uintptr
	HwndMoveSize  uintptr
	HwndCaret     uintptr
	RcCaret       rect
}

// AcquireSingleInstanceMutex 防止重复启动多个 Qin书 实例互相抢占全局键盘钩子。
func AcquireSingleInstanceMutex() (uintptr, error) {
	name, _ := syscall.UTF16PtrFromString("Local\\QinShuIME_SingleInstance_Mutex")
	handle, _, err := procCreateMutexW.Call(0, 1, uintptr(unsafe.Pointer(name)))
	if handle == 0 {
		return 0, err
	}
	if errno, ok := err.(syscall.Errno); ok && errno == errorAlreadyExists {
		_, _, _ = procCloseHandle.Call(handle)
		return 0, fmt.Errorf("Qin书输入法已在运行中，无需重复启动")
	}
	return handle, nil
}

// ReleaseHandle 释放 Win32 内核句柄。
func ReleaseHandle(handle uintptr) {
	if handle != 0 {
		_, _, _ = procCloseHandle.Call(handle)
	}
}

// LoadIconFromFile 从磁盘加载 .ico 程序图标供窗口类使用。
func LoadIconFromFile(path string) uintptr {
	u16, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0
	}
	h, _, _ := procLoadImageW.Call(0, uintptr(unsafe.Pointer(u16)), IMAGE_ICON, 0, 0, LR_LOADFROMFILE|LR_DEFAULTSIZE)
	return h
}

// DestroyIconHandle 释放加载的图标句柄。
func DestroyIconHandle(h uintptr) {
	if h != 0 {
		_, _, _ = procDestroyIcon.Call(h)
	}
}

// EnableDPIAwareness 开启 Windows 11 每显示器高 DPI 感知。
func EnableDPIAwareness() {
	if procSetProcessDpiAwareEx.Find() == nil {
		r, _, _ := procSetProcessDpiAwareEx.Call(2)
		if r == 0 {
			return
		}
	}
	_, _, _ = procSetProcessDPIAware.Call()
}

// GetSystemDPI 获取当前系统 DPI（默认 96，150% 缩放时返回 144）。
func GetSystemDPI() int32 {
	if procGetDpiForSystem.Find() == nil {
		dpi, _, _ := procGetDpiForSystem.Call()
		if dpi >= 96 && dpi <= 480 {
			return int32(dpi)
		}
	}
	return 96
}

// OpenFolderInExplorer 在资源管理器中打开指定目录（或用默认编辑器打开指定文件）。
func OpenFolderInExplorer(path string) {
	verb, _ := syscall.UTF16PtrFromString("open")
	target, _ := syscall.UTF16PtrFromString(path)
	_, _, _ = procShellExecuteW.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(target)), 0, 0, 1)
}

var sendQueue = make(chan string, 128)

func init() {
	go func() {
		for text := range sendQueue {
			SendUnicodeText(text)
		}
	}()
}

// SendUnicodeTextAsync 将待上屏文本投递至异步队列（保证顺序且不阻塞键盘钩子）。
func SendUnicodeTextAsync(text string) {
	if text == "" {
		return
	}
	select {
	case sendQueue <- text:
	default:
		SendUnicodeText(text)
	}
}

// SendUnicodeText 使用 Win32 SendInput(KEYEVENTF_UNICODE) 向当前活动窗口上屏文本。
func SendUnicodeText(text string) {
	if text == "" {
		return
	}
	units := utf16.Encode([]rune(text))
	if len(units) == 0 {
		return
	}
	events := make([]keybdInputEvent, 0, len(units)*2)
	for _, cu := range units {
		events = append(events,
			keybdInputEvent{
				Type:    INPUT_KEYBOARD,
				WScan:   cu,
				DwFlags: KEYEVENTF_UNICODE,
			},
			keybdInputEvent{
				Type:    INPUT_KEYBOARD,
				WScan:   cu,
				DwFlags: KEYEVENTF_UNICODE | KEYEVENTF_KEYUP,
			},
		)
	}
	_, _, _ = procSendInput.Call(
		uintptr(len(events)),
		uintptr(unsafe.Pointer(&events[0])),
		unsafe.Sizeof(events[0]),
	)
}

// GetCaretScreenPos 获取当前输入光标屏幕坐标，失败时回退至鼠标坐标。
func GetCaretScreenPos() (int32, int32) {
	hwndFg, _, _ := procGetForegroundWindow.Call()
	if hwndFg != 0 {
		tid, _, _ := procGetWindowThreadPID.Call(hwndFg, 0)
		var gui guiThreadInfo
		gui.CbSize = uint32(unsafe.Sizeof(gui))
		ok, _, _ := procGetGUIThreadInfo.Call(tid, uintptr(unsafe.Pointer(&gui)))
		if ok != 0 && gui.HwndCaret != 0 && (gui.RcCaret.Left != 0 || gui.RcCaret.Top != 0 || gui.RcCaret.Bottom != 0) {
			pt := point{X: gui.RcCaret.Left, Y: gui.RcCaret.Bottom}
			ok2, _, _ := procClientToScreen.Call(gui.HwndCaret, uintptr(unsafe.Pointer(&pt)))
			if ok2 != 0 && (pt.X > 0 || pt.Y > 0) {
				return pt.X, pt.Y + 6
			}
		}
	}
	var pt point
	ok, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	if ok != 0 {
		return pt.X + 12, pt.Y + 24
	}
	return 400, 300
}

func isKeyDown(vk int) bool {
	r, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
	return (r & 0x8000) != 0
}

func isCapsLock() bool {
	r, _, _ := procGetKeyState.Call(uintptr(VK_CAPITAL))
	return (r & 0x0001) != 0
}

var shiftDigitMap = map[rune]rune{
	'1': '!', '2': '@', '3': '#', '4': '$', '5': '%',
	'6': '^', '7': '&', '8': '*', '9': '(', '0': ')',
}

type oemSpec struct {
	normal  rune
	shifted rune
	name    string
}

var vkOEMMap = map[uint32]oemSpec{
	VK_OEM_1:      {';', ':', "semicolon"},
	VK_OEM_PLUS:   {'=', '+', "equal"},
	VK_OEM_COMMA:  {',', '<', "comma"},
	VK_OEM_MINUS:  {'-', '_', "minus"},
	VK_OEM_PERIOD: {'.', '>', "period"},
	VK_OEM_2:      {'/', '?', "slash"},
	VK_OEM_3:      {'`', '~', "grave"},
	VK_OEM_4:      {'[', '{', "bracketleft"},
	VK_OEM_5:      {'\\', '|', "backslash"},
	VK_OEM_6:      {']', '}', "bracketright"},
	VK_OEM_7:      {'\'', '"', "apostrophe"},
}

var specialVKMap = map[uint32]string{
	VK_BACK:   "BackSpace",
	VK_TAB:    "Tab",
	VK_RETURN: "Return",
	VK_ESCAPE: "Escape",
	VK_SPACE:  "Space",
	VK_PRIOR:  "Page_Up",
	VK_NEXT:   "Page_Down",
	VK_LEFT:   "Left",
	VK_UP:     "Up",
	VK_RIGHT:  "Right",
	VK_DOWN:   "Down",
	VK_DELETE: "Delete",
}

func vkToKeyAndRune(vk uint32, shift bool) (string, rune) {
	if name, ok := specialVKMap[vk]; ok {
		return name, 0
	}
	if vk >= 0x41 && vk <= 0x5A {
		upper := shift != isCapsLock()
		if upper {
			return string(rune(vk + 32)), rune(vk)
		}
		return string(rune(vk + 32)), rune(vk + 32)
	}
	if vk >= 0x30 && vk <= 0x39 {
		r := rune(vk)
		if shift {
			if sr, ok := shiftDigitMap[r]; ok {
				return string(r), sr
			}
		}
		return string(r), r
	}
	if spec, ok := vkOEMMap[vk]; ok {
		if shift {
			return spec.name, spec.shifted
		}
		return spec.name, spec.normal
	}
	return "", 0
}

// KeyboardHook 封装全局低级键盘钩子。
type KeyboardHook struct {
	hookID         uintptr
	shiftDownAlone bool
	hasBuffer      func() bool
	onKey          func(keyName string, ch rune, ctrl, alt, shift bool) bool
	onShiftTap     func()
}

var activeHook *KeyboardHook

// InstallPassiveShiftObserver 在 TSF 模式下安装绝不拦截任何按键的只读 Shift 单击观察器（兼容 Electron/Chromium 等不转发独立 Shift KeyUp 的宿主）。
func InstallPassiveShiftObserver(isActive func() bool, onShiftTap func()) (*KeyboardHook, error) {
	h := &KeyboardHook{
		hasBuffer:  isActive,
		onShiftTap: onShiftTap,
	}
	activeHook = h
	cb := syscall.NewCallback(passiveShiftProc)
	id, _, err := procSetWindowsHookExW.Call(WH_KEYBOARD_LL, cb, 0, 0)
	if id == 0 {
		return nil, err
	}
	h.hookID = id
	return h, nil
}

func passiveShiftProc(nCode int, wParam uintptr, lParam uintptr) uintptr {
	h := activeHook
	if nCode >= 0 && h != nil {
		kb := (*kbdLLHookStruct)(unsafe.Pointer(lParam))
		if (kb.Flags & LLKHF_INJECTED) == 0 {
			if isKeyDown(VK_LWIN) || isKeyDown(VK_RWIN) || isKeyDown(VK_CONTROL) || isKeyDown(VK_LCONTROL) || isKeyDown(VK_RCONTROL) || isKeyDown(VK_MENU) || isKeyDown(VK_LMENU) || isKeyDown(VK_RMENU) {
				h.shiftDownAlone = false
			} else {
				vk := kb.VkCode
				if vk == VK_LSHIFT || vk == VK_RSHIFT || vk == VK_SHIFT {
					if wParam == WM_KEYDOWN || wParam == WM_SYSKEYDOWN {
						h.shiftDownAlone = true
					} else if wParam == WM_KEYUP || wParam == WM_SYSKEYUP {
						if h.shiftDownAlone {
							h.shiftDownAlone = false
							if (h.hasBuffer == nil || h.hasBuffer()) && h.onShiftTap != nil {
								h.onShiftTap()
							}
						}
					}
				} else if wParam == WM_KEYDOWN || wParam == WM_SYSKEYDOWN {
					h.shiftDownAlone = false
				}
			}
		}
	}
	var hid uintptr
	if h != nil {
		hid = h.hookID
	}
	r, _, _ := procCallNextHookEx.Call(hid, uintptr(nCode), wParam, lParam)
	return r
}

// InstallHook 在当前线程安装 WH_KEYBOARD_LL 全局键盘钩子（需配合 Win32 消息循环）。
func InstallHook(
	hasBuffer func() bool,
	onKey func(keyName string, ch rune, ctrl, alt, shift bool) bool,
	onShiftTap func(),
) (*KeyboardHook, error) {
	h := &KeyboardHook{
		hasBuffer:  hasBuffer,
		onKey:      onKey,
		onShiftTap: onShiftTap,
	}
	activeHook = h
	cb := syscall.NewCallback(lowLevelKeyboardProc)
	id, _, err := procSetWindowsHookExW.Call(WH_KEYBOARD_LL, cb, 0, 0)
	if id == 0 {
		return nil, err
	}
	h.hookID = id
	return h, nil
}

// Uninstall 卸载键盘钩子。
func (h *KeyboardHook) Uninstall() {
	if h != nil && h.hookID != 0 {
		_, _, _ = procUnhookWindowsHookEx.Call(h.hookID)
		h.hookID = 0
	}
}

func lowLevelKeyboardProc(nCode int, wParam uintptr, lParam uintptr) uintptr {
	h := activeHook
	if nCode >= 0 && h != nil {
		kb := (*kbdLLHookStruct)(unsafe.Pointer(lParam))
		if (kb.Flags & LLKHF_INJECTED) != 0 {
			r, _, _ := procCallNextHookEx.Call(h.hookID, uintptr(nCode), wParam, lParam)
			return r
		}

		if isKeyDown(VK_LWIN) || isKeyDown(VK_RWIN) {
			h.shiftDownAlone = false
			r, _, _ := procCallNextHookEx.Call(h.hookID, uintptr(nCode), wParam, lParam)
			return r
		}

		vk := kb.VkCode
		if vk == VK_LSHIFT || vk == VK_RSHIFT || vk == VK_SHIFT {
			if wParam == WM_KEYDOWN || wParam == WM_SYSKEYDOWN {
				h.shiftDownAlone = true
			} else if wParam == WM_KEYUP || wParam == WM_SYSKEYUP {
				if h.shiftDownAlone {
					h.shiftDownAlone = false
					if h.onShiftTap != nil {
						h.onShiftTap()
					}
				}
			}
			r, _, _ := procCallNextHookEx.Call(h.hookID, uintptr(nCode), wParam, lParam)
			return r
		}

		if wParam == WM_KEYDOWN || wParam == WM_SYSKEYDOWN {
			h.shiftDownAlone = false
			ctrl := isKeyDown(VK_CONTROL) || isKeyDown(VK_LCONTROL) || isKeyDown(VK_RCONTROL)
			alt := isKeyDown(VK_MENU) || isKeyDown(VK_LMENU) || isKeyDown(VK_RMENU)
			shift := isKeyDown(VK_SHIFT) || isKeyDown(VK_LSHIFT) || isKeyDown(VK_RSHIFT)

			if isCapsLock() && !ctrl && !alt && vk >= 0x41 && vk <= 0x5A {
				if h.hasBuffer == nil || !h.hasBuffer() {
					r, _, _ := procCallNextHookEx.Call(h.hookID, uintptr(nCode), wParam, lParam)
					return r
				}
			}

			keyName, ch := vkToKeyAndRune(vk, shift)
			if (keyName != "" || ch != 0) && h.onKey != nil {
				if h.onKey(keyName, ch, ctrl, alt, shift) {
					return 1
				}
			}
		}
	}
	var hid uintptr
	if h != nil {
		hid = h.hookID
	}
	r, _, _ := procCallNextHookEx.Call(hid, uintptr(nCode), wParam, lParam)
	return r
}
