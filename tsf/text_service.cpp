#include "text_service.h"
#include "pipe_client.h"

#include <utility>

namespace {

// 通过 TSF 原生 ITfInsertAtSelection 将文本直接写入焦点文档上下文。
class CCommitEditSession : public ITfEditSession {
public:
    CCommitEditSession(ITfContext* pic, std::wstring text)
        : m_refCount(1), m_pic(pic), m_text(std::move(text)) {
        if (m_pic) m_pic->AddRef();
    }
    ~CCommitEditSession() {
        if (m_pic) m_pic->Release();
    }

    STDMETHODIMP QueryInterface(REFIID riid, void** ppvObj) override {
        if (!ppvObj) return E_INVALIDARG;
        *ppvObj = nullptr;
        if (IsEqualIID(riid, IID_IUnknown) || IsEqualIID(riid, IID_ITfEditSession)) {
            *ppvObj = static_cast<ITfEditSession*>(this);
            AddRef();
            return S_OK;
        }
        return E_NOINTERFACE;
    }

    STDMETHODIMP_(ULONG) AddRef() override {
        return static_cast<ULONG>(InterlockedIncrement(&m_refCount));
    }

    STDMETHODIMP_(ULONG) Release() override {
        LONG r = InterlockedDecrement(&m_refCount);
        if (r == 0) delete this;
        return static_cast<ULONG>(r);
    }

    STDMETHODIMP DoEditSession(TfEditCookie ec) override {
        if (!m_pic || m_text.empty()) return S_OK;
        ITfInsertAtSelection* pInsert = nullptr;
        if (SUCCEEDED(m_pic->QueryInterface(IID_ITfInsertAtSelection, reinterpret_cast<void**>(&pInsert))) && pInsert) {
            ITfRange* pRange = nullptr;
            if (SUCCEEDED(pInsert->InsertTextAtSelection(
                    ec, 0, m_text.c_str(), static_cast<LONG>(m_text.size()), &pRange)) && pRange) {
                pRange->Collapse(ec, TF_ANCHOR_END);
                TF_SELECTION sel = {};
                sel.range = pRange;
                sel.style.ase = TF_AE_NONE;
                sel.style.fInterimChar = FALSE;
                m_pic->SetSelection(ec, 1, &sel);
                pRange->Release();
            }
            pInsert->Release();
        }
        return S_OK;
    }

private:
    LONG m_refCount;
    ITfContext* m_pic;
    std::wstring m_text;
};

// 通过 TSF 原生 ITfContextView::GetTextExt 获取精准光标屏幕坐标。
class CCaretPosEditSession : public ITfEditSession {
public:
    CCaretPosEditSession(ITfContext* pic, POINT* outPt, bool* outOk)
        : m_refCount(1), m_pic(pic), m_outPt(outPt), m_outOk(outOk) {
        if (m_pic) m_pic->AddRef();
    }
    ~CCaretPosEditSession() {
        if (m_pic) m_pic->Release();
    }

    STDMETHODIMP QueryInterface(REFIID riid, void** ppvObj) override {
        if (!ppvObj) return E_INVALIDARG;
        *ppvObj = nullptr;
        if (IsEqualIID(riid, IID_IUnknown) || IsEqualIID(riid, IID_ITfEditSession)) {
            *ppvObj = static_cast<ITfEditSession*>(this);
            AddRef();
            return S_OK;
        }
        return E_NOINTERFACE;
    }

    STDMETHODIMP_(ULONG) AddRef() override {
        return static_cast<ULONG>(InterlockedIncrement(&m_refCount));
    }

    STDMETHODIMP_(ULONG) Release() override {
        LONG r = InterlockedDecrement(&m_refCount);
        if (r == 0) delete this;
        return static_cast<ULONG>(r);
    }

    STDMETHODIMP DoEditSession(TfEditCookie ec) override {
        if (!m_pic) return S_OK;
        TF_SELECTION sel = {};
        ULONG fetched = 0;
        if (SUCCEEDED(m_pic->GetSelection(ec, TF_DEFAULT_SELECTION, 1, &sel, &fetched)) && fetched > 0 && sel.range) {
            ITfContextView* pView = nullptr;
            if (SUCCEEDED(m_pic->GetActiveView(&pView)) && pView) {
                RECT rc = {};
                BOOL clipped = FALSE;
                if (SUCCEEDED(pView->GetTextExt(ec, sel.range, &rc, &clipped))) {
                    if (rc.left != 0 || rc.bottom != 0) {
                        m_outPt->x = rc.left;
                        m_outPt->y = rc.bottom + 6;
                        *m_outOk = true;
                    }
                }
                pView->Release();
            }
            sel.range->Release();
        }
        return S_OK;
    }

private:
    LONG m_refCount;
    ITfContext* m_pic;
    POINT* m_outPt;
    bool* m_outOk;
};

bool IsKeyDown(int vk) {
    return (GetAsyncKeyState(vk) & 0x8000) != 0;
}

wchar_t TranslateKeyToChar(UINT vk, bool shift) {
    BYTE kbState[256] = {};
    if (!GetKeyboardState(kbState)) {
        return 0;
    }
    // 显式对齐 Shift 与 CapsLock 状态，避免宿主缓存滞后导致大小写或标点错乱
    kbState[VK_SHIFT] = shift ? 0x80 : 0;
    kbState[VK_LSHIFT] = 0;
    kbState[VK_RSHIFT] = 0;
    kbState[VK_CONTROL] = 0;
    kbState[VK_LCONTROL] = 0;
    kbState[VK_RCONTROL] = 0;
    kbState[VK_MENU] = 0;
    kbState[VK_LMENU] = 0;
    kbState[VK_RMENU] = 0;
    wchar_t buf[4] = {};
    int ret = ToUnicode(vk, MapVirtualKeyW(vk, MAPVK_VK_TO_VSC), kbState, buf, 4, 0);
    if (ret == 1) {
        return buf[0];
    }
    return 0;
}

bool IsCandidateKey(UINT vk) {
    if (vk >= 'A' && vk <= 'Z') return true;
    if (vk >= '0' && vk <= '9') return true;
    switch (vk) {
        case VK_SPACE:
        case VK_RETURN:
        case VK_BACK:
        case VK_ESCAPE:
        case VK_DELETE:
        case VK_LEFT:
        case VK_RIGHT:
        case VK_UP:
        case VK_DOWN:
        case VK_PRIOR:
        case VK_NEXT:
        case VK_OEM_1:
        case VK_OEM_PLUS:
        case VK_OEM_COMMA:
        case VK_OEM_MINUS:
        case VK_OEM_PERIOD:
        case VK_OEM_2:
        case VK_OEM_3:
        case VK_OEM_4:
        case VK_OEM_5:
        case VK_OEM_6:
        case VK_OEM_7:
            return true;
        default:
            return false;
    }
}

thread_local bool g_hasPendingResp = false;
thread_local UINT g_pendingVk = 0;
thread_local PipeResponse g_pendingResp;

} // namespace

CQinShuTextService::CQinShuTextService()
    : m_refCount(1),
      m_pThreadMgr(nullptr),
      m_tfClientId(TF_CLIENTID_NULL),
      m_threadMgrEventSinkCookie(TF_INVALID_COOKIE),
      m_shiftDownAlone(false) {
    InterlockedIncrement(&g_dllRefCount);
}

CQinShuTextService::~CQinShuTextService() {
    InterlockedDecrement(&g_dllRefCount);
}

STDMETHODIMP CQinShuTextService::QueryInterface(REFIID riid, void** ppvObj) {
    if (!ppvObj) return E_INVALIDARG;
    *ppvObj = nullptr;
    if (IsEqualIID(riid, IID_IUnknown) ||
        IsEqualIID(riid, IID_ITfTextInputProcessor) ||
        IsEqualIID(riid, IID_ITfTextInputProcessorEx)) {
        *ppvObj = static_cast<ITfTextInputProcessorEx*>(this);
    } else if (IsEqualIID(riid, IID_ITfThreadMgrEventSink)) {
        *ppvObj = static_cast<ITfThreadMgrEventSink*>(this);
    } else if (IsEqualIID(riid, IID_ITfKeyEventSink)) {
        *ppvObj = static_cast<ITfKeyEventSink*>(this);
    }
    if (*ppvObj) {
        AddRef();
        return S_OK;
    }
    return E_NOINTERFACE;
}

STDMETHODIMP_(ULONG) CQinShuTextService::AddRef() {
    return static_cast<ULONG>(InterlockedIncrement(&m_refCount));
}

STDMETHODIMP_(ULONG) CQinShuTextService::Release() {
    LONG r = InterlockedDecrement(&m_refCount);
    if (r == 0) delete this;
    return static_cast<ULONG>(r);
}

STDMETHODIMP CQinShuTextService::Activate(ITfThreadMgr* ptim, TfClientId tid) {
    return ActivateEx(ptim, tid, 0);
}

STDMETHODIMP CQinShuTextService::ActivateEx(ITfThreadMgr* ptim, TfClientId tid, DWORD /*dwFlags*/) {
    m_pThreadMgr = ptim;
    if (m_pThreadMgr) m_pThreadMgr->AddRef();
    m_tfClientId = tid;

    if (!InitThreadMgrEventSink() || !InitKeyEventSink()) {
        Deactivate();
        return E_FAIL;
    }
    return S_OK;
}

STDMETHODIMP CQinShuTextService::Deactivate() {
    UninitKeyEventSink();
    UninitThreadMgrEventSink();
    if (m_pThreadMgr) {
        m_pThreadMgr->Release();
        m_pThreadMgr = nullptr;
    }
    m_tfClientId = TF_CLIENTID_NULL;
    return S_OK;
}

bool CQinShuTextService::InitThreadMgrEventSink() {
    ITfSource* pSource = nullptr;
    if (FAILED(m_pThreadMgr->QueryInterface(IID_ITfSource, reinterpret_cast<void**>(&pSource))) || !pSource) {
        return false;
    }
    HRESULT hr = pSource->AdviseSink(
        IID_ITfThreadMgrEventSink,
        static_cast<ITfThreadMgrEventSink*>(this),
        &m_threadMgrEventSinkCookie
    );
    pSource->Release();
    return SUCCEEDED(hr);
}

void CQinShuTextService::UninitThreadMgrEventSink() {
    if (m_threadMgrEventSinkCookie == TF_INVALID_COOKIE || !m_pThreadMgr) return;
    ITfSource* pSource = nullptr;
    if (SUCCEEDED(m_pThreadMgr->QueryInterface(IID_ITfSource, reinterpret_cast<void**>(&pSource))) && pSource) {
        pSource->UnadviseSink(m_threadMgrEventSinkCookie);
        pSource->Release();
    }
    m_threadMgrEventSinkCookie = TF_INVALID_COOKIE;
}

bool CQinShuTextService::InitKeyEventSink() {
    ITfKeystrokeMgr* pKeystrokeMgr = nullptr;
    if (FAILED(m_pThreadMgr->QueryInterface(IID_ITfKeystrokeMgr, reinterpret_cast<void**>(&pKeystrokeMgr))) || !pKeystrokeMgr) {
        return false;
    }
    HRESULT hr = pKeystrokeMgr->AdviseKeyEventSink(m_tfClientId, static_cast<ITfKeyEventSink*>(this), TRUE);
    pKeystrokeMgr->Release();
    return SUCCEEDED(hr);
}

void CQinShuTextService::UninitKeyEventSink() {
    if (!m_pThreadMgr) return;
    ITfKeystrokeMgr* pKeystrokeMgr = nullptr;
    if (SUCCEEDED(m_pThreadMgr->QueryInterface(IID_ITfKeystrokeMgr, reinterpret_cast<void**>(&pKeystrokeMgr))) && pKeystrokeMgr) {
        pKeystrokeMgr->UnadviseKeyEventSink(m_tfClientId);
        pKeystrokeMgr->Release();
    }
}

STDMETHODIMP CQinShuTextService::OnInitDocumentMgr(ITfDocumentMgr*) { return S_OK; }
STDMETHODIMP CQinShuTextService::OnUninitDocumentMgr(ITfDocumentMgr*) { return S_OK; }
STDMETHODIMP CQinShuTextService::OnPushContext(ITfContext*) { return S_OK; }
STDMETHODIMP CQinShuTextService::OnPopContext(ITfContext*) { return S_OK; }

STDMETHODIMP CQinShuTextService::OnSetFocus(ITfDocumentMgr* pdimFocus, ITfDocumentMgr*) {
    if (!pdimFocus) {
        m_shiftDownAlone = false;
        g_hasPendingResp = false;
        g_pendingResp = PipeResponse{};
        SendPipeFocus(false);
    }
    return S_OK;
}

STDMETHODIMP CQinShuTextService::OnSetFocus(BOOL fForeground) {
    if (!fForeground) {
        m_shiftDownAlone = false;
        g_hasPendingResp = false;
        g_pendingResp = PipeResponse{};
        SendPipeFocus(false);
    }
    return S_OK;
}

POINT CQinShuTextService::GetCaretPos(ITfContext* pic) {
    POINT pt = {400, 300};
    bool ok = false;
    if (pic && m_tfClientId != TF_CLIENTID_NULL) {
        auto* pSession = new CCaretPosEditSession(pic, &pt, &ok);
        HRESULT hrSession = S_OK;
        pic->RequestEditSession(m_tfClientId, pSession, TF_ES_SYNC | TF_ES_READ, &hrSession);
        pSession->Release();
    }
    if (!ok) {
        POINT cur = {};
        if (GetCursorPos(&cur)) {
            pt.x = cur.x + 12;
            pt.y = cur.y + 24;
        }
    }
    return pt;
}

void CQinShuTextService::CommitTextToContext(ITfContext* pic, const std::wstring& text) {
    if (!pic || text.empty() || m_tfClientId == TF_CLIENTID_NULL) return;
    auto* pSession = new CCommitEditSession(pic, text);
    HRESULT hrSession = S_OK;
    pic->RequestEditSession(m_tfClientId, pSession, TF_ES_ASYNCDONTCARE | TF_ES_READWRITE, &hrSession);
    pSession->Release();
}

STDMETHODIMP CQinShuTextService::OnTestKeyDown(ITfContext* pic, WPARAM wParam, LPARAM /*lParam*/, BOOL* pfEaten) {
    if (!pfEaten) return E_INVALIDARG;
    *pfEaten = FALSE;
    g_hasPendingResp = false;

    UINT vk = static_cast<UINT>(wParam & 0xFF);

    // 放行 Win 键组合
    if (IsKeyDown(VK_LWIN) || IsKeyDown(VK_RWIN)) {
        m_shiftDownAlone = false;
        return S_OK;
    }

    // Shift 单击追踪（不吞 Shift 键本身，避免宿主程序 Shift 粘滞）
    if (vk == VK_SHIFT || vk == VK_LSHIFT || vk == VK_RSHIFT) {
        m_shiftDownAlone = true;
        return S_OK;
    }
    m_shiftDownAlone = false;

    if (!IsCandidateKey(vk)) {
        return S_OK;
    }

    bool ctrl = IsKeyDown(VK_CONTROL) || IsKeyDown(VK_LCONTROL) || IsKeyDown(VK_RCONTROL);
    bool alt = IsKeyDown(VK_MENU) || IsKeyDown(VK_LMENU) || IsKeyDown(VK_RMENU);
    bool shift = IsKeyDown(VK_SHIFT) || IsKeyDown(VK_LSHIFT) || IsKeyDown(VK_RSHIFT);

    if (alt) {
        return S_OK;
    }

    // CapsLock 开启时由 Go 后端判断（无编码缓冲时直接放行大写英文）
    wchar_t ch = TranslateKeyToChar(vk, shift);
    POINT pt = GetCaretPos(pic);
    PipeResponse resp = SendPipeKeyEvent(vk, ch, ctrl, alt, shift, pt.x, pt.y);

    if (resp.action != PipeAction::PassThrough) {
        *pfEaten = TRUE;
        g_hasPendingResp = true;
        g_pendingVk = vk;
        g_pendingResp = std::move(resp);
    }
    return S_OK;
}

STDMETHODIMP CQinShuTextService::OnKeyDown(ITfContext* pic, WPARAM wParam, LPARAM /*lParam*/, BOOL* pfEaten) {
    if (!pfEaten) return E_INVALIDARG;
    *pfEaten = FALSE;

    UINT vk = static_cast<UINT>(wParam & 0xFF);
    if (g_hasPendingResp && g_pendingVk == vk) {
        g_hasPendingResp = false;
        *pfEaten = TRUE;
        if (g_pendingResp.action == PipeAction::Commit && !g_pendingResp.commitText.empty()) {
            CommitTextToContext(pic, g_pendingResp.commitText);
        }
        g_pendingResp = PipeResponse{};
    }
    return S_OK;
}

STDMETHODIMP CQinShuTextService::OnTestKeyUp(ITfContext* pic, WPARAM wParam, LPARAM /*lParam*/, BOOL* pfEaten) {
    if (!pfEaten) return E_INVALIDARG;
    // 绝不吞掉 Shift 的 KeyUp，防止宿主应用（Chrome/VS Code/记事本）Shift 状态粘滞！
    *pfEaten = FALSE;
    UINT vk = static_cast<UINT>(wParam & 0xFF);
    if ((vk == VK_SHIFT || vk == VK_LSHIFT || vk == VK_RSHIFT) && m_shiftDownAlone) {
        m_shiftDownAlone = false;
        POINT pt = GetCaretPos(pic);
        PipeResponse resp = SendPipeShiftTap(pt.x, pt.y);
        if (resp.action == PipeAction::Commit && !resp.commitText.empty()) {
            CommitTextToContext(pic, resp.commitText);
        }
    }
    return S_OK;
}

STDMETHODIMP CQinShuTextService::OnKeyUp(ITfContext* /*pic*/, WPARAM /*wParam*/, LPARAM /*lParam*/, BOOL* pfEaten) {
    if (pfEaten) *pfEaten = FALSE;
    return S_OK;
}

STDMETHODIMP CQinShuTextService::OnPreservedKey(ITfContext*, REFGUID, BOOL* pfEaten) {
    if (pfEaten) *pfEaten = FALSE;
    return S_OK;
}
