cmake_minimum_required(VERSION 3.20)
project(poem_cpp_sidecar LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

add_executable(poem_cpp_sidecar WIN32
  src/main.cpp
  src/ipc.cpp
  src/protocol.cpp
  src/audio.cpp
  src/renderer_d3d11.cpp
)

target_include_directories(poem_cpp_sidecar PRIVATE src)
target_compile_definitions(poem_cpp_sidecar PRIVATE UNICODE _UNICODE NOMINMAX WIN32_LEAN_AND_MEAN)
target_link_libraries(poem_cpp_sidecar PRIVATE d3d11 dxgi d3dcompiler winmm user32 gdi32 shcore shell32 ole32 uuid)
