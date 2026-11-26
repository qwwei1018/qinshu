//go:build windows

package win32

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"qinshu/internal/config"
	"qinshu/internal/dict"
	"qinshu/internal/engine"
)

func TestRunAppExitViaCommandAndCloseButton(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	repoData := filepath.Join("..", "..", "data")
	for _, f := range []string{"wubi86.txt", "pinyin.txt", "english.txt", "phrases.txt"} {
		raw, err := os.ReadFile(filepath.Join(repoData, f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dataDir, f), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := config.Default()
	d := dict.New(dataDir)
	eng := engine.New(cfg, d, tmpDir)

	done := make(chan error, 1)
	go func() {
		done <- RunApp(eng, tmpDir, false)
	}()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if activeApp != nil && activeApp.statusHwnd != 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if activeApp == nil || activeApp.statusHwnd == 0 {
		t.Fatalf("statusHwnd not created")
	}

	// Simulate clicking the '×' exit region or posting WM_COMMAND / exit
	_, _, _ = procPostMessageW.Call(activeApp.statusHwnd, WM_LBUTTONDOWN, 1, uintptr(activeApp.s(230)))
	_, _, _ = procPostMessageW.Call(activeApp.statusHwnd, WM_LBUTTONUP, 0, uintptr(activeApp.s(230)))

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunApp returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("RunApp did NOT exit within 3s after exit click!")
	}

	if activeApp != nil {
		t.Fatalf("expected activeApp to be cleared after RunApp exit")
	}

	// Test cmdExitApp menu command in TSF mode as well
	doneTSF := make(chan error, 1)
	go func() {
		doneTSF <- RunApp(eng, tmpDir, true)
	}()

	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if activeApp != nil && activeApp.statusHwnd != 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if activeApp == nil || activeApp.statusHwnd == 0 {
		t.Fatalf("TSF statusHwnd not created")
	}

	// Simulate WM_CLOSE (which invokes requestExit on the window thread, same as cmdExitApp)
	_, _, _ = procPostMessageW.Call(activeApp.statusHwnd, WM_CLOSE, 0, 0)

	select {
	case err := <-doneTSF:
		if err != nil {
			t.Fatalf("RunApp(tsf) returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("RunApp(tsf) did NOT exit within 3s after WM_CLOSE!")
	}
}
