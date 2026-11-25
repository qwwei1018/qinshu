#pragma once

#include <windows.h>
#include <msctf.h>
#include <string>

class CQinShuTextService : public ITfTextInputProcessorEx,
                           public ITfThreadMgrEventSink,
                           public ITfKeyEventSink {
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

private:
    bool InitThreadMgrEventSink();
    void UninitThreadMgrEventSink();
    bool InitKeyEventSink();
    void UninitKeyEventSink();

    POINT GetCaretPos(ITfContext* pic);
    void CommitTextToContext(ITfContext* pic, const std::wstring& text);

    LONG m_refCount;
    ITfThreadMgr* m_pThreadMgr;
    TfClientId m_tfClientId;
    DWORD m_threadMgrEventSinkCookie;
    bool m_shiftDownAlone;
};

extern LONG g_dllRefCount;
extern HINSTANCE g_hInst;
