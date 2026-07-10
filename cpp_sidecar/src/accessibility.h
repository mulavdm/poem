#pragma once

#include "protocol.h"

#include <windows.h>
#include <unknwn.h>
#include <oaidl.h>
#include <oleauto.h>
#include <UIAutomation.h>

#include <memory>
#include <mutex>
#include <string>
#include <functional>

namespace poem {

class AccessibilityHost {
  public:
    struct State;
    AccessibilityHost();
    ~AccessibilityHost();
    AccessibilityHost(const AccessibilityHost&) = delete;
    AccessibilityHost& operator=(const AccessibilityHost&) = delete;

    void SetWindow(HWND hwnd);
    void SetActionHandler(std::function<void(std::string, std::string, std::string)> handler);
    void Publish(protocol::SemanticTree tree);
    LRESULT HandleGetObject(WPARAM wParam, LPARAM lParam);

  private:
    std::shared_ptr<State> state_;
    IRawElementProviderSimple* root_ = nullptr;
};

} // namespace poem
