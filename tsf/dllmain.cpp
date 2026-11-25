#include "guids.h"
#include "text_service.h"

#include <olectl.h>
#include <shlwapi.h>
#include <string>

#pragma comment(lib, "ole32.lib")
#pragma comment(lib, "advapi32.lib")
#pragma comment(lib, "shlwapi.lib")
#pragma comment(lib, "uuid.lib")

LONG g_dllRefCount = 0;
HINSTANCE g_hInst = nullptr;

namespace {

static const GUID QINSHU_SUPPORT_CATEGORIES[] = {
    GUID_TFCAT_TIP_KEYBOARD,
    QINSHU_GUID_TFCAT_TIPCAP_IMMERSIVESUPPORT,
    QINSHU_GUID_TFCAT_TIPCAP_SYSTRAYSUPPORT,
    GUID_TFCAT_TIPCAP_UIELEMENTENABLED,
    GUID_TFCAT_TIPCAP_SECUREMODE,
};

class CQinShuClassFactory : public IClassFactory {
public:
    STDMETHODIMP QueryInterface(REFIID riid, void** ppvObj) override {
        if (!ppvObj) return E_INVALIDARG;
        *ppvObj = nullptr;
        if (IsEqualIID(riid, IID_IUnknown) || IsEqualIID(riid, IID_IClassFactory)) {
            *ppvObj = static_cast<IClassFactory*>(this);
            AddRef();
            return S_OK;
        }
        return E_NOINTERFACE;
    }
    STDMETHODIMP_(ULONG) AddRef() override {
        return static_cast<ULONG>(InterlockedIncrement(&g_dllRefCount));
    }
    STDMETHODIMP_(ULONG) Release() override {
        return static_cast<ULONG>(InterlockedDecrement(&g_dllRefCount));
    }
    STDMETHODIMP CreateInstance(IUnknown* pUnkOuter, REFIID riid, void** ppvObj) override {
        if (!ppvObj) return E_INVALIDARG;
        *ppvObj = nullptr;
        if (pUnkOuter) return CLASS_E_NOAGGREGATION;
        auto* pService = new CQinShuTextService();
        HRESULT hr = pService->QueryInterface(riid, ppvObj);
        pService->Release();
        return hr;
    }
    STDMETHODIMP LockServer(BOOL fLock) override {
        if (fLock) InterlockedIncrement(&g_dllRefCount);
        else InterlockedDecrement(&g_dllRefCount);
        return S_OK;
    }
};

CQinShuClassFactory g_classFactory;

std::wstring ClsidToString(REFCLSID clsid) {
    wchar_t buf[64] = {};
    StringFromGUID2(clsid, buf, 64);
    return buf;
}

std::wstring GetDllPath() {
    wchar_t path[MAX_PATH] = {};
    GetModuleFileNameW(g_hInst, path, MAX_PATH);
    return path;
}

std::wstring GetIconPath() {
    wchar_t path[MAX_PATH] = {};
    GetModuleFileNameW(g_hInst, path, MAX_PATH);
    PathRemoveFileSpecW(path);
    std::wstring iconPath = std::wstring(path) + L"\\assets\\qinshu.ico";
    if (GetFileAttributesW(iconPath.c_str()) != INVALID_FILE_ATTRIBUTES) {
        return iconPath;
    }
    return GetDllPath();
}

bool RegisterComServer() {
    std::wstring clsidKey = L"CLSID\\" + ClsidToString(CLSID_QinShuTSF);
    HKEY hKey = nullptr;
    if (RegCreateKeyExW(HKEY_CLASSES_ROOT, clsidKey.c_str(), 0, nullptr, 0, KEY_WRITE, nullptr, &hKey, nullptr) != ERROR_SUCCESS) {
        return false;
    }
    const wchar_t* desc = L"Qin书输入法";
    RegSetValueExW(hKey, nullptr, 0, REG_SZ, reinterpret_cast<const BYTE*>(desc), static_cast<DWORD>((wcslen(desc) + 1) * sizeof(wchar_t)));

    HKEY hInproc = nullptr;
    if (RegCreateKeyExW(hKey, L"InprocServer32", 0, nullptr, 0, KEY_WRITE, nullptr, &hInproc, nullptr) == ERROR_SUCCESS) {
        std::wstring dllPath = GetDllPath();
        RegSetValueExW(hInproc, nullptr, 0, REG_SZ, reinterpret_cast<const BYTE*>(dllPath.c_str()), static_cast<DWORD>((dllPath.size() + 1) * sizeof(wchar_t)));
        const wchar_t* model = L"Apartment";
        RegSetValueExW(hInproc, L"ThreadingModel", 0, REG_SZ, reinterpret_cast<const BYTE*>(model), static_cast<DWORD>((wcslen(model) + 1) * sizeof(wchar_t)));
        RegCloseKey(hInproc);
    }
    RegCloseKey(hKey);
    return true;
}

void UnregisterComServer() {
    std::wstring clsidKey = L"CLSID\\" + ClsidToString(CLSID_QinShuTSF);
    SHDeleteKeyW(HKEY_CLASSES_ROOT, clsidKey.c_str());
}

bool RegisterProfiles() {
    ITfInputProcessorProfileMgr* pProfileMgr = nullptr;
    HRESULT hr = CoCreateInstance(
        CLSID_TF_InputProcessorProfiles, nullptr, CLSCTX_INPROC_SERVER,
        IID_ITfInputProcessorProfileMgr, reinterpret_cast<void**>(&pProfileMgr)
    );
    if (FAILED(hr) || !pProfileMgr) return false;

    const wchar_t* desc = L"Qin书";
    std::wstring iconPath = GetIconPath();
    hr = pProfileMgr->RegisterProfile(
        CLSID_QinShuTSF,
        QINSHU_LANGID,
        GUID_QinShuProfile,
        desc, static_cast<ULONG>(wcslen(desc)),
        iconPath.c_str(), static_cast<ULONG>(iconPath.size()),
        0,
        nullptr, 0,
        TRUE,
        0
    );
    pProfileMgr->Release();
    return SUCCEEDED(hr);
}

void UnregisterProfiles() {
    ITfInputProcessorProfileMgr* pProfileMgr = nullptr;
    if (SUCCEEDED(CoCreateInstance(
            CLSID_TF_InputProcessorProfiles, nullptr, CLSCTX_INPROC_SERVER,
            IID_ITfInputProcessorProfileMgr, reinterpret_cast<void**>(&pProfileMgr))) && pProfileMgr) {
        pProfileMgr->UnregisterProfile(CLSID_QinShuTSF, QINSHU_LANGID, GUID_QinShuProfile, 0);
        pProfileMgr->Release();
    }
}

bool RegisterCategories() {
    ITfCategoryMgr* pCatMgr = nullptr;
    if (FAILED(CoCreateInstance(
            CLSID_TF_CategoryMgr, nullptr, CLSCTX_INPROC_SERVER,
            IID_ITfCategoryMgr, reinterpret_cast<void**>(&pCatMgr))) || !pCatMgr) {
        return false;
    }
    for (const auto& cat : QINSHU_SUPPORT_CATEGORIES) {
        pCatMgr->RegisterCategory(CLSID_QinShuTSF, cat, CLSID_QinShuTSF);
    }
    pCatMgr->Release();
    return true;
}

void UnregisterCategories() {
    ITfCategoryMgr* pCatMgr = nullptr;
    if (SUCCEEDED(CoCreateInstance(
            CLSID_TF_CategoryMgr, nullptr, CLSCTX_INPROC_SERVER,
            IID_ITfCategoryMgr, reinterpret_cast<void**>(&pCatMgr))) && pCatMgr) {
        for (const auto& cat : QINSHU_SUPPORT_CATEGORIES) {
            pCatMgr->UnregisterCategory(CLSID_QinShuTSF, cat, CLSID_QinShuTSF);
        }
        pCatMgr->Release();
    }
}

} // namespace

BOOL WINAPI DllMain(HINSTANCE hInstance, DWORD dwReason, LPVOID) {
    if (dwReason == DLL_PROCESS_ATTACH) {
        g_hInst = hInstance;
        DisableThreadLibraryCalls(hInstance);
    }
    return TRUE;
}

STDAPI DllGetClassObject(REFCLSID rclsid, REFIID riid, void** ppvObj) {
    if (IsEqualCLSID(rclsid, CLSID_QinShuTSF)) {
        return g_classFactory.QueryInterface(riid, ppvObj);
    }
    if (ppvObj) *ppvObj = nullptr;
    return CLASS_E_CLASSNOTAVAILABLE;
}

STDAPI DllCanUnloadNow() {
    return (g_dllRefCount == 0) ? S_OK : S_FALSE;
}

STDAPI DllRegisterServer() {
    HRESULT hrCo = CoInitializeEx(nullptr, COINIT_APARTMENTTHREADED);
    bool ok = RegisterComServer() && RegisterProfiles() && RegisterCategories();
    if (SUCCEEDED(hrCo)) {
        CoUninitialize();
    }
    return ok ? S_OK : E_FAIL;
}

STDAPI DllUnregisterServer() {
    HRESULT hrCo = CoInitializeEx(nullptr, COINIT_APARTMENTTHREADED);
    UnregisterCategories();
    UnregisterProfiles();
    UnregisterComServer();
    if (SUCCEEDED(hrCo)) {
        CoUninitialize();
    }
    return S_OK;
}
