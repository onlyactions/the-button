package tray

import (
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/onlyactions/the-button/internal/gfx"
	"github.com/onlyactions/the-button/internal/state"
)

// Tray icon via Shell_NotifyIconW, driven by a hidden window whose
// window procedure runs on one locked OS thread. State changes from
// other threads are funneled to that thread via PostMessage, since
// Shell_NotifyIcon must be called from the thread that owns the
// window (and it keeps the icon id consistent).

const (
	wmTrayCallback = 0x8000 + 1 // WM_APP+1: lParam carries the mouse message
	wmUpdateIcon   = 0x8000 + 2 // WM_APP+2: refresh icon+tooltip from state

	nimAdd    = 0
	nimModify = 1
	nimDelete = 2

	nifMessage = 0x1
	nifIcon    = 0x2
	nifTip     = 0x4

	wmDestroy     = 0x0002
	wmClose       = 0x0010
	wmContextmenu = 0x007B
	wmLbuttonup   = 0x0202
	wmRbuttonup   = 0x0205

	tpmRightbutton = 0x0002
	tpmReturcmd    = 0x0100

	mfString    = 0x0000
	mfSeparator = 0x0800

	swRestore = 9

	menuToggle = 1
	menuShow   = 2
	menuExit   = 3

	iconSize = 32
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")

	pRegisterClassExW       = user32.NewProc("RegisterClassExW")
	pCreateWindowExW        = user32.NewProc("CreateWindowExW")
	pDefWindowProcW         = user32.NewProc("DefWindowProcW")
	pGetMessageW            = user32.NewProc("GetMessageW")
	pTranslateMessage       = user32.NewProc("TranslateMessage")
	pDispatchMessageW       = user32.NewProc("DispatchMessageW")
	pPostQuitMessage        = user32.NewProc("PostQuitMessage")
	pPostMessageW           = user32.NewProc("PostMessageW")
	pCreatePopupMenu        = user32.NewProc("CreatePopupMenu")
	pDestroyMenu            = user32.NewProc("DestroyMenu")
	pAppendMenuW            = user32.NewProc("AppendMenuW")
	pGetCursorPos           = user32.NewProc("GetCursorPos")
	pSetForegroundWindow    = user32.NewProc("SetForegroundWindow")
	pTrackPopupMenu         = user32.NewProc("TrackPopupMenu")
	pDestroyWindow          = user32.NewProc("DestroyWindow")
	pCreateIconIndirect     = user32.NewProc("CreateIconIndirect")
	pDestroyIcon            = user32.NewProc("DestroyIcon")
	pFindWindowW            = user32.NewProc("FindWindowW")
	pShowWindow             = user32.NewProc("ShowWindow")
	pRegisterWindowMessageW = user32.NewProc("RegisterWindowMessageW")
	pGetDC                  = user32.NewProc("GetDC")
	pReleaseDC              = user32.NewProc("ReleaseDC")
	pCreateDIBSection       = user32.NewProc("CreateDIBSection")
	pGetModuleHandleW       = kernel32.NewProc("GetModuleHandleW")
	pShell_NotifyIconW      = shell32.NewProc("Shell_NotifyIconW")
	pDeleteObject           = gdi32.NewProc("DeleteObject")
	pCreateBitmap           = gdi32.NewProc("CreateBitmap")
)

type _POINT struct{ X, Y int32 }

type _MSG struct {
	HWnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       _POINT
	LPrivate uintptr
}

type _WNDCLASSEXW struct {
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

type _BITMAPINFOHEADER struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

type _BITMAPINFO struct {
	BmiHeader _BITMAPINFOHEADER
	_         [1]uint32
}

type _ICONINFO struct {
	FIcon    int32
	XHotspot uint32
	YHotspot uint32
	HbmMask  uintptr
	HbmColor uintptr
}

type _NOTIFYICONDATA struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         windows.GUID
	HBalloonIcon     uintptr
}

type tray struct {
	st    *state.State
	title string

	hwnd            uintptr
	iconOn, iconOff uintptr
	taskbarCreated  uint32 // "TaskbarCreated" message: explorer restarted
}

func run(st *state.State, windowTitle string) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	t := &tray{st: st, title: windowTitle}
	if err := t.init(); err != nil {
		return // no tray in this session; the app keeps running
	}
	defer t.shutdown()
	t.loop()
}

func (t *tray) init() error {
	className, err := windows.UTF16PtrFromString("the-button-tray")
	if err != nil {
		return err
	}
	hInst, _, _ := pGetModuleHandleW.Call(0)

	wc := _WNDCLASSEXW{
		LpfnWndProc:   windows.NewCallback(t.wndProc),
		LpszClassName: className,
		HInstance:     hInst,
	}
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	atom, _, _ := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if atom == 0 {
		return fmt.Errorf("RegisterClassExW failed")
	}

	// A hidden window: never shown, exists only to receive tray messages.
	hwnd, _, _ := pCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		0, 0, 0, 0, 0, 0, 0, 0, hInst, 0)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowExW failed")
	}
	t.hwnd = hwnd

	if r, _, _ := pRegisterWindowMessageW.Call(
		uintptr(unsafe.Pointer(mustUTF16("TaskbarCreated")))); r != 0 {
		t.taskbarCreated = uint32(r)
	}

	var errIcon error
	t.iconOff, errIcon = makeIcon(iconSize, false)
	if errIcon != nil {
		return errIcon
	}
	t.iconOn, errIcon = makeIcon(iconSize, true)
	if errIcon != nil {
		pDestroyIcon.Call(t.iconOff)
		return errIcon
	}

	if !t.notify(nimAdd) {
		pDestroyIcon.Call(t.iconOff)
		pDestroyIcon.Call(t.iconOn)
		return fmt.Errorf("Shell_NotifyIcon NIM_ADD failed")
	}

	// State changes from any thread (window click, tile, widget)
	// re-color the tray icon on the tray's own thread.
	t.st.OnChange(func(bool) {
		pPostMessageW.Call(t.hwnd, wmUpdateIcon, 0, 0)
	})
	return nil
}

func (t *tray) loop() {
	var msg _MSG
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if r == 0 || r == 0xFFFFFFFF { // WM_QUIT or error
			return
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func (t *tray) shutdown() {
	t.notify(nimDelete)
	pDestroyWindow.Call(t.hwnd)
	pDestroyIcon.Call(t.iconOff)
	pDestroyIcon.Call(t.iconOn)
}

func (t *tray) wndProc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	switch msg {
	case wmTrayCallback:
		switch lparam {
		case wmLbuttonup:
			t.st.Toggle() // listener posts wmUpdateIcon back to this thread
			return 0
		case wmRbuttonup, wmContextmenu:
			t.menu(hwnd)
			return 0
		}
	case wmUpdateIcon:
		t.notify(nimModify)
		return 0
	case t.taskbarCreated:
		t.notify(nimAdd) // explorer restarted: icon vanished, re-add
		return 0
	case wmDestroy:
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wparam, lparam)
	return r
}

// menu shows the context menu and dispatches the chosen command.
func (t *tray) menu(hwnd uintptr) {
	hmenu, _, _ := pCreatePopupMenu.Call()
	if hmenu == 0 {
		return
	}
	defer pDestroyMenu.Call(hmenu)

	if t.st.IsOn() {
		appendMenu(hmenu, mfString, menuToggle, "turn off")
	} else {
		appendMenu(hmenu, mfString, menuToggle, "turn on")
	}
	appendMenu(hmenu, mfSeparator, 0, "")
	appendMenu(hmenu, mfString, menuShow, "show window")
	appendMenu(hmenu, mfString, menuExit, "quit")

	var pt _POINT
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	// Required, or the menu won't dismiss when clicking elsewhere.
	pSetForegroundWindow.Call(hwnd)

	cmd, _, _ := pTrackPopupMenu.Call(
		hmenu,
		tpmRightbutton|tpmReturcmd,
		uintptr(uint32(pt.X)), uintptr(uint32(pt.Y)),
		0, hwnd, 0)

	switch cmd {
	case menuToggle:
		t.st.Toggle()
	case menuShow:
		t.showWindow()
	case menuExit:
		t.quitApp()
	}
}

// showWindow raises the Gio window by its title.
func (t *tray) showWindow() {
	if hwnd, _, _ := pFindWindowW.Call(0, uintptr(unsafe.Pointer(mustUTF16(t.title)))); hwnd != 0 {
		pShowWindow.Call(hwnd, swRestore)
		pSetForegroundWindow.Call(hwnd)
	}
}

// quitApp closes the Gio window, which ends the program (and with it
// the tray).
func (t *tray) quitApp() {
	if hwnd, _, _ := pFindWindowW.Call(0, uintptr(unsafe.Pointer(mustUTF16(t.title)))); hwnd != 0 {
		pPostMessageW.Call(hwnd, wmClose, 0, 0)
	}
}

// notify sends a Shell_NotifyIcon message with the current state's
// icon and tooltip.
func (t *tray) notify(nim uint32) bool {
	var nid _NOTIFYICONDATA
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = t.hwnd
	nid.UID = 1
	nid.UFlags = nifMessage | nifIcon | nifTip
	nid.UCallbackMessage = wmTrayCallback
	nid.HIcon = t.iconOff
	tip := "the button: off"
	if t.st.IsOn() {
		nid.HIcon = t.iconOn
		tip = "the button: on"
	}
	copy(nid.SzTip[:], utf16Slice(tip))
	r, _, _ := pShell_NotifyIconW.Call(uintptr(nim), uintptr(unsafe.Pointer(&nid)))
	return r != 0
}

func appendMenu(hmenu uintptr, flags, id uintptr, text string) {
	var p *uint16
	if text != "" {
		p = mustUTF16(text)
	}
	pAppendMenuW.Call(hmenu, flags, id, uintptr(unsafe.Pointer(p)))
}

func mustUTF16(s string) *uint16 {
	p, err := windows.UTF16PtrFromString(s)
	if err != nil {
		panic(err)
	}
	return p
}

// utf16Slice returns s as NUL-terminated UTF-16.
func utf16Slice(s string) []uint16 {
	p, err := windows.UTF16FromString(s)
	if err != nil {
		panic(err)
	}
	return p
}

// makeIcon draws the button icon and wraps it into an HICON via
// CreateDIBSection + CreateIconIndirect.
func makeIcon(size int, on bool) (uintptr, error) {
	pix := gfx.BGRA(gfx.Icon(size, on, 0.45))

	hdc, _, _ := pGetDC.Call(0)
	if hdc == 0 {
		return 0, fmt.Errorf("GetDC failed")
	}
	defer pReleaseDC.Call(0, hdc)

	bmi := _BITMAPINFO{}
	bmi.BmiHeader.BiSize = uint32(unsafe.Sizeof(bmi.BmiHeader))
	bmi.BmiHeader.BiWidth = int32(size)
	bmi.BmiHeader.BiHeight = -int32(size) // top-down
	bmi.BmiHeader.BiPlanes = 1
	bmi.BmiHeader.BiBitCount = 32

	var bits unsafe.Pointer
	colorBM, _, _ := pCreateDIBSection.Call(
		hdc, uintptr(unsafe.Pointer(&bmi)), 0, /* DIB_RGB_COLORS */
		uintptr(unsafe.Pointer(&bits)), 0, 0)
	if colorBM == 0 {
		return 0, fmt.Errorf("CreateDIBSection failed")
	}
	copy(unsafe.Slice((*uint32)(bits), len(pix)), pix)

	// An all-zero 1bpp mask marks every pixel as opaque; the 32bpp
	// alpha channel carries the real transparency.
	maskBM, _, _ := pCreateBitmap.Call(uintptr(size), uintptr(size), 1, 1, 0)

	ii := _ICONINFO{
		FIcon:    1,
		HbmMask:  maskBM,
		HbmColor: colorBM,
	}
	icon, _, _ := pCreateIconIndirect.Call(uintptr(unsafe.Pointer(&ii)))

	// CreateIconIndirect copies the bitmaps; ours can go.
	pDeleteObject.Call(maskBM)
	pDeleteObject.Call(colorBM)
	if icon == 0 {
		return 0, fmt.Errorf("CreateIconIndirect failed")
	}
	return icon, nil
}
