#include "accessibility.h"

#include <algorithm>
#include <atomic>
#include <cstdint>
#include <cstdlib>
#include <cwctype>
#include <sstream>
#include <unordered_map>
#include <vector>

namespace poem {

struct AccessibilityHost::State {
    mutable std::mutex mutex;
    HWND hwnd = nullptr;
    protocol::SemanticTree tree;
    std::function<void(std::string, std::string, std::string)> actionHandler;
};

namespace {

std::wstring Utf8ToWideUia(const std::string& input) {
    if (input.empty()) return {};
    const int count = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, input.data(), static_cast<int>(input.size()), nullptr, 0);
    if (count <= 0) return {};
    std::wstring output(static_cast<std::size_t>(count), L'\0');
    if (MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, input.data(), static_cast<int>(input.size()), output.data(), count) <= 0) return {};
    return output;
}

std::string WideToUtf8Uia(LPCWSTR input) {
    if (!input || !*input) return {};
    const int count = WideCharToMultiByte(CP_UTF8, WC_ERR_INVALID_CHARS, input, -1, nullptr, 0, nullptr, nullptr);
    if (count <= 1) return {};
    std::string output(static_cast<std::size_t>(count), '\0');
    if (WideCharToMultiByte(CP_UTF8, WC_ERR_INVALID_CHARS, input, -1, output.data(), count, nullptr, nullptr) <= 0) return {};
    output.resize(static_cast<std::size_t>(count - 1));
    return output;
}

int RuneToUtf16Index(const std::string& utf8, int runeIndex) {
    const auto text = Utf8ToWideUia(utf8);
    int runes = 0;
    int index = 0;
    while (index < static_cast<int>(text.size()) && runes < runeIndex) {
        if (text[static_cast<std::size_t>(index)] >= 0xD800 && text[static_cast<std::size_t>(index)] <= 0xDBFF && index+1 < static_cast<int>(text.size())) index += 2;
        else ++index;
        ++runes;
    }
    return index;
}

int ControlTypeForRole(const std::string& role) {
    if (role == "application" || role == "window") return UIA_WindowControlTypeId;
    if (role == "button") return UIA_ButtonControlTypeId;
    if (role == "text") return UIA_TextControlTypeId;
    if (role == "text-field") return UIA_EditControlTypeId;
    if (role == "checkbox") return UIA_CheckBoxControlTypeId;
    if (role == "radio-button") return UIA_RadioButtonControlTypeId;
    if (role == "switch") return UIA_ButtonControlTypeId;
    if (role == "slider") return UIA_SliderControlTypeId;
    if (role == "progress-bar") return UIA_ProgressBarControlTypeId;
    if (role == "list" || role == "list-box") return UIA_ListControlTypeId;
    if (role == "list-item" || role == "option") return UIA_ListItemControlTypeId;
    if (role == "table" || role == "grid") return UIA_DataGridControlTypeId;
    if (role == "row") return UIA_DataItemControlTypeId;
    if (role == "cell" || role == "grid-cell") return UIA_DataItemControlTypeId;
    if (role == "column-header") return UIA_HeaderItemControlTypeId;
    if (role == "tree") return UIA_TreeControlTypeId;
    if (role == "tree-item") return UIA_TreeItemControlTypeId;
    if (role == "tab-list") return UIA_TabControlTypeId;
    if (role == "tab") return UIA_TabItemControlTypeId;
    if (role == "dialog") return UIA_WindowControlTypeId;
    if (role == "menu") return UIA_MenuControlTypeId;
    if (role == "menu-item") return UIA_MenuItemControlTypeId;
    if (role == "combo-box" || role == "date-picker") return UIA_ComboBoxControlTypeId;
    if (role == "separator") return UIA_SeparatorControlTypeId;
    if (role == "status" || role == "alert") return UIA_StatusBarControlTypeId;
    if (role == "toolbar") return UIA_ToolBarControlTypeId;
    if (role == "tooltip") return UIA_ToolTipControlTypeId;
    if (role == "link") return UIA_HyperlinkControlTypeId;
    if (role == "navigation") return UIA_PaneControlTypeId;
    return UIA_GroupControlTypeId;
}

bool HasAction(const protocol::SemanticNode& node, const char* action) {
    return std::find(node.actions.begin(), node.actions.end(), action) != node.actions.end();
}

class SemanticProvider;
IRawElementProviderSimple* CreateSimpleProvider(const std::shared_ptr<AccessibilityHost::State>& state, const std::string& id);
ITextRangeProvider* CreateTextRange(const std::shared_ptr<AccessibilityHost::State>& state, const std::string& id, int start, int end);

class SemanticProvider final : public IRawElementProviderSimple,
                               public IRawElementProviderFragment,
                               public IRawElementProviderFragmentRoot,
                               public IInvokeProvider,
                               public IValueProvider,
                               public IToggleProvider,
                               public ISelectionItemProvider,
                               public IExpandCollapseProvider,
                               public IRangeValueProvider,
                               public ITextProvider,
                               public ISelectionProvider,
                               public IGridProvider,
                               public ITableProvider,
                               public IGridItemProvider,
                               public ITableItemProvider,
                               public IScrollProvider {
  public:
    SemanticProvider(std::shared_ptr<AccessibilityHost::State> state, std::string id)
        : state_(std::move(state)), id_(std::move(id)) {}

    IFACEMETHODIMP QueryInterface(REFIID iid, void** object) override {
        if (!object) return E_INVALIDARG;
        *object = nullptr;
        if (iid == __uuidof(IUnknown) || iid == __uuidof(IRawElementProviderSimple)) {
            *object = static_cast<IRawElementProviderSimple*>(this);
        } else if (iid == __uuidof(IRawElementProviderFragment)) {
            *object = static_cast<IRawElementProviderFragment*>(this);
        } else if (iid == __uuidof(IRawElementProviderFragmentRoot) && IsRoot()) {
            *object = static_cast<IRawElementProviderFragmentRoot*>(this);
        } else if (iid == __uuidof(IInvokeProvider) && Supports("invoke")) {
            *object = static_cast<IInvokeProvider*>(this);
        } else if (iid == __uuidof(IValueProvider) && SupportsValuePattern()) {
            *object = static_cast<IValueProvider*>(this);
        } else if (iid == __uuidof(IToggleProvider) && Supports("invoke") && IsToggleRole()) {
            *object = static_cast<IToggleProvider*>(this);
        } else if (iid == __uuidof(ISelectionItemProvider) && Supports("select")) {
            *object = static_cast<ISelectionItemProvider*>(this);
        } else if (iid == __uuidof(IExpandCollapseProvider) && (Supports("expand") || Supports("collapse"))) {
            *object = static_cast<IExpandCollapseProvider*>(this);
        } else if (iid == __uuidof(IRangeValueProvider) && IsRangeRole()) {
            *object = static_cast<IRangeValueProvider*>(this);
        } else if (iid == __uuidof(ITextProvider) && SupportsTextPattern()) {
            *object = static_cast<ITextProvider*>(this);
        } else if (iid == __uuidof(ISelectionProvider) && SupportsCollectionPattern()) {
            *object = static_cast<ISelectionProvider*>(this);
        } else if (iid == __uuidof(IGridProvider) && SupportsGridPattern()) {
            *object = static_cast<IGridProvider*>(this);
        } else if (iid == __uuidof(ITableProvider) && SupportsTablePattern()) {
            *object = static_cast<ITableProvider*>(this);
        } else if (iid == __uuidof(IGridItemProvider) && SupportsGridItemPattern()) {
            *object = static_cast<IGridItemProvider*>(this);
        } else if (iid == __uuidof(ITableItemProvider) && SupportsTableItemPattern()) {
            *object = static_cast<ITableItemProvider*>(this);
        } else if (iid == __uuidof(IScrollProvider) && SupportsScrollPattern()) {
            *object = static_cast<IScrollProvider*>(this);
        } else {
            return E_NOINTERFACE;
        }
        AddRef();
        return S_OK;
    }
    IFACEMETHODIMP_(ULONG) AddRef() override { return ++refs_; }
    IFACEMETHODIMP_(ULONG) Release() override {
        const ULONG remaining = --refs_;
        if (remaining == 0) delete this;
        return remaining;
    }

    IFACEMETHODIMP get_ProviderOptions(ProviderOptions* options) override {
        if (!options) return E_INVALIDARG;
        *options = static_cast<ProviderOptions>(ProviderOptions_ServerSideProvider | ProviderOptions_UseComThreading);
        return S_OK;
    }
    IFACEMETHODIMP GetPatternProvider(PATTERNID pattern, IUnknown** provider) override {
        if (!provider) return E_INVALIDARG;
        *provider = nullptr;
        if (pattern == UIA_InvokePatternId && Supports("invoke")) *provider = static_cast<IInvokeProvider*>(this);
        else if (pattern == UIA_ValuePatternId && SupportsValuePattern()) *provider = static_cast<IValueProvider*>(this);
        else if (pattern == UIA_TogglePatternId && Supports("invoke") && IsToggleRole()) *provider = static_cast<IToggleProvider*>(this);
        else if (pattern == UIA_SelectionItemPatternId && Supports("select")) *provider = static_cast<ISelectionItemProvider*>(this);
        else if (pattern == UIA_ExpandCollapsePatternId && (Supports("expand") || Supports("collapse"))) *provider = static_cast<IExpandCollapseProvider*>(this);
        else if (pattern == UIA_RangeValuePatternId && IsRangeRole()) *provider = static_cast<IRangeValueProvider*>(this);
        else if (pattern == UIA_TextPatternId && SupportsTextPattern()) *provider = static_cast<ITextProvider*>(this);
        else if (pattern == UIA_SelectionPatternId && SupportsCollectionPattern()) *provider = static_cast<ISelectionProvider*>(this);
        else if (pattern == UIA_GridPatternId && SupportsGridPattern()) *provider = static_cast<IGridProvider*>(this);
        else if (pattern == UIA_TablePatternId && SupportsTablePattern()) *provider = static_cast<ITableProvider*>(this);
        else if (pattern == UIA_GridItemPatternId && SupportsGridItemPattern()) *provider = static_cast<IGridItemProvider*>(this);
        else if (pattern == UIA_TableItemPatternId && SupportsTableItemPattern()) *provider = static_cast<ITableItemProvider*>(this);
        else if (pattern == UIA_ScrollPatternId && SupportsScrollPattern()) *provider = static_cast<IScrollProvider*>(this);
        if (*provider) AddRef();
        return S_OK;
    }
    IFACEMETHODIMP GetPropertyValue(PROPERTYID property, VARIANT* value) override {
        if (!value) return E_INVALIDARG;
        VariantInit(value);
        protocol::SemanticNode node;
        if (!FindNode(node)) return UIA_E_ELEMENTNOTAVAILABLE;
        switch (property) {
        case UIA_ControlTypePropertyId:
            value->vt = VT_I4;
            value->lVal = ControlTypeForRole(node.role);
            break;
        case UIA_NamePropertyId:
            SetString(value, node.name.empty() ? node.value : node.name);
            break;
        case UIA_AutomationIdPropertyId:
            SetString(value, node.id);
            break;
        case UIA_HelpTextPropertyId:
            SetString(value, node.description);
            break;
		case UIA_AccessKeyPropertyId:
			SetString(value, node.accessKey);
			break;
        case UIA_FrameworkIdPropertyId:
            SetString(value, "POEM");
            break;
        case UIA_ClassNamePropertyId:
            SetString(value, node.role);
            break;
        case UIA_IsEnabledPropertyId:
            SetBool(value, (node.state & 1u) == 0);
            break;
        case UIA_HasKeyboardFocusPropertyId:
            SetBool(value, (node.state & (1u << 1)) != 0);
            break;
        case UIA_IsKeyboardFocusablePropertyId:
            SetBool(value, HasAction(node, "focus") && (node.state & 1u) == 0);
            break;
        case UIA_IsOffscreenPropertyId:
            SetBool(value, (node.state & (1u << 9)) != 0);
            break;
        case UIA_ValueValuePropertyId:
            SetString(value, node.value);
            break;
        case UIA_IsPasswordPropertyId:
            SetBool(value, (node.state & (1u << 8)) != 0);
            break;
		case UIA_LabeledByPropertyId:
			if (!node.labeledBy.empty()) SetRelatedElement(value, node.labeledBy.front());
			break;
		case UIA_DescribedByPropertyId:
			SetRelatedElements(value, node.describedBy);
			break;
		case UIA_ControllerForPropertyId:
			SetRelatedElements(value, node.controls);
			break;
		case UIA_FlowsToPropertyId:
			SetRelatedElements(value, node.flowsTo);
			break;
        case UIA_NativeWindowHandlePropertyId:
            if (IsRoot()) {
                value->vt = VT_I4;
                value->lVal = static_cast<LONG>(reinterpret_cast<std::intptr_t>(Window()));
            }
            break;
        default:
            break;
        }
        return S_OK;
    }
    IFACEMETHODIMP get_HostRawElementProvider(IRawElementProviderSimple** provider) override {
        if (!provider) return E_INVALIDARG;
        *provider = nullptr;
        return IsRoot() ? UiaHostProviderFromHwnd(Window(), provider) : S_OK;
    }

    IFACEMETHODIMP Navigate(NavigateDirection direction, IRawElementProviderFragment** provider) override {
        if (!provider) return E_INVALIDARG;
        *provider = nullptr;
        std::string target;
        {
            std::lock_guard<std::mutex> lock(state_->mutex);
            const int index = FindIndexLocked();
            if (index < 0) return UIA_E_ELEMENTNOTAVAILABLE;
            const auto& nodes = state_->tree.nodes;
            const int parent = nodes[static_cast<std::size_t>(index)].parent;
            if (direction == NavigateDirection_Parent && parent >= 0 && parent < static_cast<int>(nodes.size())) {
                target = nodes[static_cast<std::size_t>(parent)].id;
            } else if (direction == NavigateDirection_FirstChild || direction == NavigateDirection_LastChild) {
                if (direction == NavigateDirection_FirstChild) {
                    for (const auto& node : nodes) if (node.parent == index) { target = node.id; break; }
                } else {
                    for (auto it = nodes.rbegin(); it != nodes.rend(); ++it) if (it->parent == index) { target = it->id; break; }
                }
            } else if (direction == NavigateDirection_NextSibling || direction == NavigateDirection_PreviousSibling) {
                if (direction == NavigateDirection_NextSibling) {
                    for (int next = index+1; next < static_cast<int>(nodes.size()); ++next) if (nodes[static_cast<std::size_t>(next)].parent == parent) { target = nodes[static_cast<std::size_t>(next)].id; break; }
                } else {
                    for (int previous = index-1; previous >= 0; --previous) if (nodes[static_cast<std::size_t>(previous)].parent == parent) { target = nodes[static_cast<std::size_t>(previous)].id; break; }
                }
            }
        }
        if (!target.empty()) *provider = CreateFragment(target);
        return S_OK;
    }
    IFACEMETHODIMP GetRuntimeId(SAFEARRAY** runtimeId) override {
        if (!runtimeId) return E_INVALIDARG;
        *runtimeId = SafeArrayCreateVector(VT_I4, 0, 2);
        if (!*runtimeId) return E_OUTOFMEMORY;
        LONG first = 0, second = 1;
        int prefix = UiaAppendRuntimeId;
        std::uint32_t hash = 2166136261u;
        for (const unsigned char ch : id_) { hash ^= ch; hash *= 16777619u; }
        int stable = static_cast<int>(hash & 0x7fffffff);
        SafeArrayPutElement(*runtimeId, &first, &prefix);
        SafeArrayPutElement(*runtimeId, &second, &stable);
        return S_OK;
    }
    IFACEMETHODIMP get_BoundingRectangle(UiaRect* rectangle) override {
        if (!rectangle) return E_INVALIDARG;
        protocol::SemanticNode node;
        if (!FindNode(node)) return UIA_E_ELEMENTNOTAVAILABLE;
        HWND hwnd = Window();
        POINT origin{0, 0};
        ClientToScreen(hwnd, &origin);
        const double scale = static_cast<double>(GetDpiForWindow(hwnd)) / 96.0;
        rectangle->left = origin.x + node.x1*scale;
        rectangle->top = origin.y + node.y1*scale;
        rectangle->width = (node.x2-node.x1)*scale;
        rectangle->height = (node.y2-node.y1)*scale;
        return S_OK;
    }
    IFACEMETHODIMP GetEmbeddedFragmentRoots(SAFEARRAY** roots) override {
        if (!roots) return E_INVALIDARG;
        *roots = nullptr;
        return S_OK;
    }
    IFACEMETHODIMP SetFocus() override {
        HWND hwnd = Window();
        if (!hwnd) return UIA_E_ELEMENTNOTAVAILABLE;
        ::SetFocus(hwnd);
        Dispatch("focus", "");
        return S_OK;
    }
    IFACEMETHODIMP get_FragmentRoot(IRawElementProviderFragmentRoot** root) override {
        if (!root) return E_INVALIDARG;
        *root = nullptr;
        std::string rootId;
        {
            std::lock_guard<std::mutex> lock(state_->mutex);
            if (!state_->tree.nodes.empty()) rootId = state_->tree.nodes.front().id;
        }
        if (rootId.empty()) return UIA_E_ELEMENTNOTAVAILABLE;
        auto* provider = new SemanticProvider(state_, rootId);
        *root = static_cast<IRawElementProviderFragmentRoot*>(provider);
        return S_OK;
    }

    IFACEMETHODIMP ElementProviderFromPoint(double x, double y, IRawElementProviderFragment** provider) override {
        if (!provider) return E_INVALIDARG;
        *provider = nullptr;
        HWND hwnd = Window();
        POINT origin{0, 0};
        ClientToScreen(hwnd, &origin);
        const double scale = static_cast<double>(GetDpiForWindow(hwnd))/96.0;
        const double logicalX = (x-origin.x)/scale;
        const double logicalY = (y-origin.y)/scale;
        std::string target = id_;
        {
            std::lock_guard<std::mutex> lock(state_->mutex);
            for (const auto& node : state_->tree.nodes) {
                if (logicalX >= node.x1 && logicalX < node.x2 && logicalY >= node.y1 && logicalY < node.y2) target = node.id;
            }
        }
        *provider = CreateFragment(target);
        return S_OK;
    }
    IFACEMETHODIMP GetFocus(IRawElementProviderFragment** provider) override {
        if (!provider) return E_INVALIDARG;
        *provider = nullptr;
        std::string focused;
        {
            std::lock_guard<std::mutex> lock(state_->mutex);
            for (const auto& node : state_->tree.nodes) if ((node.state & (1u << 1)) != 0) { focused = node.id; break; }
        }
        if (!focused.empty()) *provider = CreateFragment(focused);
        return S_OK;
    }

    IFACEMETHODIMP Invoke() override { return Dispatch("invoke", ""); }
    IFACEMETHODIMP SetValue(LPCWSTR value) override {
        protocol::SemanticNode node;
        if (!FindNode(node)) return UIA_E_ELEMENTNOTAVAILABLE;
        if ((node.state & (1u << 5)) != 0) return UIA_E_NOTSUPPORTED;
        return Dispatch("set-value", WideToUtf8Uia(value));
    }
    IFACEMETHODIMP get_Value(BSTR* value) override {
        if (!value) return E_INVALIDARG;
        protocol::SemanticNode node;
        if (!FindNode(node)) return UIA_E_ELEMENTNOTAVAILABLE;
        const auto wide = Utf8ToWideUia(node.value);
        *value = SysAllocStringLen(wide.data(), static_cast<UINT>(wide.size()));
        return *value || wide.empty() ? S_OK : E_OUTOFMEMORY;
    }
    IFACEMETHODIMP get_IsReadOnly(BOOL* readOnly) override {
        if (!readOnly) return E_INVALIDARG;
        protocol::SemanticNode node;
        if (!FindNode(node)) return UIA_E_ELEMENTNOTAVAILABLE;
        *readOnly = (node.state & (1u << 5)) != 0 ? TRUE : FALSE;
        return S_OK;
    }
    IFACEMETHODIMP Toggle() override { return Dispatch("invoke", ""); }
    IFACEMETHODIMP get_ToggleState(ToggleState* state) override {
        if (!state) return E_INVALIDARG;
        protocol::SemanticNode node;
        if (!FindNode(node)) return UIA_E_ELEMENTNOTAVAILABLE;
        *state = (node.state & (1u << 3)) != 0 ? ToggleState_On : ToggleState_Off;
        return S_OK;
    }
    IFACEMETHODIMP Select() override { return Dispatch("select", ""); }
    IFACEMETHODIMP AddToSelection() override { return Dispatch("select", ""); }
    IFACEMETHODIMP RemoveFromSelection() override { return UIA_E_INVALIDOPERATION; }
    IFACEMETHODIMP get_IsSelected(BOOL* selected) override {
        if (!selected) return E_INVALIDARG;
        protocol::SemanticNode node;
        if (!FindNode(node)) return UIA_E_ELEMENTNOTAVAILABLE;
        *selected = (node.state & (1u << 2)) != 0 ? TRUE : FALSE;
        return S_OK;
    }
    IFACEMETHODIMP get_SelectionContainer(IRawElementProviderSimple** provider) override {
        if (!provider) return E_INVALIDARG;
        *provider = nullptr;
        std::string parent;
        {
            std::lock_guard<std::mutex> lock(state_->mutex);
            const int index = FindIndexLocked();
            if (index < 0) return UIA_E_ELEMENTNOTAVAILABLE;
            const int parentIndex = state_->tree.nodes[static_cast<std::size_t>(index)].parent;
            if (parentIndex >= 0 && parentIndex < static_cast<int>(state_->tree.nodes.size())) parent = state_->tree.nodes[static_cast<std::size_t>(parentIndex)].id;
        }
        if (!parent.empty()) *provider = static_cast<IRawElementProviderSimple*>(new SemanticProvider(state_, parent));
        return S_OK;
    }
    IFACEMETHODIMP Expand() override { return Dispatch("expand", ""); }
    IFACEMETHODIMP Collapse() override { return Dispatch("collapse", ""); }
    IFACEMETHODIMP get_ExpandCollapseState(ExpandCollapseState* state) override {
        if (!state) return E_INVALIDARG;
        protocol::SemanticNode node;
        if (!FindNode(node)) return UIA_E_ELEMENTNOTAVAILABLE;
        *state = (node.state & (1u << 4)) != 0 ? ExpandCollapseState_Expanded : ExpandCollapseState_Collapsed;
        return S_OK;
    }
    IFACEMETHODIMP SetValue(double value) override {
        std::ostringstream text;
        text << value;
        return Dispatch("set-value", text.str());
    }
    IFACEMETHODIMP get_Value(double* value) override {
        if (!value) return E_INVALIDARG;
        protocol::SemanticNode node;
        if (!FindNode(node)) return UIA_E_ELEMENTNOTAVAILABLE;
        char* end = nullptr;
        *value = std::strtod(node.value.c_str(), &end);
        return end == node.value.c_str() ? UIA_E_NOTSUPPORTED : S_OK;
    }
    IFACEMETHODIMP get_Maximum(double* value) override { return GetRangeValue(value, &protocol::SemanticNode::rangeMax); }
    IFACEMETHODIMP get_Minimum(double* value) override { return GetRangeValue(value, &protocol::SemanticNode::rangeMin); }
    IFACEMETHODIMP get_LargeChange(double* value) override { return GetRangeValue(value, &protocol::SemanticNode::largeChange); }
    IFACEMETHODIMP get_SmallChange(double* value) override { return GetRangeValue(value, &protocol::SemanticNode::smallChange); }
    IFACEMETHODIMP GetSelection(SAFEARRAY** ranges) override {
        if (!ranges) return E_INVALIDARG;
        protocol::SemanticNode node;
        if (!FindNode(node)) return UIA_E_ELEMENTNOTAVAILABLE;
        if (node.hasCollection) {
            std::vector<std::string> selected;
            {
                std::lock_guard<std::mutex> lock(state_->mutex);
                const int container = FindIndexLocked();
                for (int index = 0; index < static_cast<int>(state_->tree.nodes.size()); ++index) {
                    const auto& candidate = state_->tree.nodes[static_cast<std::size_t>(index)];
                    if ((candidate.state & (1u << 2)) != 0 && IsDescendantLocked(index, container)) selected.push_back(candidate.id);
                }
            }
            *ranges = SafeArrayCreateVector(VT_UNKNOWN, 0, static_cast<ULONG>(selected.size()));
            if (!*ranges) return E_OUTOFMEMORY;
            for (LONG index = 0; index < static_cast<LONG>(selected.size()); ++index) {
                auto* provider = CreateSimpleProvider(state_, selected[static_cast<std::size_t>(index)]);
                SafeArrayPutElement(*ranges, &index, provider);
                provider->Release();
            }
            return S_OK;
        }
        if (!node.hasText) return UIA_E_NOTSUPPORTED;
        *ranges = SafeArrayCreateVector(VT_UNKNOWN, 0, 1);
        if (!*ranges) return E_OUTOFMEMORY;
        LONG index = 0;
        ITextRangeProvider* range = CreateTextRange(state_, id_, RuneToUtf16Index(node.value, node.selectionStart), RuneToUtf16Index(node.value, node.selectionEnd));
        SafeArrayPutElement(*ranges, &index, range);
        range->Release();
        return S_OK;
    }
    IFACEMETHODIMP GetVisibleRanges(SAFEARRAY** ranges) override {
        if (!ranges) return E_INVALIDARG;
        protocol::SemanticNode node;
        if (!FindNode(node) || !node.hasText) return UIA_E_ELEMENTNOTAVAILABLE;
        const int length = static_cast<int>(Utf8ToWideUia(node.value).size());
        *ranges = SafeArrayCreateVector(VT_UNKNOWN, 0, 1);
        if (!*ranges) return E_OUTOFMEMORY;
        LONG index = 0;
        ITextRangeProvider* range = CreateTextRange(state_, id_, 0, length);
        SafeArrayPutElement(*ranges, &index, range);
        range->Release();
        return S_OK;
    }
    IFACEMETHODIMP RangeFromChild(IRawElementProviderSimple*, ITextRangeProvider** range) override {
        if (!range) return E_INVALIDARG;
        *range = nullptr;
        return UIA_E_INVALIDOPERATION;
    }
    IFACEMETHODIMP RangeFromPoint(UiaPoint point, ITextRangeProvider** range) override {
        if (!range) return E_INVALIDARG;
        *range = nullptr;
        protocol::SemanticNode node;
        if (!FindNode(node) || !node.hasText) return UIA_E_ELEMENTNOTAVAILABLE;
        HWND hwnd = Window();
        POINT origin{0, 0};
        ClientToScreen(hwnd, &origin);
        const double scale = static_cast<double>(GetDpiForWindow(hwnd)) / 96.0;
        const double logicalX = (point.x-origin.x)/scale - node.x1;
        const int length = static_cast<int>(Utf8ToWideUia(node.value).size());
        const int width = std::max(1, node.x2-node.x1);
        const int position = std::clamp(static_cast<int>(logicalX * length / width), 0, length);
        *range = CreateTextRange(state_, id_, position, position);
        return S_OK;
    }
    IFACEMETHODIMP get_DocumentRange(ITextRangeProvider** range) override {
        if (!range) return E_INVALIDARG;
        *range = nullptr;
        protocol::SemanticNode node;
        if (!FindNode(node) || !node.hasText) return UIA_E_ELEMENTNOTAVAILABLE;
        *range = CreateTextRange(state_, id_, 0, static_cast<int>(Utf8ToWideUia(node.value).size()));
        return S_OK;
    }
    IFACEMETHODIMP get_SupportedTextSelection(SupportedTextSelection* selection) override {
        if (!selection) return E_INVALIDARG;
        *selection = SupportedTextSelection_Single;
        return S_OK;
    }
    IFACEMETHODIMP get_CanSelectMultiple(BOOL* value) override {
        if (!value) return E_INVALIDARG;
        protocol::SemanticNode node;
        if (!FindNode(node) || !node.hasCollection) return UIA_E_ELEMENTNOTAVAILABLE;
        *value = node.canSelectMultiple ? TRUE : FALSE;
        return S_OK;
    }
    IFACEMETHODIMP get_IsSelectionRequired(BOOL* value) override {
        if (!value) return E_INVALIDARG;
        protocol::SemanticNode node;
        if (!FindNode(node) || !node.hasCollection) return UIA_E_ELEMENTNOTAVAILABLE;
        *value = node.selectionRequired ? TRUE : FALSE;
        return S_OK;
    }
    IFACEMETHODIMP GetItem(int row, int column, IRawElementProviderSimple** provider) override {
        if (!provider) return E_INVALIDARG;
        *provider = nullptr;
        std::string target;
        {
            std::lock_guard<std::mutex> lock(state_->mutex);
            const int container = FindIndexLocked();
            if (container < 0) return UIA_E_ELEMENTNOTAVAILABLE;
            const auto& grid = state_->tree.nodes[static_cast<std::size_t>(container)];
            if (!grid.hasGrid || row < 0 || column < 0 || row >= grid.gridRows || column >= grid.gridColumns) return E_INVALIDARG;
            for (int index = 0; index < static_cast<int>(state_->tree.nodes.size()); ++index) {
                const auto& candidate = state_->tree.nodes[static_cast<std::size_t>(index)];
                if (candidate.hasGridItem && candidate.gridRow == row && candidate.gridColumn == column &&
                    (candidate.role == "cell" || candidate.role == "grid-cell") && IsDescendantLocked(index, container)) {
                    target = candidate.id;
                    break;
                }
            }
        }
        if (target.empty()) return UIA_E_ELEMENTNOTAVAILABLE;
        *provider = CreateSimpleProvider(state_, target);
        return S_OK;
    }
    IFACEMETHODIMP get_RowCount(int* value) override { return GetGridMetric(value, true); }
    IFACEMETHODIMP get_ColumnCount(int* value) override { return GetGridMetric(value, false); }
    IFACEMETHODIMP GetRowHeaders(SAFEARRAY** providers) override { return CreateHeaderArray(providers, "row-header", -1); }
    IFACEMETHODIMP GetColumnHeaders(SAFEARRAY** providers) override { return CreateHeaderArray(providers, "column-header", -1); }
    IFACEMETHODIMP get_RowOrColumnMajor(RowOrColumnMajor* value) override {
        if (!value) return E_INVALIDARG;
        *value = RowOrColumnMajor_RowMajor;
        return S_OK;
    }
    IFACEMETHODIMP get_Row(int* value) override { return GetGridItemMetric(value, 0); }
    IFACEMETHODIMP get_Column(int* value) override { return GetGridItemMetric(value, 1); }
    IFACEMETHODIMP get_RowSpan(int* value) override { return GetGridItemMetric(value, 2); }
    IFACEMETHODIMP get_ColumnSpan(int* value) override { return GetGridItemMetric(value, 3); }
    IFACEMETHODIMP get_ContainingGrid(IRawElementProviderSimple** provider) override {
        if (!provider) return E_INVALIDARG;
        *provider = nullptr;
        std::string grid;
        {
            std::lock_guard<std::mutex> lock(state_->mutex);
            int index = FindIndexLocked();
            while (index >= 0 && index < static_cast<int>(state_->tree.nodes.size())) {
                const auto& node = state_->tree.nodes[static_cast<std::size_t>(index)];
                if (node.hasGrid) { grid = node.id; break; }
                index = node.parent;
            }
        }
        if (grid.empty()) return UIA_E_ELEMENTNOTAVAILABLE;
        *provider = CreateSimpleProvider(state_, grid);
        return S_OK;
    }
    IFACEMETHODIMP GetRowHeaderItems(SAFEARRAY** providers) override { return CreateHeaderArray(providers, "row-header", -1); }
    IFACEMETHODIMP GetColumnHeaderItems(SAFEARRAY** providers) override {
        protocol::SemanticNode node;
        if (!FindNode(node) || !node.hasGridItem) return UIA_E_ELEMENTNOTAVAILABLE;
        return CreateHeaderArray(providers, "column-header", node.gridColumn);
    }
    IFACEMETHODIMP Scroll(ScrollAmount horizontal, ScrollAmount vertical) override {
        protocol::SemanticNode node;
        if (!FindNode(node) || !node.hasScroll) return UIA_E_ELEMENTNOTAVAILABLE;
        auto move = [](double current, double view, ScrollAmount amount) {
            if (amount == ScrollAmount_NoAmount) return current;
            const double delta = amount == ScrollAmount_LargeDecrement ? -view :
                                 amount == ScrollAmount_SmallDecrement ? -10.0 :
                                 amount == ScrollAmount_LargeIncrement ? view : 10.0;
            return std::clamp(current+delta, 0.0, 100.0);
        };
        if (horizontal != ScrollAmount_NoAmount && !node.hScrollable) return UIA_E_INVALIDOPERATION;
        if (vertical != ScrollAmount_NoAmount && !node.vScrollable) return UIA_E_INVALIDOPERATION;
        return DispatchScroll(move(node.hScrollPercent, node.hViewSize, horizontal), move(node.vScrollPercent, node.vViewSize, vertical));
    }
    IFACEMETHODIMP SetScrollPercent(double horizontal, double vertical) override {
        protocol::SemanticNode node;
        if (!FindNode(node) || !node.hasScroll) return UIA_E_ELEMENTNOTAVAILABLE;
        if (horizontal != UIA_ScrollPatternNoScroll && (!node.hScrollable || horizontal < 0 || horizontal > 100)) return E_INVALIDARG;
        if (vertical != UIA_ScrollPatternNoScroll && (!node.vScrollable || vertical < 0 || vertical > 100)) return E_INVALIDARG;
        if (horizontal == UIA_ScrollPatternNoScroll) horizontal = node.hScrollPercent;
        if (vertical == UIA_ScrollPatternNoScroll) vertical = node.vScrollPercent;
        return DispatchScroll(horizontal, vertical);
    }
    IFACEMETHODIMP get_HorizontalScrollPercent(double* value) override { return GetScrollMetric(value, 0); }
    IFACEMETHODIMP get_VerticalScrollPercent(double* value) override { return GetScrollMetric(value, 1); }
    IFACEMETHODIMP get_HorizontalViewSize(double* value) override { return GetScrollMetric(value, 2); }
    IFACEMETHODIMP get_VerticalViewSize(double* value) override { return GetScrollMetric(value, 3); }
    IFACEMETHODIMP get_HorizontallyScrollable(BOOL* value) override { return GetScrollable(value, true); }
    IFACEMETHODIMP get_VerticallyScrollable(BOOL* value) override { return GetScrollable(value, false); }

  private:
    static void SetString(VARIANT* value, const std::string& text) {
        const auto wide = Utf8ToWideUia(text);
        value->vt = VT_BSTR;
        value->bstrVal = SysAllocStringLen(wide.data(), static_cast<UINT>(wide.size()));
    }
    static void SetBool(VARIANT* value, bool enabled) {
        value->vt = VT_BOOL;
        value->boolVal = enabled ? VARIANT_TRUE : VARIANT_FALSE;
    }
	void SetRelatedElement(VARIANT* value, const std::string& id) const {
		if (id.empty()) return;
		value->vt = VT_UNKNOWN;
		value->punkVal = CreateSimpleProvider(state_, id);
	}
	void SetRelatedElements(VARIANT* value, const std::vector<std::string>& ids) const {
		if (ids.empty()) return;
		SAFEARRAY* providers = SafeArrayCreateVector(VT_UNKNOWN, 0, static_cast<ULONG>(ids.size()));
		if (!providers) return;
		for (LONG index = 0; index < static_cast<LONG>(ids.size()); ++index) {
			auto* provider = CreateSimpleProvider(state_, ids[static_cast<std::size_t>(index)]);
			if (FAILED(SafeArrayPutElement(providers, &index, provider))) {
				provider->Release();
				SafeArrayDestroy(providers);
				return;
			}
			provider->Release();
		}
		value->vt = VT_ARRAY | VT_UNKNOWN;
		value->parray = providers;
	}
    HWND Window() const {
        std::lock_guard<std::mutex> lock(state_->mutex);
        return state_->hwnd;
    }
    bool IsRoot() const {
        std::lock_guard<std::mutex> lock(state_->mutex);
        return !state_->tree.nodes.empty() && state_->tree.nodes.front().id == id_;
    }
    int FindIndexLocked() const {
        for (std::size_t index = 0; index < state_->tree.nodes.size(); ++index) if (state_->tree.nodes[index].id == id_) return static_cast<int>(index);
        return -1;
    }
    bool IsDescendantLocked(int index, int ancestor) const {
        if (ancestor < 0) return false;
        while (index >= 0 && index < static_cast<int>(state_->tree.nodes.size())) {
            const int parent = state_->tree.nodes[static_cast<std::size_t>(index)].parent;
            if (parent == ancestor) return true;
            index = parent;
        }
        return false;
    }
    bool FindNode(protocol::SemanticNode& node) const {
        std::lock_guard<std::mutex> lock(state_->mutex);
        const int index = FindIndexLocked();
        if (index < 0) return false;
        node = state_->tree.nodes[static_cast<std::size_t>(index)];
        return true;
    }
    bool Supports(const char* action) const {
        protocol::SemanticNode node;
        return FindNode(node) && HasAction(node, action);
    }
    bool IsToggleRole() const {
        protocol::SemanticNode node;
        return FindNode(node) && (node.role == "checkbox" || node.role == "switch");
    }
    bool IsRangeRole() const {
        protocol::SemanticNode node;
        return FindNode(node) && node.role == "slider";
    }
    bool SupportsTextPattern() const {
        protocol::SemanticNode node;
        return FindNode(node) && node.hasText && (node.state & (1u << 8)) == 0;
    }
    bool SupportsValuePattern() const {
        protocol::SemanticNode node;
        return FindNode(node) && (HasAction(node, "set-value") || node.role == "cell" || node.role == "grid-cell") &&
               (node.state & (1u << 8)) == 0;
    }
    bool SupportsCollectionPattern() const {
        protocol::SemanticNode node;
        return FindNode(node) && node.hasCollection;
    }
    bool SupportsGridPattern() const {
        protocol::SemanticNode node;
        return FindNode(node) && node.hasGrid;
    }
    bool SupportsTablePattern() const {
        protocol::SemanticNode node;
        return FindNode(node) && node.hasGrid && node.role == "table";
    }
    bool SupportsGridItemPattern() const {
        protocol::SemanticNode node;
        return FindNode(node) && node.hasGridItem;
    }
    bool SupportsTableItemPattern() const {
        std::lock_guard<std::mutex> lock(state_->mutex);
        int index = FindIndexLocked();
        if (index < 0 || !state_->tree.nodes[static_cast<std::size_t>(index)].hasGridItem) return false;
        while (index >= 0 && index < static_cast<int>(state_->tree.nodes.size())) {
            const auto& node = state_->tree.nodes[static_cast<std::size_t>(index)];
            if (node.hasGrid) return node.role == "table";
            index = node.parent;
        }
        return false;
    }
    bool SupportsScrollPattern() const {
        protocol::SemanticNode node;
        return FindNode(node) && node.hasScroll;
    }
    HRESULT GetGridMetric(int* value, bool rows) const {
        if (!value) return E_INVALIDARG;
        protocol::SemanticNode node;
        if (!FindNode(node) || !node.hasGrid) return UIA_E_ELEMENTNOTAVAILABLE;
        *value = rows ? node.gridRows : node.gridColumns;
        return S_OK;
    }
    HRESULT GetGridItemMetric(int* value, int metric) const {
        if (!value) return E_INVALIDARG;
        protocol::SemanticNode node;
        if (!FindNode(node) || !node.hasGridItem) return UIA_E_ELEMENTNOTAVAILABLE;
        switch (metric) {
        case 0: *value = node.gridRow; break;
        case 1: *value = node.gridColumn; break;
        case 2: *value = std::max(1, node.gridRowSpan); break;
        default: *value = std::max(1, node.gridColumnSpan); break;
        }
        return S_OK;
    }
    HRESULT CreateHeaderArray(SAFEARRAY** providers, const char* role, int column) const {
        if (!providers) return E_INVALIDARG;
        std::vector<std::string> ids;
        {
            std::lock_guard<std::mutex> lock(state_->mutex);
            const int origin = FindIndexLocked();
            int grid = origin;
            while (grid >= 0 && grid < static_cast<int>(state_->tree.nodes.size()) && !state_->tree.nodes[static_cast<std::size_t>(grid)].hasGrid)
                grid = state_->tree.nodes[static_cast<std::size_t>(grid)].parent;
            if (grid < 0) return UIA_E_ELEMENTNOTAVAILABLE;
            int headerColumn = 0;
            for (int index = 0; index < static_cast<int>(state_->tree.nodes.size()); ++index) {
                const auto& candidate = state_->tree.nodes[static_cast<std::size_t>(index)];
                if (candidate.role == role && IsDescendantLocked(index, grid)) {
                    if (column < 0 || headerColumn == column) ids.push_back(candidate.id);
                    ++headerColumn;
                }
            }
        }
        *providers = SafeArrayCreateVector(VT_UNKNOWN, 0, static_cast<ULONG>(ids.size()));
        if (!*providers) return E_OUTOFMEMORY;
        for (LONG index = 0; index < static_cast<LONG>(ids.size()); ++index) {
            auto* provider = CreateSimpleProvider(state_, ids[static_cast<std::size_t>(index)]);
            SafeArrayPutElement(*providers, &index, provider);
            provider->Release();
        }
        return S_OK;
    }
    HRESULT GetScrollMetric(double* value, int metric) const {
        if (!value) return E_INVALIDARG;
        protocol::SemanticNode node;
        if (!FindNode(node) || !node.hasScroll) return UIA_E_ELEMENTNOTAVAILABLE;
        switch (metric) {
        case 0: *value = node.hScrollPercent; break;
        case 1: *value = node.vScrollPercent; break;
        case 2: *value = node.hViewSize; break;
        default: *value = node.vViewSize; break;
        }
        return S_OK;
    }
    HRESULT GetScrollable(BOOL* value, bool horizontal) const {
        if (!value) return E_INVALIDARG;
        protocol::SemanticNode node;
        if (!FindNode(node) || !node.hasScroll) return UIA_E_ELEMENTNOTAVAILABLE;
        *value = (horizontal ? node.hScrollable : node.vScrollable) ? TRUE : FALSE;
        return S_OK;
    }
    HRESULT DispatchScroll(double horizontal, double vertical) const {
        std::ostringstream value;
        value << horizontal << ":" << vertical;
        return Dispatch("set-scroll-percent", value.str());
    }
    HRESULT Dispatch(std::string action, std::string value) const {
        std::function<void(std::string, std::string, std::string)> handler;
        {
            std::lock_guard<std::mutex> lock(state_->mutex);
            if (FindIndexLocked() < 0) return UIA_E_ELEMENTNOTAVAILABLE;
            handler = state_->actionHandler;
        }
        if (!handler) return UIA_E_NOTSUPPORTED;
        handler(id_, std::move(action), std::move(value));
        return S_OK;
    }
    HRESULT GetRangeValue(double* value, double protocol::SemanticNode::* member) const {
        if (!value) return E_INVALIDARG;
        protocol::SemanticNode node;
        if (!FindNode(node)) return UIA_E_ELEMENTNOTAVAILABLE;
        if (!node.hasRange) return UIA_E_NOTSUPPORTED;
        *value = node.*member;
        return S_OK;
    }
    IRawElementProviderFragment* CreateFragment(const std::string& id) const {
        return static_cast<IRawElementProviderFragment*>(new SemanticProvider(state_, id));
    }

    std::atomic<ULONG> refs_{1};
    std::shared_ptr<AccessibilityHost::State> state_;
    std::string id_;
};

class TextRangeProvider final : public ITextRangeProvider {
  public:
    TextRangeProvider(std::shared_ptr<AccessibilityHost::State> state, std::string id, int start, int end)
        : state_(std::move(state)), id_(std::move(id)), start_(start), end_(end) { Normalize(); }

    IFACEMETHODIMP QueryInterface(REFIID iid, void** object) override {
        if (!object) return E_INVALIDARG;
        *object = nullptr;
        if (iid == __uuidof(IUnknown) || iid == __uuidof(ITextRangeProvider)) *object = static_cast<ITextRangeProvider*>(this);
        else return E_NOINTERFACE;
        AddRef();
        return S_OK;
    }
    IFACEMETHODIMP_(ULONG) AddRef() override { return ++refs_; }
    IFACEMETHODIMP_(ULONG) Release() override { const ULONG value = --refs_; if (!value) delete this; return value; }

    IFACEMETHODIMP Clone(ITextRangeProvider** clone) override {
        if (!clone) return E_INVALIDARG;
        *clone = new TextRangeProvider(state_, id_, start_, end_);
        return S_OK;
    }
    IFACEMETHODIMP Compare(ITextRangeProvider* range, BOOL* equal) override {
        if (!range || !equal) return E_INVALIDARG;
        auto* other = dynamic_cast<TextRangeProvider*>(range);
        *equal = other && other->id_ == id_ && other->start_ == start_ && other->end_ == end_ ? TRUE : FALSE;
        return S_OK;
    }
    IFACEMETHODIMP CompareEndpoints(TextPatternRangeEndpoint endpoint, ITextRangeProvider* range,
                                   TextPatternRangeEndpoint targetEndpoint, int* comparison) override {
        if (!range || !comparison) return E_INVALIDARG;
        auto* other = dynamic_cast<TextRangeProvider*>(range);
        if (!other || other->id_ != id_) return E_INVALIDARG;
        *comparison = Endpoint(endpoint) - other->Endpoint(targetEndpoint);
        return S_OK;
    }
    IFACEMETHODIMP ExpandToEnclosingUnit(TextUnit unit) override {
        const auto text = Text();
        if (unit == TextUnit_Document || unit == TextUnit_Page) { start_ = 0; end_ = static_cast<int>(text.size()); return S_OK; }
        if (unit == TextUnit_Line || unit == TextUnit_Paragraph) {
            start_ = LineStart(text, start_); end_ = LineEnd(text, end_); return S_OK;
        }
        if (unit == TextUnit_Word) {
            start_ = WordStart(text, start_); end_ = WordEnd(text, std::max(start_, end_)); return S_OK;
        }
        start_ = std::clamp(start_, 0, static_cast<int>(text.size()));
        end_ = AdvanceCharacter(text, start_, 1);
        return S_OK;
    }
    IFACEMETHODIMP FindAttribute(TEXTATTRIBUTEID, VARIANT, BOOL, ITextRangeProvider** range) override {
        if (!range) return E_INVALIDARG; *range = nullptr; return S_OK;
    }
    IFACEMETHODIMP FindText(BSTR text, BOOL backward, BOOL ignoreCase, ITextRangeProvider** range) override {
        if (!range) return E_INVALIDARG;
        *range = nullptr;
        if (!text) return E_INVALIDARG;
        std::wstring haystack = Text().substr(static_cast<std::size_t>(start_), static_cast<std::size_t>(end_-start_));
        std::wstring needle(text, SysStringLen(text));
        if (ignoreCase) {
            std::transform(haystack.begin(), haystack.end(), haystack.begin(), towlower);
            std::transform(needle.begin(), needle.end(), needle.begin(), towlower);
        }
        const auto found = backward ? haystack.rfind(needle) : haystack.find(needle);
        if (found != std::wstring::npos) *range = new TextRangeProvider(state_, id_, start_+static_cast<int>(found), start_+static_cast<int>(found+needle.size()));
        return S_OK;
    }
    IFACEMETHODIMP GetAttributeValue(TEXTATTRIBUTEID, VARIANT* value) override {
        if (!value) return E_INVALIDARG;
        VariantInit(value);
        IUnknown* unsupported = nullptr;
        UiaGetReservedNotSupportedValue(&unsupported);
        value->vt = VT_UNKNOWN;
        value->punkVal = unsupported;
        return S_OK;
    }
    IFACEMETHODIMP GetBoundingRectangles(SAFEARRAY** rectangles) override {
        if (!rectangles) return E_INVALIDARG;
        protocol::SemanticNode node;
        if (!Node(node)) return UIA_E_ELEMENTNOTAVAILABLE;
        if (start_ == end_) { *rectangles = SafeArrayCreateVector(VT_R8, 0, 0); return *rectangles ? S_OK : E_OUTOFMEMORY; }
        HWND hwnd = Window(); POINT origin{0, 0}; ClientToScreen(hwnd, &origin);
        const double scale = static_cast<double>(GetDpiForWindow(hwnd))/96.0;
        double values[4]{origin.x+node.x1*scale, origin.y+node.y1*scale, (node.x2-node.x1)*scale, (node.y2-node.y1)*scale};
        *rectangles = SafeArrayCreateVector(VT_R8, 0, 4);
        if (!*rectangles) return E_OUTOFMEMORY;
        for (LONG index = 0; index < 4; ++index) SafeArrayPutElement(*rectangles, &index, &values[index]);
        return S_OK;
    }
    IFACEMETHODIMP GetEnclosingElement(IRawElementProviderSimple** provider) override {
        if (!provider) return E_INVALIDARG;
        *provider = CreateSimpleProvider(state_, id_);
        return *provider ? S_OK : UIA_E_ELEMENTNOTAVAILABLE;
    }
    IFACEMETHODIMP GetText(int maxLength, BSTR* text) override {
        if (!text) return E_INVALIDARG;
        const auto value = Text();
        const int available = std::max(0, end_-start_);
        const int count = maxLength < 0 ? available : std::min(available, maxLength);
        *text = SysAllocStringLen(value.data()+start_, static_cast<UINT>(count));
        return *text || count == 0 ? S_OK : E_OUTOFMEMORY;
    }
    IFACEMETHODIMP Move(TextUnit unit, int count, int* moved) override {
        if (!moved) return E_INVALIDARG;
        const auto result = MovePosition(Text(), start_, unit, count);
        start_ = result.first;
        end_ = start_;
        *moved = result.second;
        return S_OK;
    }
    IFACEMETHODIMP MoveEndpointByUnit(TextPatternRangeEndpoint endpoint, TextUnit unit, int count, int* moved) override {
        if (!moved) return E_INVALIDARG;
        int& position = endpoint == TextPatternRangeEndpoint_Start ? start_ : end_;
        const auto result = MovePosition(Text(), position, unit, count);
        position = result.first;
        if (start_ > end_) { if (endpoint == TextPatternRangeEndpoint_Start) end_ = start_; else start_ = end_; }
        *moved = result.second;
        return S_OK;
    }
    IFACEMETHODIMP MoveEndpointByRange(TextPatternRangeEndpoint endpoint, ITextRangeProvider* range, TextPatternRangeEndpoint target) override {
        if (!range) return E_INVALIDARG;
        auto* other = dynamic_cast<TextRangeProvider*>(range);
        if (!other || other->id_ != id_) return E_INVALIDARG;
        int& position = endpoint == TextPatternRangeEndpoint_Start ? start_ : end_;
        position = other->Endpoint(target);
        if (start_ > end_) { if (endpoint == TextPatternRangeEndpoint_Start) end_ = start_; else start_ = end_; }
        return S_OK;
    }
    IFACEMETHODIMP Select() override { return DispatchSelection(); }
    IFACEMETHODIMP AddToSelection() override { return UIA_E_INVALIDOPERATION; }
    IFACEMETHODIMP RemoveFromSelection() override { return UIA_E_INVALIDOPERATION; }
    IFACEMETHODIMP ScrollIntoView(BOOL) override { return S_OK; }
    IFACEMETHODIMP GetChildren(SAFEARRAY** children) override { if (!children) return E_INVALIDARG; *children = nullptr; return S_OK; }

  private:
    bool Node(protocol::SemanticNode& node) const {
        std::lock_guard<std::mutex> lock(state_->mutex);
        for (const auto& candidate : state_->tree.nodes) if (candidate.id == id_) { node = candidate; return true; }
        return false;
    }
    HWND Window() const { std::lock_guard<std::mutex> lock(state_->mutex); return state_->hwnd; }
    std::wstring Text() const { protocol::SemanticNode node; return Node(node) ? Utf8ToWideUia(node.value) : std::wstring{}; }
    void Normalize() { const int length = static_cast<int>(Text().size()); start_ = std::clamp(start_, 0, length); end_ = std::clamp(end_, start_, length); }
    int Endpoint(TextPatternRangeEndpoint endpoint) const { return endpoint == TextPatternRangeEndpoint_Start ? start_ : end_; }
    static int LineStart(const std::wstring& text, int position) { while (position > 0 && text[static_cast<std::size_t>(position-1)] != L'\n') --position; return position; }
    static int LineEnd(const std::wstring& text, int position) { while (position < static_cast<int>(text.size()) && text[static_cast<std::size_t>(position)] != L'\n') ++position; return position; }
    static int WordStart(const std::wstring& text, int position) { while (position > 0 && !iswspace(text[static_cast<std::size_t>(position-1)])) --position; return position; }
    static int WordEnd(const std::wstring& text, int position) { while (position < static_cast<int>(text.size()) && !iswspace(text[static_cast<std::size_t>(position)])) ++position; return position; }
    static bool HighSurrogate(wchar_t value) { return value >= 0xD800 && value <= 0xDBFF; }
    static bool LowSurrogate(wchar_t value) { return value >= 0xDC00 && value <= 0xDFFF; }
    static int AdvanceCharacter(const std::wstring& text, int position, int direction) {
        if (direction > 0) {
            if (position >= static_cast<int>(text.size())) return static_cast<int>(text.size());
            return position + (HighSurrogate(text[static_cast<std::size_t>(position)]) && position+1 < static_cast<int>(text.size()) ? 2 : 1);
        }
        if (position <= 0) return 0;
        return position - (position >= 2 && LowSurrogate(text[static_cast<std::size_t>(position-1)]) && HighSurrogate(text[static_cast<std::size_t>(position-2)]) ? 2 : 1);
    }
    static std::pair<int, int> MovePosition(const std::wstring& text, int position, TextUnit unit, int count) {
        position = std::clamp(position, 0, static_cast<int>(text.size()));
        if (count == 0) return {position, 0};
        if (unit == TextUnit_Document || unit == TextUnit_Page) {
            const int target = count < 0 ? 0 : static_cast<int>(text.size());
            return {target, target == position ? 0 : (count < 0 ? -1 : 1)};
        }
        int moved = 0;
        const int direction = count < 0 ? -1 : 1;
        for (int step = 0; step < std::abs(count); ++step) {
            int next = position;
            if (unit == TextUnit_Character || unit == TextUnit_Format) next = AdvanceCharacter(text, position, direction);
            else if (unit == TextUnit_Word) {
                if (direction > 0) { next = WordEnd(text, position); while (next < static_cast<int>(text.size()) && iswspace(text[static_cast<std::size_t>(next)])) ++next; }
                else { while (next > 0 && iswspace(text[static_cast<std::size_t>(next-1)])) --next; next = WordStart(text, next); }
            } else if (unit == TextUnit_Line || unit == TextUnit_Paragraph) {
                if (direction > 0) { next = LineEnd(text, position); if (next < static_cast<int>(text.size())) ++next; }
                else { next = LineStart(text, position); if (next > 0) next = LineStart(text, next-1); }
            } else next = AdvanceCharacter(text, position, direction);
            if (next == position) break;
            position = next;
            moved += direction;
        }
        return {position, moved};
    }
    static int Utf16ToRune(const std::wstring& text, int position) {
        int runes = 0;
        for (int index = 0; index < std::min(position, static_cast<int>(text.size())); ++index, ++runes)
            if (text[static_cast<std::size_t>(index)] >= 0xD800 && text[static_cast<std::size_t>(index)] <= 0xDBFF && index+1 < position) ++index;
        return runes;
    }
    HRESULT DispatchSelection() const {
        std::function<void(std::string, std::string, std::string)> handler;
        protocol::SemanticNode node;
        {
            std::lock_guard<std::mutex> lock(state_->mutex);
            for (const auto& candidate : state_->tree.nodes) if (candidate.id == id_) { node = candidate; break; }
            handler = state_->actionHandler;
        }
        if (!handler || node.id.empty()) return UIA_E_ELEMENTNOTAVAILABLE;
        const auto text = Utf8ToWideUia(node.value);
        handler(id_, "set-selection", std::to_string(Utf16ToRune(text, start_))+":"+std::to_string(Utf16ToRune(text, end_)));
        return S_OK;
    }
    std::atomic<ULONG> refs_{1};
    std::shared_ptr<AccessibilityHost::State> state_;
    std::string id_;
    int start_{};
    int end_{};
};

IRawElementProviderSimple* CreateSimpleProvider(const std::shared_ptr<AccessibilityHost::State>& state, const std::string& id) {
    return static_cast<IRawElementProviderSimple*>(new SemanticProvider(state, id));
}

ITextRangeProvider* CreateTextRange(const std::shared_ptr<AccessibilityHost::State>& state, const std::string& id, int start, int end) {
    return new TextRangeProvider(state, id, start, end);
}

} // namespace

AccessibilityHost::AccessibilityHost() : state_(std::make_shared<State>()) {}
AccessibilityHost::~AccessibilityHost() { if (root_) root_->Release(); }

void AccessibilityHost::SetWindow(HWND hwnd) {
    std::lock_guard<std::mutex> lock(state_->mutex);
    state_->hwnd = hwnd;
}

void AccessibilityHost::SetActionHandler(std::function<void(std::string, std::string, std::string)> handler) {
    std::lock_guard<std::mutex> lock(state_->mutex);
    state_->actionHandler = std::move(handler);
}

void AccessibilityHost::Publish(protocol::SemanticTree tree) {
    protocol::SemanticTree previous;
    protocol::SemanticTree current;
    {
        std::lock_guard<std::mutex> lock(state_->mutex);
        previous = state_->tree;
        state_->tree = std::move(tree);
        current = state_->tree;
    }
    if (current.nodes.empty()) return;

    bool structureChanged = previous.nodes.size() != current.nodes.size();
    std::unordered_map<std::string, protocol::SemanticNode> oldNodes;
    oldNodes.reserve(previous.nodes.size());
    for (const auto& node : previous.nodes) oldNodes.emplace(node.id, node);

    auto raiseBool = [this](const std::string& id, PROPERTYID property, bool before, bool after) {
        if (before == after) return;
        auto* provider = static_cast<IRawElementProviderSimple*>(new SemanticProvider(state_, id));
        VARIANT oldValue{}, newValue{};
        oldValue.vt = newValue.vt = VT_BOOL;
        oldValue.boolVal = before ? VARIANT_TRUE : VARIANT_FALSE;
        newValue.boolVal = after ? VARIANT_TRUE : VARIANT_FALSE;
        UiaRaiseAutomationPropertyChangedEvent(provider, property, oldValue, newValue);
        provider->Release();
    };
    auto raiseInt = [this](const std::string& id, PROPERTYID property, int before, int after) {
        if (before == after) return;
        auto* provider = static_cast<IRawElementProviderSimple*>(new SemanticProvider(state_, id));
        VARIANT oldValue{}, newValue{};
        oldValue.vt = newValue.vt = VT_I4;
        oldValue.lVal = before;
        newValue.lVal = after;
        UiaRaiseAutomationPropertyChangedEvent(provider, property, oldValue, newValue);
        provider->Release();
    };
    auto raiseDouble = [this](const std::string& id, PROPERTYID property, double before, double after) {
        if (before == after) return;
        auto* provider = static_cast<IRawElementProviderSimple*>(new SemanticProvider(state_, id));
        VARIANT oldValue{}, newValue{};
        oldValue.vt = newValue.vt = VT_R8;
        oldValue.dblVal = before;
        newValue.dblVal = after;
        UiaRaiseAutomationPropertyChangedEvent(provider, property, oldValue, newValue);
        provider->Release();
    };
    auto raiseString = [this](const std::string& id, PROPERTYID property, const std::string& before, const std::string& after) {
        if (before == after) return;
        auto* provider = static_cast<IRawElementProviderSimple*>(new SemanticProvider(state_, id));
        const auto oldWide = Utf8ToWideUia(before);
        const auto newWide = Utf8ToWideUia(after);
        VARIANT oldValue{}, newValue{};
        oldValue.vt = newValue.vt = VT_BSTR;
        oldValue.bstrVal = SysAllocStringLen(oldWide.data(), static_cast<UINT>(oldWide.size()));
        newValue.bstrVal = SysAllocStringLen(newWide.data(), static_cast<UINT>(newWide.size()));
        UiaRaiseAutomationPropertyChangedEvent(provider, property, oldValue, newValue);
        VariantClear(&oldValue);
        VariantClear(&newValue);
        provider->Release();
    };

    for (const auto& node : current.nodes) {
        const auto found = oldNodes.find(node.id);
        if (found == oldNodes.end()) {
            structureChanged = true;
            continue;
        }
        const auto& old = found->second;
        if (old.parent != node.parent) structureChanged = true;
        raiseString(node.id, UIA_NamePropertyId, old.name, node.name);
        raiseString(node.id, UIA_ValueValuePropertyId, old.value, node.value);
		if (old.hasText && node.hasText && old.value != node.value) {
			auto* provider = static_cast<IRawElementProviderSimple*>(new SemanticProvider(state_, node.id));
			UiaRaiseAutomationEvent(provider, UIA_Text_TextChangedEventId);
			provider->Release();
		}
		if (old.hasText && node.hasText && (old.selectionStart != node.selectionStart || old.selectionEnd != node.selectionEnd)) {
			auto* provider = static_cast<IRawElementProviderSimple*>(new SemanticProvider(state_, node.id));
			UiaRaiseAutomationEvent(provider, UIA_Text_TextSelectionChangedEventId);
			provider->Release();
		}
        raiseBool(node.id, UIA_IsEnabledPropertyId, (old.state & 1u) == 0, (node.state & 1u) == 0);
        raiseBool(node.id, UIA_IsOffscreenPropertyId, (old.state & (1u << 9)) != 0, (node.state & (1u << 9)) != 0);
        raiseBool(node.id, UIA_ValueIsReadOnlyPropertyId, (old.state & (1u << 5)) != 0, (node.state & (1u << 5)) != 0);
        const bool wasFocused = (old.state & (1u << 1)) != 0;
        const bool isFocused = (node.state & (1u << 1)) != 0;
        raiseBool(node.id, UIA_HasKeyboardFocusPropertyId, wasFocused, isFocused);
        raiseBool(node.id, UIA_SelectionItemIsSelectedPropertyId, (old.state & (1u << 2)) != 0, (node.state & (1u << 2)) != 0);
        raiseInt(node.id, UIA_ToggleToggleStatePropertyId, (old.state & (1u << 3)) != 0 ? ToggleState_On : ToggleState_Off,
                 (node.state & (1u << 3)) != 0 ? ToggleState_On : ToggleState_Off);
        raiseInt(node.id, UIA_ExpandCollapseExpandCollapseStatePropertyId,
                 (old.state & (1u << 4)) != 0 ? ExpandCollapseState_Expanded : ExpandCollapseState_Collapsed,
                 (node.state & (1u << 4)) != 0 ? ExpandCollapseState_Expanded : ExpandCollapseState_Collapsed);
        if (!wasFocused && isFocused) {
            auto* provider = static_cast<IRawElementProviderSimple*>(new SemanticProvider(state_, node.id));
            UiaRaiseAutomationEvent(provider, UIA_AutomationFocusChangedEventId);
            provider->Release();
        }
        if (old.hasGrid && node.hasGrid) {
            raiseInt(node.id, UIA_GridRowCountPropertyId, old.gridRows, node.gridRows);
            raiseInt(node.id, UIA_GridColumnCountPropertyId, old.gridColumns, node.gridColumns);
        }
        if (old.hasScroll && node.hasScroll) {
            raiseBool(node.id, UIA_ScrollHorizontallyScrollablePropertyId, old.hScrollable, node.hScrollable);
            raiseBool(node.id, UIA_ScrollVerticallyScrollablePropertyId, old.vScrollable, node.vScrollable);
            raiseDouble(node.id, UIA_ScrollHorizontalScrollPercentPropertyId, old.hScrollPercent, node.hScrollPercent);
            raiseDouble(node.id, UIA_ScrollVerticalScrollPercentPropertyId, old.vScrollPercent, node.vScrollPercent);
            raiseDouble(node.id, UIA_ScrollHorizontalViewSizePropertyId, old.hViewSize, node.hViewSize);
            raiseDouble(node.id, UIA_ScrollVerticalViewSizePropertyId, old.vViewSize, node.vViewSize);
        }
    }
    if (structureChanged) {
        auto* provider = static_cast<IRawElementProviderSimple*>(new SemanticProvider(state_, current.nodes.front().id));
        UiaRaiseStructureChangedEvent(provider, StructureChangeType_ChildrenInvalidated, nullptr, 0);
        provider->Release();
    }
}

LRESULT AccessibilityHost::HandleGetObject(WPARAM wParam, LPARAM lParam) {
    if (static_cast<long>(lParam) != UiaRootObjectId) return 0;
    std::string rootId;
    {
        std::lock_guard<std::mutex> lock(state_->mutex);
        if (!state_->tree.nodes.empty()) rootId = state_->tree.nodes.front().id;
    }
    if (rootId.empty()) return 0;
    if (root_) root_->Release();
    root_ = static_cast<IRawElementProviderSimple*>(new SemanticProvider(state_, rootId));
    return UiaReturnRawElementProvider(state_->hwnd, wParam, lParam, root_);
}

} // namespace poem
