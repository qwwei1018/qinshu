#pragma once

#include <windows.h>
#include <msctf.h>
#include <string>

class CQinShuTextService : public ITfTextInputProcessorEx,
                           public ITfThreadMgrEventSink,
                           public ITfKeyEventSink,
                           public ITfCompartmentEventSink {
public:
    CQinShuTextService();
    ~CQinShuTextService();

    // IUnknown
    STDMETHODIMP QueryInterface(REFIID riid, void** ppvObj) override;
    STDMETHODIMP_(ULONG) AddRef() override;
    STDMETHODIMP_(ULONG) Release() override;

    // ITfTextInputProcessor
    STDMETHODIMP Activate(ITfThreadMgr* ptim, TfClientId tid) override;
    STDMETHODIMP Deactivate() override;

    // ITfTextInputProcessorEx
    STDMETHODIMP ActivateEx(ITfThreadMgr* ptim, TfClientId tid, DWORD dwFlags) override;

    // ITfThreadMgrEventSink
    STDMETHODIMP OnInitDocumentMgr(ITfDocumentMgr* pdim) override;
    STDMETHODIMP OnUninitDocumentMgr(ITfDocumentMgr* pdim) override;
    STDMETHODIMP OnSetFocus(ITfDocumentMgr* pdimFocus, ITfDocumentMgr* pdimPrevFocus) override;
    STDMETHODIMP OnPushContext(ITfContext* pic) override;
    STDMETHODIMP OnPopContext(ITfContext* pic) override;

    // ITfKeyEventSink
    STDMETHODIMP OnSetFocus(BOOL fForeground) override;
    STDMETHODIMP OnTestKeyDown(ITfContext* pic, WPARAM wParam, LPARAM lParam, BOOL* pfEaten) override;
    STDMETHODIMP OnKeyDown(ITfContext* pic, WPARAM wParam, LPARAM lParam, BOOL* pfEaten) override;
    STDMETHODIMP OnTestKeyUp(ITfContext* pic, WPARAM wParam, LPARAM lParam, BOOL* pfEaten) override;
    STDMETHODIMP OnKeyUp(ITfContext* pic, WPARAM wParam, LPARAM lParam, BOOL* pfEaten) override;
    STDMETHODIMP OnPreservedKey(ITfContext* pic, REFGUID rguid, BOOL* pfEaten) override;

    // ITfCompartmentEventSink
    STDMETHODIMP OnChange(REFGUID rguid) override;

private:
    bool InitThreadMgrEventSink();
    void UninitThreadMgrEventSink();
    bool InitKeyEventSink();
    void UninitKeyEventSink();
    void InitCompartmentSink();
    void UninitCompartmentSink();
    // 中文 (0x0804) TIP 必须打开 GUID_COMPARTMENT_KEYBOARD_OPENCLOSE，TSF 才会把按键派发给输入法
    void EnsureKeyboardOpen();

    POINT GetCaretPos(ITfContext* pic);
    void CommitTextToContext(ITfContext* pic, const std::wstring& text);
    // 将按键发送给 Go 引擎；返回 true 表示需要吞掉该键，结果写入 out
    bool ProcessKeyThroughEngine(ITfContext* pic, UINT vk, struct PipeResponse* out);

    LONG m_refCount;
    ITfThreadMgr* m_pThreadMgr;
    TfClientId m_tfClientId;
    DWORD m_threadMgrEventSinkCookie;
    DWORD m_compartmentSinkCookie;
    bool m_shiftDownAlone;
};

extern LONG g_dllRefCount;
extern HINSTANCE g_hInst;
