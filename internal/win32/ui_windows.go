//go:build windows

package win32

import (
	"fmt"
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"

	"qinshu/internal/config"
	"qinshu/internal/engine"
)

const (
	WS_POPUP         = 0x80000000
	WS_VISIBLE       = 0x10000000
	WS_EX_TOPMOST    = 0x00000008
	WS_EX_TOOLWINDOW = 0x00000080
	WS_EX_NOACTIVATE = 0x08000000

	WM_DESTROY       = 0x0002
	WM_PAINT         = 0x000F
	WM_ERASEBKGND    = 0x0014
	WM_MOUSEACTIVATE = 0x0021
	WM_MOUSEMOVE     = 0x0200
	WM_LBUTTONDOWN   = 0x0201
	WM_LBUTTONUP     = 0x0202
	WM_RBUTTONUP     = 0x0205
	WM_USER          = 0x0400
	wmRefreshUI      = WM_USER + 101

	MA_NOACTIVATE = 3

	SW_HIDE           = 0
	SW_SHOWNOACTIVATE = 4

	HWND_TOPMOST   = ^uintptr(0)
	SWP_NOACTIVATE = 0x0010
	SWP_SHOWWINDOW = 0x0040

	TRANSPARENT   = 1
	DT_LEFT       = 0x00000000
	DT_CENTER     = 0x00000001
	DT_VCENTER    = 0x00000004
	DT_SINGLELINE = 0x00000020

	IDC_ARROW = 32512

	MF_STRING       = 0x00000000
	MF_CHECKED      = 0x00000008
	MF_SEPARATOR    = 0x00000800
	TPM_LEFTALIGN   = 0x0000
	TPM_BOTTOMALIGN = 0x0020
	TPM_RETURNCMD   = 0x0100

	cmdModeMixed     = 1001
	cmdModeWubi      = 1002
	cmdModePinyin    = 1003
	cmdFuzzyMaster   = 1010
	cmdFuzzySouth    = 1011
	cmdFuzzyAllOn    = 1012
	cmdFuzzyAllOff   = 1013
	cmdAuto4         = 1020
	cmdPush5         = 1021
	cmdReverseHint   = 1022
	cmdHorizToggle   = 1023
	cmdAutoLearn     = 1024
	cmdCtrlSpace     = 1025
	cmdThemeLight    = 1030
	cmdThemeBlue     = 1031
	cmdThemeDark     = 1032
	cmdReloadDict    = 1040
	cmdOpenDataDir   = 1041
	cmdEditPhrases   = 1042
	cmdExitApp       = 1099
	cmdFuzzyRuleBase = 2000
)

var fuzzyMenuItems = []struct {
	key   string
	label string
}{
	{"z_zh", "模糊音：z ↔ zh"},
	{"c_ch", "模糊音：c ↔ ch"},
	{"s_sh", "模糊音：s ↔ sh"},
	{"l_n", "模糊音：l ↔ n"},
	{"f_h", "模糊音：f ↔ h"},
	{"r_l", "模糊音：r ↔ l"},
	{"k_g", "模糊音：k ↔ g"},
	{"an_ang", "模糊音：an ↔ ang"},
	{"en_eng", "模糊音：en ↔ eng"},
	{"in_ing", "模糊音：in ↔ ing"},
	{"ian_iang", "模糊音：ian ↔ iang"},
	{"uan_uang", "模糊音：uan ↔ uang"},
}

type wndClassExW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type paintStruct struct {
	Hdc         uintptr
	FErase      int32
	RcPaint     rect
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}

type candHitBox struct {
	rc    rect
	index int
}

// App 管理 Windows 11 原生 GDI 双缓冲悬浮状态栏、高 DPI 悬浮候选窗，支持绿色钩子模式与原生 TSF 命名管道后端模式。
type App struct {
	eng     *engine.Engine
	baseDir string
	tsfMode bool
	hook    *KeyboardHook
	dpi     int32

	hInst      uintptr
	hIcon      uintptr
	statusHwnd uintptr
	candHwnd   uintptr

	fontBold   uintptr
	fontNormal uintptr
	fontSmall  uintptr

	dragging    bool
	dragOffsetX int32
	dragOffsetY int32
	candHits    []candHitBox

	caretMu     sync.Mutex
	hasTSFCaret bool
	tsfCaretX   int32
	tsfCaretY   int32
}

var activeApp *App

func (a *App) s(px int32) int32 {
	return (px*a.dpi + 48) / 96
}

func (a *App) setTSFCaret(x, y int32) {
	if x <= 0 && y <= 0 {
		return
	}
	a.caretMu.Lock()
	a.hasTSFCaret = true
	a.tsfCaretX = x
	a.tsfCaretY = y
	a.caretMu.Unlock()
}

func (a *App) getPopupCaretPos() (int32, int32) {
	if a.tsfMode {
		a.caretMu.Lock()
		ok, x, y := a.hasTSFCaret, a.tsfCaretX, a.tsfCaretY
		a.caretMu.Unlock()
		if ok {
			return x, y
		}
	}
	return GetCaretScreenPos()
}

// RunApp 启动 Qin书 Windows 11 悬浮输入法。tsfMode=true 时运行为原生 TSF COM 的轻量后端服务。
func RunApp(eng *engine.Engine, baseDir string, tsfMode bool) error {
	mutex, err := AcquireSingleInstanceMutex()
	if err != nil {
		return err
	}
	defer ReleaseHandle(mutex)

	EnableDPIAwareness()
	dpi := GetSystemDPI()

	hInst, _, _ := procGetModuleHandleW.Call(0)
	hIcon := LoadIconFromFile(filepath.Join(baseDir, "assets", "qinshu.ico"))
	defer DestroyIconHandle(hIcon)

	app := &App{
		eng:     eng,
		baseDir: baseDir,
		tsfMode: tsfMode,
		dpi:     dpi,
		hInst:   hInst,
		hIcon:   hIcon,
	}
	activeApp = app
	app.initFonts()
	defer app.cleanupFonts()

	if err := app.createStatusWindow(); err != nil {
		return err
	}
	if err := app.createCandidateWindow(); err != nil {
		return err
	}

	if tsfMode {
		// 原生 TSF 模式：不安装 WH_KEYBOARD_LL 钩子，仅监听轻量 Named Pipe
		stopPipe := StartTSFPipeServer(PipeServerHandler{
			HasBuffer: app.eng.HasActiveBuffer,
			OnKey: func(keyName string, ch rune, ctrl, alt, shift bool, x, y int32) (bool, string) {
				app.setTSFCaret(x, y)
				act := app.eng.HandleKey(keyName, ch, ctrl, alt, shift)
				if act.UpdateUI {
					app.postRefresh()
				}
				return act.Handled, act.CommitText
			},
			OnShiftTap: func(x, y int32) string {
				app.setTSFCaret(x, y)
				committed := app.eng.ToggleLanguage()
				app.postRefresh()
				return committed
			},
			OnFocus: func(focused bool) {
				if !focused && app.eng.HasActiveBuffer() {
					app.eng.Clear()
					app.postRefresh()
				}
			},
		})
		defer close(stopPipe)
	} else {
		// 绿色免安装模式：安装全局低级键盘钩子
		hook, err := InstallHook(
			func() bool {
				return app.eng.HasActiveBuffer()
			},
			func(keyName string, ch rune, ctrl, alt, shift bool) bool {
				act := app.eng.HandleKey(keyName, ch, ctrl, alt, shift)
				if act.CommitText != "" {
					SendUnicodeTextAsync(act.CommitText)
				}
				if act.UpdateUI {
					app.postRefresh()
				}
				return act.Handled
			},
			func() {
				committed := app.eng.ToggleLanguage()
				if committed != "" {
					SendUnicodeTextAsync(committed)
				}
				app.postRefresh()
			},
		)
		if err != nil {
			return err
		}
		app.hook = hook
		defer app.hook.Uninstall()
	}

	var msg msgStruct
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		_, _, _ = procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		_, _, _ = procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
	app.eng.Flush()
	return nil
}

func (a *App) postRefresh() {
	if a.statusHwnd != 0 {
		_, _, _ = procPostMessageW.Call(a.statusHwnd, wmRefreshUI, 0, 0)
	}
}

func (a *App) initFonts() {
	face, _ := syscall.UTF16PtrFromString("Microsoft YaHei UI")
	a.fontBold, _, _ = procCreateFontW.Call(
		uintptr(uint32(-a.s(16))), 0, 0, 0, 700, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(face)),
	)
	a.fontNormal, _, _ = procCreateFontW.Call(
		uintptr(uint32(-a.s(15))), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(face)),
	)
	a.fontSmall, _, _ = procCreateFontW.Call(
		uintptr(uint32(-a.s(12))), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(face)),
	)
}

func (a *App) cleanupFonts() {
	if a.fontBold != 0 {
		_, _, _ = procDeleteObject.Call(a.fontBold)
	}
	if a.fontNormal != 0 {
		_, _, _ = procDeleteObject.Call(a.fontNormal)
	}
	if a.fontSmall != 0 {
		_, _, _ = procDeleteObject.Call(a.fontSmall)
	}
}

func (a *App) createStatusWindow() error {
	clsName, _ := syscall.UTF16PtrFromString("QinShuStatusBarClass")
	title, _ := syscall.UTF16PtrFromString("Qin书")
	cursor, _, _ := procLoadCursorW.Call(0, IDC_ARROW)

	wc := wndClassExW{
		CbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		LpfnWndProc:   syscall.NewCallback(statusWndProc),
		HInstance:     a.hInst,
		HIcon:         a.hIcon,
		HIconSm:       a.hIcon,
		HCursor:       cursor,
		LpszClassName: clsName,
	}
	_, _, _ = procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	sw, _, _ := procGetSystemMetrics.Call(SM_CXSCREEN)
	sh, _, _ := procGetSystemMetrics.Call(SM_CYSCREEN)
	barW := a.s(252)
	barH := a.s(36)
	x := int32(sw) - barW - a.s(36)
	y := int32(sh) - barH - a.s(76)
	if a.eng.Config.HasSavedStatusPos {
		x = int32(a.eng.Config.StatusBarX)
		y = int32(a.eng.Config.StatusBarY)
	}

	hwnd, _, err := procCreateWindowExW.Call(
		WS_EX_TOPMOST|WS_EX_TOOLWINDOW|WS_EX_NOACTIVATE,
		uintptr(unsafe.Pointer(clsName)),
		uintptr(unsafe.Pointer(title)),
		WS_POPUP|WS_VISIBLE,
		uintptr(x), uintptr(y), uintptr(barW), uintptr(barH),
		0, 0, a.hInst, 0,
	)
	if hwnd == 0 {
		return err
	}
	a.statusHwnd = hwnd
	_, _, _ = procShowWindow.Call(hwnd, SW_SHOWNOACTIVATE)
	return nil
}

func (a *App) createCandidateWindow() error {
	clsName, _ := syscall.UTF16PtrFromString("QinShuCandidateClass")
	title, _ := syscall.UTF16PtrFromString("Qin书候选窗")
	cursor, _, _ := procLoadCursorW.Call(0, IDC_ARROW)

	wc := wndClassExW{
		CbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		LpfnWndProc:   syscall.NewCallback(candWndProc),
		HInstance:     a.hInst,
		HIcon:         a.hIcon,
		HIconSm:       a.hIcon,
		HCursor:       cursor,
		LpszClassName: clsName,
	}
	_, _, _ = procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	hwnd, _, err := procCreateWindowExW.Call(
		WS_EX_TOPMOST|WS_EX_TOOLWINDOW|WS_EX_NOACTIVATE,
		uintptr(unsafe.Pointer(clsName)),
		uintptr(unsafe.Pointer(title)),
		WS_POPUP,
		0, 0, uintptr(a.s(320)), uintptr(a.s(72)),
		0, 0, a.hInst, 0,
	)
	if hwnd == 0 {
		return err
	}
	a.candHwnd = hwnd
	return nil
}

func (a *App) refreshAllWindows() {
	if a.statusHwnd != 0 {
		_, _, _ = procInvalidateRect.Call(a.statusHwnd, 0, 0)
	}
	if a.candHwnd == 0 {
		return
	}
	state := a.eng.GetViewState()
	if !state.Visible {
		_, _, _ = procShowWindow.Call(a.candHwnd, SW_HIDE)
		return
	}

	width := a.s(290)
	height := a.s(72)
	if a.eng.Config.HorizontalCandidates {
		est := a.s(32)
		for _, c := range state.PageCandidates {
			var txtW int32
			for _, r := range c.Text {
				if r > 127 {
					txtW += 17
				} else {
					txtW += 9
				}
			}
			est += a.s(txtW + 38)
			if c.Hint != "" {
				est += a.s(int32(len([]rune(c.Hint)))*9 + 12)
			}
		}
		if est > width {
			width = est
		}
		if maxW := a.s(960); width > maxW {
			width = maxW
		}
	} else {
		width = a.s(300)
		cnt := int32(len(state.PageCandidates))
		if cnt < 1 {
			cnt = 1
		}
		height = a.s(38 + cnt*28 + 10)
	}

	cx, cy := a.getPopupCaretPos()
	sw, _, _ := procGetSystemMetrics.Call(SM_CXSCREEN)
	sh, _, _ := procGetSystemMetrics.Call(SM_CYSCREEN)
	if cx+width > int32(sw)-a.s(12) {
		cx = int32(sw) - width - a.s(12)
		if cx < a.s(12) {
			cx = a.s(12)
		}
	}
	if cy+height > int32(sh)-a.s(48) {
		cy = cy - height - a.s(32)
		if cy < a.s(12) {
			cy = a.s(12)
		}
	}

	_, _, _ = procSetWindowPos.Call(
		a.candHwnd,
		HWND_TOPMOST,
		uintptr(cx), uintptr(cy), uintptr(width), uintptr(height),
		SWP_NOACTIVATE|SWP_SHOWWINDOW,
	)
	_, _, _ = procInvalidateRect.Call(a.candHwnd, 0, 0)
}

func drawTextRect(hdc uintptr, text string, rc rect, color uint32, font uintptr, flags uint32) {
	_, _, _ = procSetBkMode.Call(hdc, TRANSPARENT)
	_, _, _ = procSetTextColor.Call(hdc, uintptr(color))
	old := uintptr(0)
	if font != 0 {
		old, _, _ = procSelectObject.Call(hdc, font)
	}
	u16, _ := syscall.UTF16FromString(text)
	if len(u16) > 1 {
		_, _, _ = procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(&u16[0])), uintptr(len(u16)-1), uintptr(unsafe.Pointer(&rc)), uintptr(flags))
	}
	if old != 0 {
		_, _, _ = procSelectObject.Call(hdc, old)
	}
}

func measureTextWidth(hdc uintptr, text string, font uintptr) int32 {
	old := uintptr(0)
	if font != 0 {
		old, _, _ = procSelectObject.Call(hdc, font)
	}
	var sz sizeStruct
	u16, _ := syscall.UTF16FromString(text)
	if len(u16) > 1 {
		_, _, _ = procGetTextExtentW.Call(hdc, uintptr(unsafe.Pointer(&u16[0])), uintptr(len(u16)-1), uintptr(unsafe.Pointer(&sz)))
	}
	if old != 0 {
		_, _, _ = procSelectObject.Call(hdc, old)
	}
	return sz.CX
}

func fillRectColor(hdc uintptr, rc rect, color uint32) {
	br, _, _ := procCreateSolidBrush.Call(uintptr(color))
	_, _, _ = procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), br)
	_, _, _ = procDeleteObject.Call(br)
}

func fillAndFrame(hdc uintptr, rc rect, bg, border uint32) {
	fillRectColor(hdc, rc, bg)
	brBorder, _, _ := procCreateSolidBrush.Call(uintptr(border))
	_, _, _ = procFrameRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), brBorder)
	_, _, _ = procDeleteObject.Call(brBorder)
}

func withDoubleBuffer(hdc uintptr, w, h int32, draw func(memDC uintptr)) {
	memDC, _, _ := procCreateCompatibleDC.Call(hdc)
	if memDC == 0 {
		draw(hdc)
		return
	}
	bmp, _, _ := procCreateCompatibleBitmap.Call(hdc, uintptr(w), uintptr(h))
	if bmp == 0 {
		_, _, _ = procDeleteDC.Call(memDC)
		draw(hdc)
		return
	}
	oldBmp, _, _ := procSelectObject.Call(memDC, bmp)
	draw(memDC)
	_, _, _ = procBitBlt.Call(hdc, 0, 0, uintptr(w), uintptr(h), memDC, 0, 0, SRCCOPY)
	_, _, _ = procSelectObject.Call(memDC, oldBmp)
	_, _, _ = procDeleteObject.Call(bmp)
	_, _, _ = procDeleteDC.Call(memDC)
}

func (a *App) paintStatusBar(hwnd uintptr) {
	var ps paintStruct
	hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	defer procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))

	barW := a.s(252)
	barH := a.s(36)

	withDoubleBuffer(hdc, barW, barH, func(dc uintptr) {
		theme := a.eng.Config.CurrentTheme()
		fillAndFrame(dc, rect{0, 0, barW, barH}, theme.StatusBg, theme.Border)

		for _, divX := range []int32{68, 104, 140, 176, 214} {
			dx := a.s(divX)
			fillRectColor(dc, rect{dx, a.s(8), dx + 1, barH - a.s(8)}, theme.Divider)
		}

		drawTextRect(dc, "Qin书", rect{a.s(4), 0, a.s(68), barH}, theme.Accent, a.fontBold, DT_CENTER|DT_VCENTER|DT_SINGLELINE)

		langText := "中"
		if a.eng.Config.Language == config.LangEn {
			langText = "En"
		}
		drawTextRect(dc, langText, rect{a.s(68), 0, a.s(104), barH}, theme.StatusFg, a.fontBold, DT_CENTER|DT_VCENTER|DT_SINGLELINE)

		modeText := "混"
		switch a.eng.Config.InputMode {
		case config.ModeWubi:
			modeText = "五"
		case config.ModePinyin:
			modeText = "拼"
		}
		drawTextRect(dc, modeText, rect{a.s(104), 0, a.s(140), barH}, theme.StatusFg, a.fontNormal, DT_CENTER|DT_VCENTER|DT_SINGLELINE)

		punctText := "。"
		if !a.eng.Config.ChinesePunctuation {
			punctText = "."
		}
		drawTextRect(dc, punctText, rect{a.s(140), 0, a.s(176), barH}, theme.StatusFg, a.fontNormal, DT_CENTER|DT_VCENTER|DT_SINGLELINE)

		fuzzyText := "糊"
		fuzzyColor := theme.HintFg
		if !a.eng.Config.FuzzyEnabled {
			fuzzyText = "准"
			fuzzyColor = theme.IndexFg
		}
		drawTextRect(dc, fuzzyText, rect{a.s(176), 0, a.s(214), barH}, fuzzyColor, a.fontNormal, DT_CENTER|DT_VCENTER|DT_SINGLELINE)

		drawTextRect(dc, "✕", rect{a.s(214), 0, barW - a.s(2), barH}, theme.IndexFg, a.fontSmall, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	})
}

func (a *App) paintCandidateBar(hwnd uintptr) {
	var ps paintStruct
	hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	defer procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))

	var wr rect
	_, _, _ = procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr)))
	w := wr.Right - wr.Left
	h := wr.Bottom - wr.Top

	withDoubleBuffer(hdc, w, h, func(dc uintptr) {
		theme := a.eng.Config.CurrentTheme()
		fillAndFrame(dc, rect{0, 0, w, h}, theme.Bg, theme.Border)

		state := a.eng.GetViewState()
		modeTag := "混"
		if state.SmartMode == "email" {
			modeTag = "邮箱"
		} else if state.SmartMode == "url" {
			modeTag = "网址"
		} else if state.TempEnglish {
			modeTag = "临时英文"
		} else if state.InputMode == config.ModeWubi {
			modeTag = "五笔"
		} else if state.InputMode == config.ModePinyin {
			modeTag = "拼音"
		}
		fuzzyTag := ""
		if state.FuzzyEnabled && !state.TempEnglish && state.SmartMode == "" && state.InputMode != config.ModeWubi {
			fuzzyTag = "·糊"
		}
		headerText := fmt.Sprintf("%s  [%s%s]", state.Buffer, modeTag, fuzzyTag)
		drawTextRect(dc, headerText, rect{a.s(10), a.s(4), w - a.s(54), a.s(30)}, theme.CodeFg, a.fontBold, DT_LEFT|DT_VCENTER|DT_SINGLELINE)

		pageText := fmt.Sprintf("%d/%d", state.Page+1, state.TotalPages)
		drawTextRect(dc, pageText, rect{w - a.s(54), a.s(4), w - a.s(8), a.s(30)}, theme.IndexFg, a.fontSmall, DT_CENTER|DT_VCENTER|DT_SINGLELINE)

		fillRectColor(dc, rect{a.s(8), a.s(32), w - a.s(8), a.s(33)}, theme.Divider)

		a.candHits = a.candHits[:0]
		if len(state.PageCandidates) == 0 {
			drawTextRect(dc, "（无匹配词条，Enter 直接上屏）", rect{a.s(10), a.s(35), w - a.s(10), h - a.s(4)}, theme.IndexFg, a.fontSmall, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
			return
		}

		if a.eng.Config.HorizontalCandidates {
			curX := a.s(8)
			for idx, c := range state.PageCandidates {
				idxStr := fmt.Sprintf("%d.", idx+1)
				wIdx := measureTextWidth(dc, idxStr, a.fontSmall)
				wTxt := measureTextWidth(dc, c.Text, a.fontNormal)
				wHint := int32(0)
				if c.Hint != "" {
					wHint = measureTextWidth(dc, " "+c.Hint, a.fontSmall)
				}
				boxW := wIdx + wTxt + wHint + a.s(16)
				itemRc := rect{curX, a.s(36), curX + boxW, h - a.s(6)}
				isSel := idx == state.SelectedIndex
				fg := theme.TextFg
				if isSel {
					fillRectColor(dc, itemRc, theme.ActiveBg)
					fg = theme.ActiveFg
				}
				drawTextRect(dc, idxStr, rect{curX + a.s(4), a.s(36), curX + a.s(4) + wIdx + a.s(4), h - a.s(6)}, theme.IndexFg, a.fontSmall, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
				drawTextRect(dc, c.Text, rect{curX + a.s(6) + wIdx, a.s(36), curX + a.s(6) + wIdx + wTxt + a.s(4), h - a.s(6)}, fg, a.fontNormal, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
				if c.Hint != "" {
					drawTextRect(dc, " "+c.Hint, rect{curX + a.s(6) + wIdx + wTxt, a.s(36), itemRc.Right, h - a.s(6)}, theme.HintFg, a.fontSmall, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
				}
				a.candHits = append(a.candHits, candHitBox{rc: itemRc, index: idx})
				curX += boxW + a.s(4)
			}
		} else {
			curY := a.s(36)
			rowH := a.s(26)
			for idx, c := range state.PageCandidates {
				itemRc := rect{a.s(8), curY, w - a.s(8), curY + rowH}
				isSel := idx == state.SelectedIndex
				fg := theme.TextFg
				if isSel {
					fillRectColor(dc, itemRc, theme.ActiveBg)
					fg = theme.ActiveFg
				}
				line := fmt.Sprintf("%d. %s", idx+1, c.Text)
				if c.Hint != "" {
					line += "  " + c.Hint
				}
				drawTextRect(dc, line, rect{a.s(14), curY, w - a.s(12), curY + rowH}, fg, a.fontNormal, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
				a.candHits = append(a.candHits, candHitBox{rc: itemRc, index: idx})
				curY += a.s(28)
			}
		}
	})
}

func appendMenuItem(menu uintptr, flags uint32, id uintptr, text string) {
	if flags == MF_SEPARATOR {
		_, _, _ = procAppendMenuW.Call(menu, MF_SEPARATOR, 0, 0)
		return
	}
	u16, _ := syscall.UTF16PtrFromString(text)
	_, _, _ = procAppendMenuW.Call(menu, uintptr(flags), id, uintptr(unsafe.Pointer(u16)))
}

func checkedFlag(on bool) uint32 {
	if on {
		return MF_STRING | MF_CHECKED
	}
	return MF_STRING
}

func (a *App) showContextMenu(hwnd uintptr) {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)

	cfg := a.eng.Config
	appendMenuItem(menu, checkedFlag(cfg.InputMode == config.ModeMixed), cmdModeMixed, "五拼混输（参考万能五笔）")
	appendMenuItem(menu, checkedFlag(cfg.InputMode == config.ModeWubi), cmdModeWubi, "纯五笔86模式")
	appendMenuItem(menu, checkedFlag(cfg.InputMode == config.ModePinyin), cmdModePinyin, "纯拼音模式")
	appendMenuItem(menu, MF_SEPARATOR, 0, "")
	appendMenuItem(menu, checkedFlag(cfg.FuzzyEnabled), cmdFuzzyMaster, "启用拼音模糊音总开关 (Ctrl+Shift+F)")
	appendMenuItem(menu, MF_STRING, cmdFuzzySouth, "一键套用：南方常用模糊音预设")
	appendMenuItem(menu, MF_STRING, cmdFuzzyAllOn, "一键开启全部 12 组模糊音")
	appendMenuItem(menu, MF_STRING, cmdFuzzyAllOff, "一键关闭全部模糊音")
	for i, item := range fuzzyMenuItems {
		appendMenuItem(menu, checkedFlag(cfg.FuzzyRules[item.key]), uintptr(cmdFuzzyRuleBase+i), item.label)
	}
	appendMenuItem(menu, MF_SEPARATOR, 0, "")
	appendMenuItem(menu, checkedFlag(cfg.AutoCommit4thCode), cmdAuto4, "五笔四码唯一自动上屏")
	appendMenuItem(menu, checkedFlag(cfg.PushCommit5thCode), cmdPush5, "五笔五码顶字上屏")
	appendMenuItem(menu, checkedFlag(cfg.ShowReverseHint), cmdReverseHint, "五笔 ↔ 拼音双向反查编码提示")
	appendMenuItem(menu, checkedFlag(cfg.HorizontalCandidates), cmdHorizToggle, "候选窗横向排列（取消则竖排）")
	appendMenuItem(menu, checkedFlag(cfg.AutoLearnPhrases), cmdAutoLearn, "整句选词自动学习记忆")
	appendMenuItem(menu, checkedFlag(cfg.CtrlSpaceToggle), cmdCtrlSpace, "允许 Ctrl+Space 切换中英文")
	appendMenuItem(menu, MF_SEPARATOR, 0, "")
	appendMenuItem(menu, checkedFlag(cfg.Theme == "classic_light"), cmdThemeLight, "主题：简约白")
	appendMenuItem(menu, checkedFlag(cfg.Theme == "wanneng_blue"), cmdThemeBlue, "主题：万能蓝")
	appendMenuItem(menu, checkedFlag(cfg.Theme == "dark_slate"), cmdThemeDark, "主题：深邃暗")
	appendMenuItem(menu, MF_SEPARATOR, 0, "")
	appendMenuItem(menu, MF_STRING, cmdEditPhrases, "编辑自定义短语 (data/phrases.txt)")
	appendMenuItem(menu, MF_STRING, cmdOpenDataDir, "打开词库目录 (data/)")
	appendMenuItem(menu, MF_STRING, cmdReloadDict, "重新加载词库与短语")
	appendMenuItem(menu, MF_STRING, cmdExitApp, "退出 Qin书输入法")

	var pt point
	_, _, _ = procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	_, _, _ = procSetForegroundWindow.Call(hwnd)
	cmd, _, _ := procTrackPopupMenu.Call(
		menu,
		TPM_LEFTALIGN|TPM_BOTTOMALIGN|TPM_RETURNCMD,
		uintptr(pt.X), uintptr(pt.Y),
		0, hwnd, 0,
	)
	a.handleMenuCommand(int(cmd))
}

func (a *App) handleMenuCommand(cmd int) {
	if cmd == 0 {
		return
	}
	cfg := a.eng.Config
	switch {
	case cmd == cmdModeMixed:
		a.eng.SetInputMode(config.ModeMixed)
	case cmd == cmdModeWubi:
		a.eng.SetInputMode(config.ModeWubi)
	case cmd == cmdModePinyin:
		a.eng.SetInputMode(config.ModePinyin)
	case cmd == cmdFuzzyMaster:
		a.eng.ToggleFuzzy()
	case cmd == cmdFuzzySouth:
		cfg.ApplySouthPreset()
		config.SaveAsync(cfg, a.baseDir)
	case cmd == cmdFuzzyAllOn:
		cfg.SetAllFuzzy(true)
		config.SaveAsync(cfg, a.baseDir)
	case cmd == cmdFuzzyAllOff:
		cfg.SetAllFuzzy(false)
		config.SaveAsync(cfg, a.baseDir)
	case cmd >= cmdFuzzyRuleBase && cmd < cmdFuzzyRuleBase+len(fuzzyMenuItems):
		key := fuzzyMenuItems[cmd-cmdFuzzyRuleBase].key
		cfg.FuzzyRules[key] = !cfg.FuzzyRules[key]
		cfg.FuzzyEnabled = true
		config.SaveAsync(cfg, a.baseDir)
	case cmd == cmdAuto4:
		cfg.AutoCommit4thCode = !cfg.AutoCommit4thCode
		config.SaveAsync(cfg, a.baseDir)
	case cmd == cmdPush5:
		cfg.PushCommit5thCode = !cfg.PushCommit5thCode
		config.SaveAsync(cfg, a.baseDir)
	case cmd == cmdReverseHint:
		cfg.ShowReverseHint = !cfg.ShowReverseHint
		config.SaveAsync(cfg, a.baseDir)
	case cmd == cmdHorizToggle:
		cfg.HorizontalCandidates = !cfg.HorizontalCandidates
		config.SaveAsync(cfg, a.baseDir)
	case cmd == cmdAutoLearn:
		cfg.AutoLearnPhrases = !cfg.AutoLearnPhrases
		config.SaveAsync(cfg, a.baseDir)
	case cmd == cmdCtrlSpace:
		cfg.CtrlSpaceToggle = !cfg.CtrlSpaceToggle
		config.SaveAsync(cfg, a.baseDir)
	case cmd == cmdThemeLight:
		cfg.Theme = "classic_light"
		config.SaveAsync(cfg, a.baseDir)
	case cmd == cmdThemeBlue:
		cfg.Theme = "wanneng_blue"
		config.SaveAsync(cfg, a.baseDir)
	case cmd == cmdThemeDark:
		cfg.Theme = "dark_slate"
		config.SaveAsync(cfg, a.baseDir)
	case cmd == cmdEditPhrases:
		OpenFolderInExplorer(a.eng.Dict.PhrasesPath())
		return
	case cmd == cmdOpenDataDir:
		OpenFolderInExplorer(config.UserDataDir(a.baseDir))
		return
	case cmd == cmdReloadDict:
		a.eng.Dict.Reload()
	case cmd == cmdExitApp:
		_, _, _ = procPostQuitMessage.Call(0)
		return
	}
	a.refreshAllWindows()
}

func lowWord(lp uintptr) int32 {
	return int32(int16(lp & 0xFFFF))
}

func highWord(lp uintptr) int32 {
	return int32(int16((lp >> 16) & 0xFFFF))
}

func statusWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	a := activeApp
	if a == nil {
		r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
		return r
	}
	switch msg {
	case WM_MOUSEACTIVATE:
		return MA_NOACTIVATE
	case WM_ERASEBKGND:
		return 1
	case wmRefreshUI:
		a.refreshAllWindows()
		return 0
	case WM_PAINT:
		a.paintStatusBar(hwnd)
		return 0
	case WM_LBUTTONDOWN:
		x := lowWord(lParam)
		y := highWord(lParam)
		switch {
		case x < a.s(68):
			a.dragging = true
			a.dragOffsetX = x
			a.dragOffsetY = y
			_, _, _ = procSetCapture.Call(hwnd)
		case x < a.s(104):
			if committed := a.eng.ToggleLanguage(); committed != "" {
				SendUnicodeTextAsync(committed)
			}
			a.refreshAllWindows()
		case x < a.s(140):
			a.eng.CycleInputMode()
			a.refreshAllWindows()
		case x < a.s(176):
			a.eng.TogglePunctuation()
			a.refreshAllWindows()
		case x < a.s(214):
			a.eng.ToggleFuzzy()
			a.refreshAllWindows()
		default:
			_, _, _ = procPostQuitMessage.Call(0)
		}
		return 0
	case WM_MOUSEMOVE:
		if a.dragging {
			var pt point
			_, _, _ = procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
			nx := pt.X - a.dragOffsetX
			ny := pt.Y - a.dragOffsetY
			_, _, _ = procSetWindowPos.Call(hwnd, HWND_TOPMOST, uintptr(nx), uintptr(ny), uintptr(a.s(252)), uintptr(a.s(36)), SWP_NOACTIVATE)
			a.eng.Config.StatusBarX = int(nx)
			a.eng.Config.StatusBarY = int(ny)
			a.eng.Config.HasSavedStatusPos = true
		}
		return 0
	case WM_LBUTTONUP:
		if a.dragging {
			a.dragging = false
			_, _, _ = procReleaseCapture.Call()
			config.SaveAsync(a.eng.Config, a.baseDir)
		}
		return 0
	case WM_RBUTTONUP:
		a.showContextMenu(hwnd)
		return 0
	case WM_DESTROY:
		_, _, _ = procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func candWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	a := activeApp
	if a == nil {
		r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
		return r
	}
	switch msg {
	case WM_MOUSEACTIVATE:
		return MA_NOACTIVATE
	case WM_ERASEBKGND:
		return 1
	case WM_PAINT:
		a.paintCandidateBar(hwnd)
		return 0
	case WM_LBUTTONDOWN:
		x := lowWord(lParam)
		y := highWord(lParam)
		for _, hb := range a.candHits {
			if x >= hb.rc.Left && x <= hb.rc.Right && y >= hb.rc.Top && y <= hb.rc.Bottom {
				if text, ok := a.eng.SelectCandidate(hb.index); ok {
					SendUnicodeTextAsync(text)
				}
				a.refreshAllWindows()
				break
			}
		}
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}
