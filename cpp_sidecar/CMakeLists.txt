cmake_minimum_required(VERSION 3.20)
project(poem_windows_host LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

add_executable(poem_windows_host WIN32
  src/main.cpp
  src/native_app.cpp
  ../shared/poem/protocol.cpp
  src/audio.cpp
  src/renderer_d3d11.cpp
  src/accessibility.cpp
)

# ../shared holds platform-neutral C++ used by every host (see shared/poem).
target_include_directories(poem_windows_host PRIVATE src ${CMAKE_CURRENT_SOURCE_DIR}/../shared)
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

  # Shared map maths: no GPU, window or platform SDK, so it runs anywhere the
  # host builds and pins the projection/lighting every presenter shares.
  add_executable(poem_map_view_test tests/map_view_test.cpp)
  target_include_directories(poem_map_view_test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/../shared)
  add_test(NAME poem_map_view COMMAND poem_map_view_test)

  if(WIN32)
    add_executable(poem_map_shader_test tests/map_shader_test.cpp)
    target_include_directories(poem_map_shader_test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/../shared)
    target_compile_definitions(poem_map_shader_test PRIVATE NOMINMAX WIN32_LEAN_AND_MEAN)
    target_link_libraries(poem_map_shader_test PRIVATE d3dcompiler)
    add_test(NAME poem_map_shader COMMAND poem_map_shader_test)
  endif()
endif()
