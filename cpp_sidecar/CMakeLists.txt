cmake_minimum_required(VERSION 3.20)
project(poem_windows_host LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

add_executable(poem_windows_host WIN32
  src/main.cpp
  src/native_app.cpp
  src/protocol.cpp
  src/audio.cpp
  src/renderer_d3d11.cpp
  src/accessibility.cpp
)

target_include_directories(poem_windows_host PRIVATE src)
target_compile_definitions(poem_windows_host PRIVATE UNICODE _UNICODE NOMINMAX WIN32_LEAN_AND_MEAN)
target_link_libraries(poem_windows_host PRIVATE d3d11 dxgi d3dcompiler winmm user32 gdi32 shcore shell32 ole32 oleaut32 uuid imm32 uiautomationcore)

include(CTest)
if(BUILD_TESTING)
  add_executable(poem_windows_host_contract_test
    tests/native_app_test.cpp
    src/native_app.cpp
  )
  target_include_directories(poem_windows_host_contract_test PRIVATE src)
  target_compile_definitions(poem_windows_host_contract_test PRIVATE UNICODE _UNICODE NOMINMAX WIN32_LEAN_AND_MEAN)
  add_test(NAME poem_windows_host_contract COMMAND poem_windows_host_contract_test)
endif()
