// Compiles the canonical shared map shader with the Windows shader compiler.
// The portable maths are otherwise only exercised as C++, which cannot catch
// HLSL reserved words or intrinsic-signature errors before a native window
// attempts to create its D3D11 pipeline.

#include "poem/map_shader.h"

#include <d3dcompiler.h>
#include <wrl/client.h>

#include <cstdio>
#include <cstring>
#include <string>

int main() {
    const std::string source = std::string(poem::mapshader::kHlslPrelude) +
        poem::mapshader::kBody + R"(
struct MarkerInput {
    float2 corner : TEXCOORD0;
    float symbol : TEXCOORD1;
};
float4 main(MarkerInput input) : SV_TARGET {
    return float4(1.0, 1.0, 1.0, MapMarkerAlpha(input.corner, input.symbol));
}
)";

    Microsoft::WRL::ComPtr<ID3DBlob> shader;
    Microsoft::WRL::ComPtr<ID3DBlob> errors;
    const HRESULT result = D3DCompile(source.data(), source.size(), "poem_map_shader", nullptr, nullptr,
                                      "main", "ps_5_0", D3DCOMPILE_ENABLE_STRICTNESS, 0, &shader, &errors);
    if (FAILED(result)) {
        if (errors && errors->GetBufferPointer()) {
            std::fwrite(errors->GetBufferPointer(), 1, errors->GetBufferSize(), stderr);
        }
        return 1;
    }
    return shader && shader->GetBufferSize() > 0 ? 0 : 1;
}
