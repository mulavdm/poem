#include "renderer_d3d11.h"

#include "poem/map_view.h"

#include <d3dcompiler.h>
#include <dxgi.h>
#include <algorithm>
#include <cmath>
#include <cstring>
#include <cstdlib>
#include <unordered_set>
#include <vector>

#ifdef DrawText
#undef DrawText
#endif

namespace poem {

namespace {

struct ConstantBuffer {
    float screenWidth;
    float screenHeight;
    float pad[2];
};

// MapViewConstants mirrors shared/poem/map_view.h View for the GPU map path.
// The camera centre is split into hi+lo floats (relative-to-eye) because world
// units near zoom 15 exceed single-float precision when differenced in-shader.
struct MapViewConstants {
    float centerHiX, centerHiY, centerLoX, centerLoY;
    float worldPixels, cosBearing, sinBearing, metersToPixels;
    float pitchCos, pitchSin, cameraDistance, screenWidth;
    float screenHeight, rectCenterX, rectCenterY, ambient;
    float sunX, sunY, sunZ, fogDensity;
    float fogR, fogG, fogB, fogPitch;
    // From shared/poem/map_view.h so the HLSL never hard-codes them.
    float fogReferenceMeters, fogScaleHeightMeters, pad0, pad1;
};

struct MapDrawConstants {
    float opacity;
    float shaded; // 1 = light against the sun (extrusions), 0 = styled colour
    float pad[2];
};

// The GPU map shader projects raw cartography vertices (world x/y in mercator
// units, z in metres) exactly as shared/poem/map_view.h does on the CPU, and
// shades per-face from screen-space derivatives of the local position — the
// derivative of a linearly interpolated varying is constant per triangle, so
// cross(ddx, ddy) recovers the true face plane without per-vertex normals.
const char* kMapShader = R"(
cbuffer MapView : register(b0) {
    float4 centerHiLo;    // hiX hiY loX loY
    float4 worldBearing;  // worldPixels cosB sinB metersToPixels
    float4 pitchScreen;   // pitchCos pitchSin cameraDistance screenW
    float4 screenRect;    // screenH rectCX rectCY ambient
    float4 sunFog;        // sunX sunY sunZ fogDensity
    float4 fogColorPitch; // fogR fogG fogB pitchSin
    float4 fogParams;     // referenceMeters scaleHeight - -
};
cbuffer MapDraw : register(b1) {
    float4 drawParams;    // opacity shaded - -
};

struct VSIn {
    float3 pos : POSITION;
    float4 color : COLOR0;
};
struct VSOut {
    float4 pos : SV_POSITION;
    float4 color : COLOR0;
    float3 local : TEXCOORD0;
};

VSOut vsmain(VSIn input) {
    float dx = (input.pos.x - centerHiLo.x) - centerHiLo.z;
    if (dx > 0.5) dx -= 1.0;
    if (dx < -0.5) dx += 1.0;
    float dy = (input.pos.y - centerHiLo.y) - centerHiLo.w;
    float localX = (dx * worldBearing.y - dy * worldBearing.z) * worldBearing.x;
    float localY = (dx * worldBearing.z + dy * worldBearing.y) * worldBearing.x;
    float localZ = input.pos.z * worldBearing.w;
    float projY = localY * pitchScreen.x - localZ * pitchScreen.y;
    float depth = localY * pitchScreen.y + localZ * pitchScreen.x;
    float persp = pitchScreen.z / max(pitchScreen.z * 0.05, pitchScreen.z - depth);
    float sx = screenRect.y + localX * persp;
    float sy = screenRect.z + projY * persp;
    VSOut output;
    // Depth spans [-19*cameraDistance, +cameraDistance): the perspective divide
    // floors at cameraDistance*0.05, i.e. depth = -19*cd at the far clamp.
    // Normalizing over that exact range keeps 24-bit precision (~0.001 px)
    // instead of saturating the pitched scene into z-fighting ties.
    output.pos = float4(sx / pitchScreen.w * 2.0 - 1.0, 1.0 - sy / screenRect.x * 2.0,
                        saturate((depth + 19.0 * pitchScreen.z) / (20.0 * pitchScreen.z)), 1.0);
    output.color = input.color;
    output.local = float3(localX, localY, localZ);
    return output;
}

float4 psmain(VSOut input) : SV_TARGET {
    float3 color = input.color.rgb;
    if (drawParams.y > 0.5) {
        float3 normal = cross(ddx(input.local), ddy(input.local));
        float len = length(normal);
        if (len > 1e-6) {
            normal /= len;
            // abs() lights a face and its back identically: adjacent buildings
            // share coplanar walls that are drawn twice with opposite winding,
            // and symmetric shading makes that unavoidable z-tie invisible.
            float lambert = abs(dot(normal, sunFog.xyz));
            color *= screenRect.w + (1.0 - screenRect.w) * lambert;
        }
    }
    if (sunFog.w > 0.0 && fogColorPitch.w > 0.0) {
        float meters = length(input.local.xy) / worldBearing.w;
        float elevM = input.local.z / worldBearing.w;
        float altitude = exp(-max(elevM, 0.0) / fogParams.y);
        float fog = 1.0 - exp(-sunFog.w * (meters / fogParams.x) * altitude * fogColorPitch.w);
        color = lerp(color, fogColorPitch.rgb, saturate(fog));
    }
    return float4(color, input.color.a * drawParams.x);
}
)";

std::string MapHashKey(const std::array<std::uint8_t, 32>& hash) {
    return std::string(reinterpret_cast<const char*>(hash.data()), hash.size());
}

std::vector<std::uint32_t> DecodeUtf8(const std::string& text) {
    std::vector<std::uint32_t> codepoints;
    codepoints.reserve(text.size());
    for (std::size_t index = 0; index < text.size();) {
        const auto first = static_cast<std::uint8_t>(text[index]);
        std::uint32_t codepoint = 0;
        std::size_t length = 0;
        std::uint32_t minimum = 0;
        if (first < 0x80) {
            codepoint = first;
            length = 1;
        } else if ((first & 0xE0) == 0xC0) {
            codepoint = first & 0x1F;
            length = 2;
            minimum = 0x80;
        } else if ((first & 0xF0) == 0xE0) {
            codepoint = first & 0x0F;
            length = 3;
            minimum = 0x800;
        } else if ((first & 0xF8) == 0xF0) {
            codepoint = first & 0x07;
            length = 4;
            minimum = 0x10000;
        }
        bool valid = length != 0 && index + length <= text.size();
        for (std::size_t offset = 1; valid && offset < length; ++offset) {
            const auto next = static_cast<std::uint8_t>(text[index + offset]);
            if ((next & 0xC0) != 0x80) {
                valid = false;
                break;
            }
            codepoint = (codepoint << 6) | (next & 0x3F);
        }
        if (!valid || codepoint < minimum || codepoint > 0x10FFFF || (codepoint >= 0xD800 && codepoint <= 0xDFFF)) {
            codepoints.push_back(0xFFFD);
            ++index;
            continue;
        }
        codepoints.push_back(codepoint);
        index += length;
    }
    return codepoints;
}

constexpr const char* kVertexShader = R"(
cbuffer Globals : register(b0) {
    float screenWidth;
    float screenHeight;
    float2 pad;
};
struct VSIn {
    float2 pos : POSITION;
    float2 uv : TEXCOORD0;
    float4 color : COLOR0;
    float4 rectParams : TEXCOORD1;
    float4 extras0 : TEXCOORD2;
    float4 extras1 : TEXCOORD3;
};
struct VSOut {
    float4 pos : SV_POSITION;
    float2 fragPos : TEXCOORD0;
    float2 uv : TEXCOORD1;
    float4 color : COLOR0;
    float4 rectParams : TEXCOORD2;
    float4 extras0 : TEXCOORD3;
    float4 extras1 : TEXCOORD4;
};
VSOut main(VSIn input) {
    VSOut o;
    float x = (input.pos.x / screenWidth) * 2.0 - 1.0;
    float y = 1.0 - (input.pos.y / screenHeight) * 2.0;
    o.pos = float4(x, y, 0.0, 1.0);
    o.fragPos = input.pos;
    o.uv = input.uv;
    o.color = input.color;
    o.rectParams = input.rectParams;
    o.extras0 = input.extras0;
    o.extras1 = input.extras1;
    return o;
}
)";

constexpr const char* kPixelShader = R"(
Texture2D atlasTex : register(t0);
StructuredBuffer<uint> mapData : register(t1);
Texture2D imageTex : register(t2);
SamplerState atlasSampler : register(s0);

struct PSIn {
    float4 pos : SV_POSITION;
    float2 fragPos : TEXCOORD0;
    float2 uv : TEXCOORD1;
    float4 color : COLOR0;
    float4 rectParams : TEXCOORD2; // x, y, w, h
    float4 extras0 : TEXCOORD3;    // drawType, glow, isGlass, radius
    float4 extras1 : TEXCOORD4;    // shadowOffsetX, shadowOffsetY, shadowSoftness, padding
};

float sdRoundedRect(float2 p, float2 b, float r) {
    float2 q = abs(p) - b + float2(r, r);
    return length(max(q, float2(0.0, 0.0))) + min(max(q.x, q.y), 0.0) - r;
}

uint getGridVal(int x, int y) {
    if (x < 0 || x >= 64 || y < 0 || y >= 64) {
        return 1u;
    }
    return mapData[y * 64 + x];
}

float4 sampleText(PSIn input) {
    float4 sample = atlasTex.Sample(atlasSampler, input.uv);
    return float4(input.color.rgb, input.color.a * sample.a);
}

float4 sampleImage(PSIn input) {
    return imageTex.Sample(atlasSampler, input.uv);
}

float4 shadeRaycaster(PSIn input) {
    const float posX = input.extras1.x;
    const float posY = input.extras1.y;
    const float angle = input.extras1.z;

    const float colRatio = (input.fragPos.x - input.rectParams.x) / max(input.rectParams.z, 1.0);
    const float cameraX = 2.0 * colRatio - 1.0;
    const float dirX = cos(angle);
    const float dirY = sin(angle);
    const float planeX = -dirY * 0.66;
    const float planeY = dirX * 0.66;
    const float rayDirX = dirX + planeX * cameraX;
    const float rayDirY = dirY + planeY * cameraX;

    int mapX = (int)floor(posX);
    int mapY = (int)floor(posY);

    float deltaDistX = 1e30;
    if (abs(rayDirX) > 0.0001) {
        deltaDistX = abs(1.0 / rayDirX);
    }
    float deltaDistY = 1e30;
    if (abs(rayDirY) > 0.0001) {
        deltaDistY = abs(1.0 / rayDirY);
    }

    int stepX = 0;
    int stepY = 0;
    float sideDistX = 0.0;
    float sideDistY = 0.0;

    if (rayDirX < 0.0) {
        stepX = -1;
        sideDistX = (posX - (float)mapX) * deltaDistX;
    } else {
        stepX = 1;
        sideDistX = ((float)mapX + 1.0 - posX) * deltaDistX;
    }
    if (rayDirY < 0.0) {
        stepY = -1;
        sideDistY = (posY - (float)mapY) * deltaDistY;
    } else {
        stepY = 1;
        sideDistY = ((float)mapY + 1.0 - posY) * deltaDistY;
    }

    int side = 0;
    [loop]
    for (int i = 0; i < 40; ++i) {
        if (sideDistX < sideDistY) {
            sideDistX += deltaDistX;
            mapX += stepX;
            side = 0;
        } else {
            sideDistY += deltaDistY;
            mapY += stepY;
            side = 1;
        }
        if (getGridVal(mapX, mapY) == 1u) {
            break;
        }
    }

    float perpWallDist = 0.0;
    if (side == 0) {
        perpWallDist = ((float)mapX - posX + (1.0 - (float)stepX) * 0.5) / rayDirX;
    } else {
        perpWallDist = ((float)mapY - posY + (1.0 - (float)stepY) * 0.5) / rayDirY;
    }
    perpWallDist = max(perpWallDist, 0.05);

    const float wallH = input.rectParams.w * 0.78 / perpWallDist;
    const float yCenter = input.rectParams.y + input.rectParams.w * 0.5 + 30.0;
    const float drawStart = yCenter - wallH * 0.5;
    const float drawEnd = yCenter + wallH * 0.5;

    float wallX = 0.0;
    if (side == 0) {
        wallX = posY + perpWallDist * rayDirY;
    } else {
        wallX = posX + perpWallDist * rayDirX;
    }
    wallX = frac(wallX);

    if (input.fragPos.y >= drawStart && input.fragPos.y <= drawEnd) {
        float3 accent = max(input.color.rgb, float3(0.35, 0.18, 0.08));
        float3 wallColor = lerp(float3(0.95, 0.42, 0.18), accent, 0.35);
        if (side == 1) {
            wallColor *= 0.72;
        }
        const float depthShade = saturate(1.5 / (1.0 + perpWallDist * 0.08));
        wallColor *= depthShade;

        const float stripe = step(0.92, frac(((float)mapX + (float)mapY + input.fragPos.y * 0.015) * 0.5));
        const float rib = 1.0 - smoothstep(0.0, 0.08, abs(wallX - 0.5));
        const float panel = smoothstep(0.18, 0.0, abs(frac(wallX * 2.0) - 0.5));
        const float viewportV = saturate((input.fragPos.y - drawStart) / max(drawEnd - drawStart, 1.0));
        const float baseStrip = smoothstep(0.82, 1.0, viewportV);
        const float topGloss = smoothstep(0.22, 0.0, viewportV) * smoothstep(0.20, 0.0, abs(wallX - 0.18));
        wallColor = lerp(wallColor, wallColor + float3(0.16, 0.10, 0.03), stripe * 0.35);
        wallColor = lerp(wallColor, wallColor + float3(0.12, 0.09, 0.06), rib * 0.32);
        wallColor = lerp(wallColor, wallColor + float3(0.08, 0.05, 0.04), panel * 0.22);
        wallColor = lerp(wallColor, float3(0.34, 0.16, 0.10), baseStrip * 0.55);
        wallColor = lerp(wallColor, float3(1.0, 0.82, 0.52), topGloss * 0.45);

        if (input.fragPos.y < drawStart + 2.0 || input.fragPos.y > drawEnd - 2.0) {
            return float4(float3(1.0, 0.58, 0.18) * depthShade, 1.0);
        }
        return float4(wallColor, 1.0);
    }

    if (input.fragPos.y < drawStart) {
        const float dist = (input.rectParams.w * 0.39) / max(yCenter - input.fragPos.y, 1.0);
        const float x3d = posX + dirX * dist + planeX * cameraX * dist;
        const float y3d = posY + dirY * dist + planeY * cameraX * dist;
        const float gridX = frac(x3d);
        const float gridY = frac(y3d);
        const float arch = 1.0 - smoothstep(0.2, 0.95, abs(colRatio));
        const float ribbing = smoothstep(0.06, 0.0, abs(frac(x3d * 0.7 + y3d * 0.7) - 0.5));
        if (gridX < 0.035 || gridY < 0.035) {
            return float4(float3(0.35, 0.12, 0.34) * (1.0 / (1.0 + dist * 0.1)), 1.0);
        }
        const float3 ceilBase = lerp(float3(0.12, 0.08, 0.14), float3(0.24, 0.12, 0.20), arch * 0.55);
        const float3 ceilCol = lerp(ceilBase, ceilBase + float3(0.10, 0.08, 0.05), ribbing * 0.22);
        return float4(ceilCol, 1.0);
    }

    const float dist = (input.rectParams.w * 0.39) / max(input.fragPos.y - yCenter, 1.0);
    const float x3d = posX + dirX * dist + planeX * cameraX * dist;
    const float y3d = posY + dirY * dist + planeY * cameraX * dist;
    const float gridX = frac(x3d);
    const float gridY = frac(y3d);
    const float tile = step(0.5, frac((floor(x3d) + floor(y3d)) * 0.5));
    float3 floorCol = lerp(float3(0.22, 0.12, 0.20), float3(0.30, 0.18, 0.16), tile * 0.35);
    const float runLane = smoothstep(0.16, 0.0, abs(frac(x3d * 0.5) - 0.5));
    const float dustMotes = smoothstep(0.94, 1.0, frac(sin(dot(float2(floor(x3d * 3.0), floor(y3d * 3.0)), float2(12.9898, 78.233))) * 43758.5453));
    if (gridX < 0.035 || gridY < 0.035) {
        return float4(float3(0.75, 0.32, 0.12) * (1.0 / (1.0 + dist * 0.1)), 1.0);
    }
    floorCol = lerp(floorCol, floorCol + float3(0.08, 0.05, 0.02), runLane * 0.18);
    floorCol = lerp(floorCol, float3(0.75, 0.66, 0.42), dustMotes * 0.08);
    return float4(floorCol, 1.0);
}

float4 shadeBillboard(PSIn input, float radius) {
    const float posX = input.uv.x;
    const float posY = input.uv.y;
    const float angle = input.extras1.z;

    const float colRatio = (input.fragPos.x - input.rectParams.x) / max(input.rectParams.z, 1.0);
    const float cameraX = 2.0 * colRatio - 1.0;
    const float dirX = cos(angle);
    const float dirY = sin(angle);
    const float planeX = -dirY * 0.66;
    const float planeY = dirX * 0.66;
    const float rayDirX = dirX + planeX * cameraX;
    const float rayDirY = dirY + planeY * cameraX;

    int mapX = (int)floor(posX);
    int mapY = (int)floor(posY);
    float deltaDistX = (abs(rayDirX) > 0.0001) ? abs(1.0 / rayDirX) : 1e30;
    float deltaDistY = (abs(rayDirY) > 0.0001) ? abs(1.0 / rayDirY) : 1e30;
    int stepX = 0;
    int stepY = 0;
    float sideDistX = 0.0;
    float sideDistY = 0.0;

    if (rayDirX < 0.0) {
        stepX = -1;
        sideDistX = (posX - (float)mapX) * deltaDistX;
    } else {
        stepX = 1;
        sideDistX = ((float)mapX + 1.0 - posX) * deltaDistX;
    }
    if (rayDirY < 0.0) {
        stepY = -1;
        sideDistY = (posY - (float)mapY) * deltaDistY;
    } else {
        stepY = 1;
        sideDistY = ((float)mapY + 1.0 - posY) * deltaDistY;
    }

    int side = 0;
    [loop]
    for (int i = 0; i < 40; ++i) {
        if (sideDistX < sideDistY) {
            sideDistX += deltaDistX;
            mapX += stepX;
            side = 0;
        } else {
            sideDistY += deltaDistY;
            mapY += stepY;
            side = 1;
        }
        if (getGridVal(mapX, mapY) == 1u) {
            break;
        }
    }

    float perpWallDist = 0.0;
    if (side == 0) {
        perpWallDist = ((float)mapX - posX + (1.0 - (float)stepX) * 0.5) / rayDirX;
    } else {
        perpWallDist = ((float)mapY - posY + (1.0 - (float)stepY) * 0.5) / rayDirY;
    }
    perpWallDist = max(perpWallDist, 0.05);

    if (input.extras1.w > perpWallDist) {
        discard;
    }

    const float2 center = input.rectParams.xy + input.rectParams.zw * 0.5;
    const float2 p = (input.fragPos - center) / max(input.rectParams.zw * 0.5, float2(1.0, 1.0));

    if (radius == -998.0) {
        const float d = length(p);
        if (d > 0.8) discard;
        const float3 normal = float3(p.xy, sqrt(saturate(1.0 - dot(p.xy, p.xy))));
        const float3 lightDir = normalize(float3(0.5, 0.5, 1.0));
        const float diffuse = max(dot(normal, lightDir), 0.0);
        return float4(float3(1.0, 0.8, 0.0) * (diffuse + 0.3), 1.0);
    }
    if (radius == -997.0) {
        const float d = length(p);
        if (d > 0.9) discard;
        float3 body = input.color.rgb;
        if (abs(d - 0.7) < 0.05) {
            body *= 0.5;
        }
        const float3 normal = float3(p.xy, sqrt(saturate(1.0 - dot(p.xy, p.xy))));
        const float3 lightDir = normalize(float3(0.3, 0.7, 0.9));
        const float diffuse = max(dot(normal, lightDir), 0.0);
        const float spec = pow(max(dot(reflect(-lightDir, normal), float3(0.0, 0.0, 1.0)), 0.0), 8.0);
        float3 finalColor = body * (diffuse + 0.2) + spec * 0.4;
        if (length(p - float2(0.0, -0.3)) < 0.2) {
            finalColor = float3(1.0, 0.0, 0.0);
        }
        return float4(finalColor, 1.0);
    }
    if (radius == -996.0) {
        if (abs(p.x) > 0.62 || p.y < -0.78 || p.y > 0.82) discard;
        float3 jar = float3(1.0, 0.72, 0.12);
        if (p.y < -0.48) jar = float3(0.95, 0.20, 0.32);
        const float glassEdge = smoothstep(0.46, 0.62, abs(p.x));
        const float shine = smoothstep(0.18, 0.0, abs(p.x + 0.25)) * smoothstep(0.62, -0.2, p.y);
        jar = lerp(jar, float3(1.0, 0.95, 0.55), shine * 0.55);
        jar = lerp(jar, float3(1.0, 0.45, 0.08), glassEdge * 0.35);
        return float4(jar, 0.96);
    }
    if (radius == -995.0) {
        const float chevron = abs(abs(p.x) - (0.22 + p.y * 0.42));
        if (p.y < -0.55 || p.y > 0.55 || chevron > 0.16) discard;
        return float4(lerp(input.color.rgb, float3(1.0, 0.95, 0.2), 0.35), 0.85);
    }
    if (radius == -994.0) {
        const float d = length(p);
        if (d < 0.45 || d > 0.95) discard;
        const float band = smoothstep(0.02, 0.0, abs(d - 0.70));
        return float4(lerp(float3(0.96, 0.48, 0.20), float3(1.0, 0.84, 0.60), band * 0.65), 0.92);
    }
    if (radius == -993.0) {
        const float mound = (p.x * p.x) / 0.90 + ((p.y + 0.18) * (p.y + 0.18)) / 0.46;
        if (mound > 1.0 || p.y > 0.55) discard;
        const float speck = smoothstep(0.94, 1.0, frac(sin(dot(float2(floor((p.x + 1.0) * 5.0), floor((p.y + 1.0) * 5.0)), float2(91.77, 21.13))) * 12511.331));
        float3 nest = float3(0.83, 0.66, 0.34);
        nest = lerp(nest, float3(0.96, 0.84, 0.52), speck * 0.25);
        return float4(nest, 0.95);
    }
    if (radius == -992.0) {
        const float rim = abs(length(float2(p.x, p.y + 0.12)) - 0.62);
        if (p.y > 0.46 || abs(p.x) > 0.74 || ((p.y + 0.12) > 0.0 && rim > 0.12)) discard;
        float3 bowl = float3(0.20, 0.78, 0.96);
        if (p.y < -0.05) bowl = float3(1.0, 0.82, 0.18);
        return float4(bowl, 0.96);
    }

    const float ring = abs(length(p) - 0.76);
    const float spoke = smoothstep(0.05, 0.0, abs(p.x)) + smoothstep(0.05, 0.0, abs(p.y));
    if (ring > 0.10 && spoke < 0.9) discard;
    return float4(lerp(float3(0.92, 0.26, 0.30), float3(1.0, 0.82, 0.22), spoke * 0.45), 0.90);
}

float4 shadeShape(PSIn input, float radius, float glow, float isGlass, float2 shadowOffset, float shadowSoftness) {
    const float2 size = input.rectParams.zw;
    const float2 center = input.rectParams.xy + size * 0.5;
    const float2 p = input.fragPos - center;
    const float2 b = size * 0.5;
    const float clampedRadius = max(radius, 0.0);
    const float d = sdRoundedRect(p, b, clampedRadius);

    float shadowAlpha = 0.0;
    if (shadowSoftness > 0.0) {
        const float2 shadowP = input.fragPos - (center + shadowOffset);
        const float shadowD = sdRoundedRect(shadowP, b, clampedRadius);
        shadowAlpha = (1.0 - smoothstep(-shadowSoftness, shadowSoftness, shadowD)) * 0.6;
    }

    const float shapeAlpha = 1.0 - smoothstep(-1.0, 1.0, d);
    const float glowAlpha = exp(-max(0.0, d) * max(1.0, 10.0 - glow)) * (glow / 10.0);
    const float finalAlpha = max(shapeAlpha, glowAlpha);

    const float3 baseColor = input.color.rgb;
    const float4 shadowCol = float4(0.0, 0.0, 0.0, shadowAlpha * 0.8);
    float4 shapeCol = float4(baseColor, input.color.a * finalAlpha);
    if (isGlass > 0.5) {
        const float3 glassTint = lerp(float3(0.12, 0.15, 0.18), baseColor, 0.55);
        shapeCol = float4(glassTint, input.color.a * max(shapeAlpha, 0.35));
    }
    return lerp(shadowCol, shapeCol, shapeAlpha);
}

float4 main(PSIn input) : SV_TARGET {
    const float drawType = input.extras0.x;
    const float glow = input.extras0.y;
    const float isGlass = input.extras0.z;
    const float radius = input.extras0.w;
    const float2 shadowOffset = input.extras1.xy;
    const float shadowSoftness = input.extras1.z;

    if (drawType > 1.5) {
        return sampleImage(input);
    }
    if (drawType > 0.5) {
        return sampleText(input);
    }
    if (radius == -999.0) {
        return shadeRaycaster(input);
    }
    if (radius <= -991.0 && radius >= -998.0) {
        return shadeBillboard(input, radius);
    }
    return shadeShape(input, radius, glow, isGlass, shadowOffset, shadowSoftness);
}
)";

Microsoft::WRL::ComPtr<ID3DBlob> CompileShader(const char* source, const char* entry, const char* target) {
    Microsoft::WRL::ComPtr<ID3DBlob> blob;
    Microsoft::WRL::ComPtr<ID3DBlob> errors;
    const UINT flags = D3DCOMPILE_ENABLE_STRICTNESS;
    const HRESULT hr = D3DCompile(source, std::strlen(source), nullptr, nullptr, nullptr, entry, target, flags, 0, &blob, &errors);
    if (FAILED(hr)) {
        return nullptr;
    }
    return blob;
}

} // namespace

bool RendererD3D11::Initialize(HWND hwnd, int width, int height, const protocol::InitEngine& init) {
    hwnd_ = hwnd;
    width_ = width;
    height_ = height;
    mapCells_.fill(1);
    if (!CreateDeviceAndSwapchain(hwnd, width, height)) return false;
    if (!CreateShaders()) return false;
    if (!UpdateFontAtlas(init)) return false;
    if (!CreateMapBuffer()) return false;
    return true;
}

bool RendererD3D11::UpdateFontAtlas(const protocol::InitEngine& init) {
    if (init.atlasWidth <= 0 || init.atlasHeight <= 0 || init.atlasPixels.empty()) return false;
    if (!CreateAtlas(init)) return false;
    glyphs_.clear();
    for (const auto& ch : init.chars) {
        glyphs_[static_cast<std::uint32_t>(ch.r)] = GlyphInfo{ch.u1, ch.v1, ch.u2, ch.v2, ch.width, ch.height, ch.advance};
    }
    return true;
}

bool RendererD3D11::CreateDeviceAndSwapchain(HWND hwnd, int width, int height) {
    DXGI_SWAP_CHAIN_DESC desc{};
    desc.BufferCount = 2;
    desc.BufferDesc.Width = width;
    desc.BufferDesc.Height = height;
    desc.BufferDesc.Format = DXGI_FORMAT_R8G8B8A8_UNORM;
    desc.BufferUsage = DXGI_USAGE_RENDER_TARGET_OUTPUT;
    desc.OutputWindow = hwnd;
    desc.SampleDesc.Count = 1;
    desc.Windowed = TRUE;
    desc.SwapEffect = DXGI_SWAP_EFFECT_DISCARD;

    const UINT flags = D3D11_CREATE_DEVICE_BGRA_SUPPORT;
    D3D_FEATURE_LEVEL featureLevel{};
    const D3D_FEATURE_LEVEL levels[] = {D3D_FEATURE_LEVEL_11_0};
    const HRESULT hr = D3D11CreateDeviceAndSwapChain(
        nullptr, D3D_DRIVER_TYPE_HARDWARE, nullptr, flags, levels, 1, D3D11_SDK_VERSION,
        &desc, &swapchain_, &device_, &featureLevel, &context_);
    if (FAILED(hr)) return false;
    RecreateRenderTarget();
    return true;
}

bool RendererD3D11::CreateShaders() {
    auto vsBlob = CompileShader(kVertexShader, "main", "vs_5_0");
    auto psBlob = CompileShader(kPixelShader, "main", "ps_5_0");
    if (!vsBlob || !psBlob) return false;

    if (FAILED(device_->CreateVertexShader(vsBlob->GetBufferPointer(), vsBlob->GetBufferSize(), nullptr, &vertexShader_))) return false;
    if (FAILED(device_->CreatePixelShader(psBlob->GetBufferPointer(), psBlob->GetBufferSize(), nullptr, &pixelShader_))) return false;

    D3D11_INPUT_ELEMENT_DESC layout[] = {
        {"POSITION", 0, DXGI_FORMAT_R32G32_FLOAT, 0, offsetof(Vertex, x), D3D11_INPUT_PER_VERTEX_DATA, 0},
        {"TEXCOORD", 0, DXGI_FORMAT_R32G32_FLOAT, 0, offsetof(Vertex, u), D3D11_INPUT_PER_VERTEX_DATA, 0},
        {"COLOR", 0, DXGI_FORMAT_R32G32B32A32_FLOAT, 0, offsetof(Vertex, r), D3D11_INPUT_PER_VERTEX_DATA, 0},
        {"TEXCOORD", 1, DXGI_FORMAT_R32G32B32A32_FLOAT, 0, offsetof(Vertex, rectX), D3D11_INPUT_PER_VERTEX_DATA, 0},
        {"TEXCOORD", 2, DXGI_FORMAT_R32G32B32A32_FLOAT, 0, offsetof(Vertex, drawType), D3D11_INPUT_PER_VERTEX_DATA, 0},
        {"TEXCOORD", 3, DXGI_FORMAT_R32G32B32A32_FLOAT, 0, offsetof(Vertex, shadowOffsetX), D3D11_INPUT_PER_VERTEX_DATA, 0},
    };
    if (FAILED(device_->CreateInputLayout(layout, 6, vsBlob->GetBufferPointer(), vsBlob->GetBufferSize(), &inputLayout_))) return false;

    D3D11_BUFFER_DESC cbDesc{};
    cbDesc.ByteWidth = sizeof(ConstantBuffer);
    cbDesc.BindFlags = D3D11_BIND_CONSTANT_BUFFER;
    cbDesc.Usage = D3D11_USAGE_DYNAMIC;
    cbDesc.CPUAccessFlags = D3D11_CPU_ACCESS_WRITE;
    if (FAILED(device_->CreateBuffer(&cbDesc, nullptr, &constantBuffer_))) return false;

    D3D11_BLEND_DESC blend{};
    blend.RenderTarget[0].BlendEnable = TRUE;
    blend.RenderTarget[0].SrcBlend = D3D11_BLEND_SRC_ALPHA;
    blend.RenderTarget[0].DestBlend = D3D11_BLEND_INV_SRC_ALPHA;
    blend.RenderTarget[0].BlendOp = D3D11_BLEND_OP_ADD;
    blend.RenderTarget[0].SrcBlendAlpha = D3D11_BLEND_ONE;
    blend.RenderTarget[0].DestBlendAlpha = D3D11_BLEND_INV_SRC_ALPHA;
    blend.RenderTarget[0].BlendOpAlpha = D3D11_BLEND_OP_ADD;
    blend.RenderTarget[0].RenderTargetWriteMask = D3D11_COLOR_WRITE_ENABLE_ALL;
    if (FAILED(device_->CreateBlendState(&blend, &blendState_))) return false;

    D3D11_RASTERIZER_DESC rast{};
    rast.FillMode = D3D11_FILL_SOLID;
    rast.CullMode = D3D11_CULL_NONE;
    rast.ScissorEnable = TRUE;
    if (FAILED(device_->CreateRasterizerState(&rast, &rasterizer_))) return false;

    // GPU map pipeline: shaders over the raw cartography vertex format
    // (stride 32: float3 position, u8x4 colour; width/featureID skipped).
    auto mapVs = CompileShader(kMapShader, "vsmain", "vs_5_0");
    auto mapPs = CompileShader(kMapShader, "psmain", "ps_5_0");
    if (!mapVs || !mapPs) return false;
    if (FAILED(device_->CreateVertexShader(mapVs->GetBufferPointer(), mapVs->GetBufferSize(), nullptr, &mapVertexShader_))) return false;
    if (FAILED(device_->CreatePixelShader(mapPs->GetBufferPointer(), mapPs->GetBufferSize(), nullptr, &mapPixelShader_))) return false;
    D3D11_INPUT_ELEMENT_DESC mapLayout[] = {
        {"POSITION", 0, DXGI_FORMAT_R32G32B32_FLOAT, 0, 0, D3D11_INPUT_PER_VERTEX_DATA, 0},
        {"COLOR", 0, DXGI_FORMAT_R8G8B8A8_UNORM, 0, 12, D3D11_INPUT_PER_VERTEX_DATA, 0},
    };
    if (FAILED(device_->CreateInputLayout(mapLayout, 2, mapVs->GetBufferPointer(), mapVs->GetBufferSize(), &mapInputLayout_))) return false;

    cbDesc.ByteWidth = sizeof(MapViewConstants);
    if (FAILED(device_->CreateBuffer(&cbDesc, nullptr, &mapViewBuffer_))) return false;
    cbDesc.ByteWidth = sizeof(MapDrawConstants);
    if (FAILED(device_->CreateBuffer(&cbDesc, nullptr, &mapDrawBuffer_))) return false;

    // Depth semantics: the shader writes "nearness" (bigger = closer), so the
    // map path tests GREATER_EQUAL against a buffer cleared to 0. Equal-depth
    // ground layers then resolve by draw order, exactly like the flat path.
    D3D11_DEPTH_STENCIL_DESC depth{};
    depth.DepthEnable = TRUE;
    depth.DepthWriteMask = D3D11_DEPTH_WRITE_MASK_ALL;
    depth.DepthFunc = D3D11_COMPARISON_GREATER_EQUAL;
    if (FAILED(device_->CreateDepthStencilState(&depth, &mapDepthState_))) return false;
    depth.DepthEnable = FALSE;
    depth.DepthWriteMask = D3D11_DEPTH_WRITE_MASK_ZERO;
    if (FAILED(device_->CreateDepthStencilState(&depth, &uiDepthState_))) return false;

    D3D11_SAMPLER_DESC samp{};
    samp.Filter = D3D11_FILTER_MIN_MAG_MIP_LINEAR;
    samp.AddressU = D3D11_TEXTURE_ADDRESS_CLAMP;
    samp.AddressV = D3D11_TEXTURE_ADDRESS_CLAMP;
    samp.AddressW = D3D11_TEXTURE_ADDRESS_CLAMP;
    samp.MaxLOD = D3D11_FLOAT32_MAX;
    if (FAILED(device_->CreateSamplerState(&samp, &sampler_))) return false;

    return true;
}

bool RendererD3D11::CreateAtlas(const protocol::InitEngine& init) {
    D3D11_TEXTURE2D_DESC desc{};
    desc.Width = static_cast<UINT>(init.atlasWidth);
    desc.Height = static_cast<UINT>(init.atlasHeight);
    desc.MipLevels = 1;
    desc.ArraySize = 1;
    desc.Format = DXGI_FORMAT_R8G8B8A8_UNORM;
    desc.SampleDesc.Count = 1;
    desc.BindFlags = D3D11_BIND_SHADER_RESOURCE;

    D3D11_SUBRESOURCE_DATA data{};
    data.pSysMem = init.atlasPixels.data();
    data.SysMemPitch = static_cast<UINT>(init.atlasWidth * 4);

    Microsoft::WRL::ComPtr<ID3D11Texture2D> texture;
    if (FAILED(device_->CreateTexture2D(&desc, &data, &texture))) return false;

    D3D11_SHADER_RESOURCE_VIEW_DESC srvDesc{};
    srvDesc.Format = desc.Format;
    srvDesc.ViewDimension = D3D11_SRV_DIMENSION_TEXTURE2D;
    srvDesc.Texture2D.MipLevels = 1;
    if (FAILED(device_->CreateShaderResourceView(texture.Get(), &srvDesc, &atlasSrv_))) return false;
    return true;
}

bool RendererD3D11::CreateMapBuffer() {
    D3D11_BUFFER_DESC desc{};
    desc.ByteWidth = static_cast<UINT>(mapCells_.size() * sizeof(std::uint32_t));
    desc.Usage = D3D11_USAGE_DEFAULT;
    desc.BindFlags = D3D11_BIND_SHADER_RESOURCE;
    desc.MiscFlags = D3D11_RESOURCE_MISC_BUFFER_STRUCTURED;
    desc.StructureByteStride = sizeof(std::uint32_t);

    D3D11_SUBRESOURCE_DATA data{};
    data.pSysMem = mapCells_.data();
    if (FAILED(device_->CreateBuffer(&desc, &data, &mapBuffer_))) return false;

    D3D11_SHADER_RESOURCE_VIEW_DESC srvDesc{};
    srvDesc.ViewDimension = D3D11_SRV_DIMENSION_BUFFEREX;
    srvDesc.Format = DXGI_FORMAT_UNKNOWN;
    srvDesc.BufferEx.FirstElement = 0;
    srvDesc.BufferEx.NumElements = static_cast<UINT>(mapCells_.size());
    if (FAILED(device_->CreateShaderResourceView(mapBuffer_.Get(), &srvDesc, &mapSrv_))) return false;
    return true;
}

bool RendererD3D11::EnsureImageTexture(int width, int height, const std::vector<std::uint8_t>& bytes) {
    if (width <= 0 || height <= 0) return false;
    if (bytes.size() < static_cast<std::size_t>(width * height * 4)) return false;

    if (!imageTexture_ || imageTextureWidth_ != width || imageTextureHeight_ != height) {
        imageTexture_.Reset();
        imageSrv_.Reset();

        D3D11_TEXTURE2D_DESC desc{};
        desc.Width = static_cast<UINT>(width);
        desc.Height = static_cast<UINT>(height);
        desc.MipLevels = 1;
        desc.ArraySize = 1;
        desc.Format = DXGI_FORMAT_R8G8B8A8_UNORM;
        desc.SampleDesc.Count = 1;
        desc.Usage = D3D11_USAGE_DEFAULT;
        desc.BindFlags = D3D11_BIND_SHADER_RESOURCE;

        if (FAILED(device_->CreateTexture2D(&desc, nullptr, &imageTexture_))) return false;

        D3D11_SHADER_RESOURCE_VIEW_DESC srvDesc{};
        srvDesc.Format = desc.Format;
        srvDesc.ViewDimension = D3D11_SRV_DIMENSION_TEXTURE2D;
        srvDesc.Texture2D.MipLevels = 1;
        if (FAILED(device_->CreateShaderResourceView(imageTexture_.Get(), &srvDesc, &imageSrv_))) return false;

        imageTextureWidth_ = width;
        imageTextureHeight_ = height;
    }

    context_->UpdateSubresource(imageTexture_.Get(), 0, nullptr, bytes.data(), static_cast<UINT>(width * 4), 0);
    return true;
}

void RendererD3D11::RecreateRenderTarget() {
    rtv_.Reset();
    Microsoft::WRL::ComPtr<ID3D11Texture2D> backbuffer;
    swapchain_->GetBuffer(0, IID_PPV_ARGS(&backbuffer));
    device_->CreateRenderTargetView(backbuffer.Get(), nullptr, &rtv_);

    // Depth buffer for the GPU map path, sized with the backbuffer.
    depthView_.Reset();
    D3D11_TEXTURE2D_DESC backDesc{};
    backbuffer->GetDesc(&backDesc);
    D3D11_TEXTURE2D_DESC depthDesc{};
    depthDesc.Width = backDesc.Width;
    depthDesc.Height = backDesc.Height;
    depthDesc.MipLevels = 1;
    depthDesc.ArraySize = 1;
    depthDesc.Format = DXGI_FORMAT_D24_UNORM_S8_UINT;
    depthDesc.SampleDesc.Count = 1;
    depthDesc.Usage = D3D11_USAGE_DEFAULT;
    depthDesc.BindFlags = D3D11_BIND_DEPTH_STENCIL;
    Microsoft::WRL::ComPtr<ID3D11Texture2D> depthTexture;
    if (SUCCEEDED(device_->CreateTexture2D(&depthDesc, nullptr, &depthTexture))) {
        device_->CreateDepthStencilView(depthTexture.Get(), nullptr, &depthView_);
    }
}

void RendererD3D11::Resize(int width, int height) {
    if (width <= 0 || height <= 0 || !swapchain_) return;
    width_ = width;
    height_ = height;
    context_->OMSetRenderTargets(0, nullptr, nullptr);
    rtv_.Reset();
    swapchain_->ResizeBuffers(0, width, height, DXGI_FORMAT_UNKNOWN, 0);
    RecreateRenderTarget();
}

void RendererD3D11::ApplyMapScene(const protocol::MapSceneDelta& scene) {
    auto& retained = mapScenes_[scene.viewportId];
    if (scene.generation <= retained.generation) return;
    for (const auto& resource : scene.resources) {
        const auto key = MapHashKey(resource.hash);
        if (resource.operation == protocol::MapResourceOperation::Release) {
			retained.resources.erase(key);
			retained.textures.erase(key);
			retained.gpuVertexBuffers.erase(key);
			retained.gpuIndexBuffers.erase(key);
		} else {
			retained.resources[key] = resource;
			retained.textures.erase(key);
			retained.gpuVertexBuffers.erase(key);
			retained.gpuIndexBuffers.erase(key);
		}
    }
    retained.generation = scene.generation;
    retained.camera = scene.camera;
    retained.sunAzimuth = scene.sunAzimuth;
    retained.sunElevation = scene.sunElevation;
    retained.fogDensity = scene.fogDensity;
    retained.fogRed = scene.fogRed;
    retained.fogGreen = scene.fogGreen;
    retained.fogBlue = scene.fogBlue;
    retained.draws = scene.draws;
	retained.vertexBuffer.Reset();
	retained.vertexCount = 0;
	retained.geometryRanges.clear();
	std::unordered_set<std::string> referenced;
	for (const auto& draw : retained.draws) {
		referenced.insert(MapHashKey(draw.vertexHash));
		referenced.insert(MapHashKey(draw.indexHash));
		referenced.insert(MapHashKey(draw.textureHash));
	}
	for (auto resource = retained.resources.begin(); resource != retained.resources.end();) {
		if (referenced.find(resource->first) == referenced.end()) {
			retained.textures.erase(resource->first);
			retained.gpuVertexBuffers.erase(resource->first);
			retained.gpuIndexBuffers.erase(resource->first);
			resource = retained.resources.erase(resource);
		} else ++resource;
	}
}

bool RendererD3D11::EnsureMapGeometryBuffer(const std::string& viewportId, float left, float top, float right, float bottom, float previewScale) {
	auto found = mapScenes_.find(viewportId);
	if (found == mapScenes_.end() || found->second.draws.empty()) return false;
	auto& scene = found->second;
	if (scene.vertexBuffer && scene.vertexCount > 0 && scene.left == left && scene.top == top && scene.right == right && scene.bottom == bottom && scene.previewScale == previewScale) return true;
	std::vector<Vertex> vertices;
	std::vector<MapGeometryRange> ranges;
	AppendMapGeometry(vertices, ranges, viewportId, left, top, right, bottom, previewScale);
	if (vertices.empty()) {
		scene.vertexBuffer.Reset();
		scene.vertexCount = 0;
		return false;
	}
	D3D11_BUFFER_DESC desc{};
	desc.ByteWidth = static_cast<UINT>(vertices.size() * sizeof(Vertex));
	desc.Usage = D3D11_USAGE_IMMUTABLE;
	desc.BindFlags = D3D11_BIND_VERTEX_BUFFER;
	D3D11_SUBRESOURCE_DATA initial{};
	initial.pSysMem = vertices.data();
	Microsoft::WRL::ComPtr<ID3D11Buffer> buffer;
	if (FAILED(device_->CreateBuffer(&desc, &initial, &buffer))) return false;
	scene.vertexBuffer = std::move(buffer);
	scene.vertexCount = static_cast<std::uint32_t>(vertices.size());
	scene.geometryRanges = std::move(ranges);
	scene.left = left; scene.top = top; scene.right = right; scene.bottom = bottom;
	scene.previewScale = previewScale;
	return true;
}

void RendererD3D11::AppendMapGeometry(std::vector<Vertex>& vertices, std::vector<MapGeometryRange>& ranges, const std::string& viewportId, float left, float top, float right, float bottom, float previewScale) {
    const auto project = [](double longitude, double latitude) {
        latitude = std::max(-85.05112878, std::min(85.05112878, latitude));
        const double x = (longitude + 180.0) / 360.0;
        const double sine = std::sin(latitude * 3.14159265358979323846 / 180.0);
        const double y = .5 - std::log((1.0 + sine) / (1.0 - sine)) / (4.0 * 3.14159265358979323846);
        return std::array<double, 2>{x, y};
    };
    struct MapVertex { float x, y, z, width; float r, g, b, a; float u, v, offsetX, offsetY; };
    auto readVertex = [](const protocol::MapSceneResource& resource, std::uint32_t index, MapVertex& out) {
        if (resource.stride < 28 || static_cast<std::uint64_t>(index + 1) * resource.stride > resource.bytes.size()) return false;
        const auto* data = resource.bytes.data() + static_cast<std::size_t>(index) * resource.stride;
        std::memcpy(&out.x, data, 4); std::memcpy(&out.y, data + 4, 4); std::memcpy(&out.z, data + 8, 4); std::memcpy(&out.width, data + 16, 4);
        out.r = data[12] / 255.0f; out.g = data[13] / 255.0f; out.b = data[14] / 255.0f; out.a = data[15] / 255.0f;
		if (resource.stride >= 44) { std::memcpy(&out.u, data + 28, 4); std::memcpy(&out.v, data + 32, 4); std::memcpy(&out.offsetX, data + 36, 4); std::memcpy(&out.offsetY, data + 40, 4); }
        return std::isfinite(out.x) && std::isfinite(out.y) && std::isfinite(out.z) && std::isfinite(out.width);
    };
    auto solidVertex = [](float x, float y, const MapVertex& source) {
        return Vertex{x, y, 0, 0, source.r, source.g, source.b, source.a, x, y, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0};
    };
    for (const auto& sceneEntry : mapScenes_) {
      if (sceneEntry.first != viewportId) continue;
      const auto& retained = sceneEntry.second;
      if (retained.draws.empty() || retained.camera.viewportWidth == 0 || retained.camera.viewportHeight == 0) continue;
      // Camera transform, solar direction and face shading all come from the
      // shared, platform-neutral module so this presenter cannot drift from the
      // GLES one (see shared/poem/map_view.h).
      const auto view = poem::mapview::Make(retained.camera.latitude, retained.camera.longitude, retained.camera.zoom,
                                            retained.camera.bearing, retained.camera.pitch, previewScale,
                                            left, top, right, bottom);
      const auto sun = poem::mapview::SunDirection(view, retained.sunAzimuth, retained.sunElevation);
      auto screen = [&](float worldX, float worldY, float elevation) {
          const auto point = poem::mapview::Project(view, worldX, worldY, elevation);
          return std::array<float, 2>{point.x, point.y};
      };
	  // Only extruded batches (draw.depthTest, set from the style's Extrude) are
	  // shaded, so flat land/water keep exactly the colours the style asked for.
	  auto shadeTriangle = [&](const MapVertex& a, const MapVertex& b, const MapVertex& c) {
		  return poem::mapview::ShadeTriangle(view, sun, a.x, a.y, a.z, b.x, b.y, b.z, c.x, c.y, c.z);
	  };
	  auto applyShade = [](MapVertex& v, float shade) { v.r *= shade; v.g *= shade; v.b *= shade; };
	  // Height fog applies to every map layer, not just extrusions: the ground
	  // has to recede too, or buildings fade into a crisp landscape.
	  auto applyFog = [&](MapVertex& v) {
		  const float factor = poem::mapview::FogFactor(view, v.x, v.y, v.z, retained.fogDensity);
		  if (factor <= 0.0f) return;
		  v.r = poem::mapview::MixFog(v.r, retained.fogRed, factor);
		  v.g = poem::mapview::MixFog(v.g, retained.fogGreen, factor);
		  v.b = poem::mapview::MixFog(v.b, retained.fogBlue, factor);
	  };
	  auto depthOf = [&](float worldX, float worldY, float elevation) {
		  return poem::mapview::Depth(view, worldX, worldY, elevation);
	  };

      for (const auto& draw : retained.draws) {
		const auto rangeStart = static_cast<std::uint32_t>(vertices.size());
		const bool textured = std::any_of(draw.textureHash.begin(), draw.textureHash.end(), [](std::uint8_t value) { return value != 0; });
		// Untextured triangles (ground, extrusions) are drawn by the GPU map
		// path straight from the retained buffers; the CPU composite carries
		// only dots, lines and textured label quads on top.
		if (draw.primitive == protocol::MapPrimitive::Triangles && !textured) continue;
        const auto vertexIt = retained.resources.find(MapHashKey(draw.vertexHash));
        const auto indexIt = retained.resources.find(MapHashKey(draw.indexHash));
        if (vertexIt == retained.resources.end() || indexIt == retained.resources.end() || indexIt->second.stride != 4) continue;
        const auto& vertexResource = vertexIt->second;
        const auto& indexResource = indexIt->second;
        if (static_cast<std::uint64_t>(draw.first + draw.count) * 4 > indexResource.bytes.size()) continue;
        auto indexAt = [&](std::uint32_t offset) { std::uint32_t value{}; std::memcpy(&value, indexResource.bytes.data() + static_cast<std::size_t>(draw.first + offset) * 4, 4); return value; };
        if (draw.primitive == protocol::MapPrimitive::Lines) {
            for (std::uint32_t i = 0; i + 1 < draw.count; i += 2) {
                MapVertex a{}, b{}; if (!readVertex(vertexResource, indexAt(i), a) || !readVertex(vertexResource, indexAt(i + 1), b)) continue;
                const auto pa = screen(a.x, a.y, a.z), pb = screen(b.x, b.y, b.z);
                AppendLineQuad(vertices, pa[0], pa[1], pb[0], pb[1], std::max(1.0f, a.width), a.r, a.g, a.b, a.a * draw.opacity);
            }
        } else if (draw.primitive == protocol::MapPrimitive::Triangles && textured) {
			for (std::uint32_t i = 0; i + 2 < draw.count; i += 3) {
				MapVertex points[3]{}; bool valid = true;
				for (int p = 0; p < 3; ++p) valid = valid && readVertex(vertexResource, indexAt(i + p), points[p]);
				if (!valid || vertexResource.stride < 44) continue;
				for (auto& point : points) {
					const auto projected = screen(point.x, point.y, point.z);
					vertices.push_back(Vertex{projected[0] + point.offsetX, projected[1] + point.offsetY, point.u, point.v, point.r, point.g, point.b, point.a * draw.opacity, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0});
				}
			}
        } else if (draw.primitive == protocol::MapPrimitive::Triangles && draw.depthTest) {
            // Extruded geometry: the map is CPU-projected to 2D and there is no
            // depth buffer, so sort faces back-to-front (painter's algorithm) or
            // buildings behind paint over the ones in front. Shade each face
            // against the sun while its 3D positions are in hand.
            struct Face { std::array<float, 2> pa, pb, pc; MapVertex a, b, c; double depth; };
            std::vector<Face> faces;
            faces.reserve(draw.count / 3);
            for (std::uint32_t i = 0; i + 2 < draw.count; i += 3) {
                MapVertex a{}, b{}, c{}; if (!readVertex(vertexResource, indexAt(i), a) || !readVertex(vertexResource, indexAt(i + 1), b) || !readVertex(vertexResource, indexAt(i + 2), c)) continue;
                a.a *= draw.opacity; b.a *= draw.opacity; c.a *= draw.opacity;
                const float shade = shadeTriangle(a, b, c);
                applyShade(a, shade); applyShade(b, shade); applyShade(c, shade);
                applyFog(a); applyFog(b); applyFog(c);
                faces.push_back(Face{screen(a.x, a.y, a.z), screen(b.x, b.y, b.z), screen(c.x, c.y, c.z), a, b, c,
                                     (depthOf(a.x, a.y, a.z) + depthOf(b.x, b.y, b.z) + depthOf(c.x, c.y, c.z)) / 3.0});
            }
            // Depth grows toward the camera, so ascending draws far first.
            // Looking straight down nothing occludes, so skip the sort - it is
            // the expensive part of a rebuild and pan/zoom rebuilds run often.
            if (poem::mapview::NeedsDepthSort(retained.camera.pitch)) {
                std::sort(faces.begin(), faces.end(), [](const Face& l, const Face& r) { return l.depth < r.depth; });
            }
            for (const auto& face : faces) {
                vertices.push_back(solidVertex(face.pa[0], face.pa[1], face.a));
                vertices.push_back(solidVertex(face.pb[0], face.pb[1], face.b));
                vertices.push_back(solidVertex(face.pc[0], face.pc[1], face.c));
            }
        } else if (draw.primitive == protocol::MapPrimitive::Triangles) {
            for (std::uint32_t i = 0; i + 2 < draw.count; i += 3) {
                MapVertex a{}, b{}, c{}; if (!readVertex(vertexResource, indexAt(i), a) || !readVertex(vertexResource, indexAt(i + 1), b) || !readVertex(vertexResource, indexAt(i + 2), c)) continue;
                const auto pa = screen(a.x, a.y, a.z), pb = screen(b.x, b.y, b.z), pc = screen(c.x, c.y, c.z);
                a.a *= draw.opacity; b.a *= draw.opacity; c.a *= draw.opacity;
                applyFog(a); applyFog(b); applyFog(c);
                vertices.push_back(solidVertex(pa[0], pa[1], a)); vertices.push_back(solidVertex(pb[0], pb[1], b)); vertices.push_back(solidVertex(pc[0], pc[1], c));
            }
        } else {
            for (std::uint32_t i = 0; i < draw.count; ++i) {
                MapVertex point{}; if (!readVertex(vertexResource, indexAt(i), point)) continue;
                const auto p = screen(point.x, point.y, point.z); const float radius = std::max(3.0f, point.width);
                AppendRect(vertices, p[0] - radius, p[1] - radius, p[0] + radius, p[1] + radius, point.r, point.g, point.b, point.a * draw.opacity, 0, 0, 0, radius, 0, 0, 0, 0);
            }
        }
		const auto rangeCount = static_cast<std::uint32_t>(vertices.size()) - rangeStart;
		if (rangeCount > 0) ranges.push_back(MapGeometryRange{rangeStart, rangeCount, textured ? MapHashKey(draw.textureHash) : std::string{}});
      }
    }
}

ID3D11Buffer* RendererD3D11::EnsureMapGpuBuffer(RetainedMapScene& scene, const std::string& hash, bool index) {
	auto& cache = index ? scene.gpuIndexBuffers : scene.gpuVertexBuffers;
	if (const auto cached = cache.find(hash); cached != cache.end()) return cached->second.Get();
	const auto resource = scene.resources.find(hash);
	if (resource == scene.resources.end() || resource->second.bytes.empty()) return nullptr;
	D3D11_BUFFER_DESC desc{};
	desc.ByteWidth = static_cast<UINT>(resource->second.bytes.size());
	desc.Usage = D3D11_USAGE_IMMUTABLE;
	desc.BindFlags = index ? D3D11_BIND_INDEX_BUFFER : D3D11_BIND_VERTEX_BUFFER;
	D3D11_SUBRESOURCE_DATA data{resource->second.bytes.data(), 0, 0};
	Microsoft::WRL::ComPtr<ID3D11Buffer> buffer;
	if (FAILED(device_->CreateBuffer(&desc, &data, &buffer))) return nullptr;
	ID3D11Buffer* raw = buffer.Get();
	cache.emplace(hash, std::move(buffer));
	return raw;
}

void RendererD3D11::DrawGpuMapBatches(RetainedMapScene& scene, const DrawRange& range) {
	if (!mapVertexShader_ || !depthView_) return;
	bool any = false;
	for (const auto& draw : scene.draws) {
		const bool textured = std::any_of(draw.textureHash.begin(), draw.textureHash.end(), [](std::uint8_t value) { return value != 0; });
		if (draw.primitive == protocol::MapPrimitive::Triangles && !textured) { any = true; break; }
	}
	if (!any) return;

	const auto view = poem::mapview::Make(scene.camera.latitude, scene.camera.longitude, scene.camera.zoom,
	                                      scene.camera.bearing, scene.camera.pitch, range.mapScale,
	                                      range.mapLeft, range.mapTop, range.mapRight, range.mapBottom);
	const auto sun = poem::mapview::SunDirection(view, scene.sunAzimuth, scene.sunElevation);

	MapViewConstants constants{};
	constants.centerHiX = static_cast<float>(view.centerX);
	constants.centerLoX = static_cast<float>(view.centerX - static_cast<double>(constants.centerHiX));
	constants.centerHiY = static_cast<float>(view.centerY);
	constants.centerLoY = static_cast<float>(view.centerY - static_cast<double>(constants.centerHiY));
	constants.worldPixels = static_cast<float>(view.worldPixels);
	constants.cosBearing = static_cast<float>(view.cosBearing);
	constants.sinBearing = static_cast<float>(view.sinBearing);
	constants.metersToPixels = static_cast<float>(view.metersToPixels);
	constants.pitchCos = static_cast<float>(view.pitchCos);
	constants.pitchSin = static_cast<float>(view.pitchSin);
	constants.cameraDistance = static_cast<float>(view.cameraDistance);
	constants.screenWidth = static_cast<float>(width_);
	constants.screenHeight = static_cast<float>(height_);
	constants.rectCenterX = static_cast<float>((view.left + view.right) * .5);
	constants.rectCenterY = static_cast<float>((view.top + view.bottom) * .5);
	constants.ambient = static_cast<float>(poem::mapview::kDefaultAmbient);
	constants.sunX = static_cast<float>(sun.x);
	constants.sunY = static_cast<float>(sun.y);
	constants.sunZ = static_cast<float>(sun.z);
	constants.fogDensity = scene.fogDensity;
	constants.fogR = scene.fogRed;
	constants.fogG = scene.fogGreen;
	constants.fogB = scene.fogBlue;
	constants.fogPitch = static_cast<float>(view.pitchSin);
	constants.fogReferenceMeters = static_cast<float>(poem::mapview::kFogReferenceMeters);
	constants.fogScaleHeightMeters = static_cast<float>(poem::mapview::kFogScaleHeightMeters);

	D3D11_MAPPED_SUBRESOURCE mapped{};
	if (SUCCEEDED(context_->Map(mapViewBuffer_.Get(), 0, D3D11_MAP_WRITE_DISCARD, 0, &mapped))) {
		std::memcpy(mapped.pData, &constants, sizeof(constants));
		context_->Unmap(mapViewBuffer_.Get(), 0);
	}

	context_->IASetInputLayout(mapInputLayout_.Get());
	context_->VSSetShader(mapVertexShader_.Get(), nullptr, 0);
	context_->PSSetShader(mapPixelShader_.Get(), nullptr, 0);
	ID3D11Buffer* viewBuffers[2] = {mapViewBuffer_.Get(), mapDrawBuffer_.Get()};
	context_->VSSetConstantBuffers(0, 2, viewBuffers);
	context_->PSSetConstantBuffers(0, 2, viewBuffers);
	context_->OMSetDepthStencilState(mapDepthState_.Get(), 0);

	const UINT stride = 32, offset = 0;
	for (const auto& draw : scene.draws) {
		const bool textured = std::any_of(draw.textureHash.begin(), draw.textureHash.end(), [](std::uint8_t value) { return value != 0; });
		if (draw.primitive != protocol::MapPrimitive::Triangles || textured) continue;
		ID3D11Buffer* vertexBuffer = EnsureMapGpuBuffer(scene, MapHashKey(draw.vertexHash), false);
		ID3D11Buffer* indexBuffer = EnsureMapGpuBuffer(scene, MapHashKey(draw.indexHash), true);
		if (!vertexBuffer || !indexBuffer) continue;
		MapDrawConstants perDraw{draw.opacity, draw.depthTest ? 1.0f : 0.0f, {0, 0}};
		if (SUCCEEDED(context_->Map(mapDrawBuffer_.Get(), 0, D3D11_MAP_WRITE_DISCARD, 0, &mapped))) {
			std::memcpy(mapped.pData, &perDraw, sizeof(perDraw));
			context_->Unmap(mapDrawBuffer_.Get(), 0);
		}
		context_->IASetVertexBuffers(0, 1, &vertexBuffer, &stride, &offset);
		context_->IASetIndexBuffer(indexBuffer, DXGI_FORMAT_R32_UINT, 0);
		context_->DrawIndexed(draw.count, draw.first, 0);
	}

	// Restore the UI pipeline (the caller rebinds the UI vertex buffer).
	context_->OMSetDepthStencilState(uiDepthState_.Get(), 0);
	context_->IASetInputLayout(inputLayout_.Get());
	context_->VSSetShader(vertexShader_.Get(), nullptr, 0);
	context_->PSSetShader(pixelShader_.Get(), nullptr, 0);
	context_->VSSetConstantBuffers(0, 1, constantBuffer_.GetAddressOf());
	context_->PSSetConstantBuffers(0, 1, constantBuffer_.GetAddressOf());
}

bool RendererD3D11::EnsureMapTexture(RetainedMapScene& scene, const std::string& hash) {
	if (hash.empty()) return true;
	if (scene.textures.find(hash) != scene.textures.end()) return true;
	const auto found = scene.resources.find(hash);
	if (found == scene.resources.end()) return false;
	const auto& resource = found->second;
	if (resource.type != protocol::MapResourceType::TextureAlpha || resource.width == 0 || resource.height == 0 || resource.width > 4096 || resource.height > 4096 || resource.bytes.size() != static_cast<std::size_t>(resource.width) * resource.height) return false;
	D3D11_TEXTURE2D_DESC desc{};
	desc.Width = resource.width; desc.Height = resource.height; desc.MipLevels = 1; desc.ArraySize = 1;
	desc.Format = DXGI_FORMAT_A8_UNORM; desc.SampleDesc.Count = 1; desc.BindFlags = D3D11_BIND_SHADER_RESOURCE;
	D3D11_SUBRESOURCE_DATA data{}; data.pSysMem = resource.bytes.data(); data.SysMemPitch = resource.width;
	Microsoft::WRL::ComPtr<ID3D11Texture2D> texture;
	if (FAILED(device_->CreateTexture2D(&desc, &data, &texture))) return false;
	D3D11_SHADER_RESOURCE_VIEW_DESC srvDesc{}; srvDesc.Format = desc.Format; srvDesc.ViewDimension = D3D11_SRV_DIMENSION_TEXTURE2D; srvDesc.Texture2D.MipLevels = 1;
	Microsoft::WRL::ComPtr<ID3D11ShaderResourceView> srv;
	if (FAILED(device_->CreateShaderResourceView(texture.Get(), &srvDesc, &srv))) return false;
	scene.textures.emplace(hash, std::move(srv));
	return true;
}

bool RendererD3D11::CaptureBackbufferRGBA(std::vector<std::uint8_t>& rgba, int& width, int& height) {
    rgba.clear();
    width = 0;
    height = 0;

    if (!device_ || !context_ || !swapchain_) {
        return false;
    }

    Microsoft::WRL::ComPtr<ID3D11Texture2D> backbuffer;
    if (FAILED(swapchain_->GetBuffer(0, IID_PPV_ARGS(&backbuffer)))) {
        return false;
    }

    D3D11_TEXTURE2D_DESC desc{};
    backbuffer->GetDesc(&desc);
    width = static_cast<int>(desc.Width);
    height = static_cast<int>(desc.Height);
    if (width <= 0 || height <= 0) {
        return false;
    }

    D3D11_TEXTURE2D_DESC stagingDesc = desc;
    stagingDesc.BindFlags = 0;
    stagingDesc.MiscFlags = 0;
    stagingDesc.Usage = D3D11_USAGE_STAGING;
    stagingDesc.CPUAccessFlags = D3D11_CPU_ACCESS_READ;

    Microsoft::WRL::ComPtr<ID3D11Texture2D> staging;
    if (FAILED(device_->CreateTexture2D(&stagingDesc, nullptr, &staging))) {
        return false;
    }

    context_->CopyResource(staging.Get(), backbuffer.Get());

    D3D11_MAPPED_SUBRESOURCE mapped{};
    if (FAILED(context_->Map(staging.Get(), 0, D3D11_MAP_READ, 0, &mapped))) {
        return false;
    }

    rgba.resize(static_cast<std::size_t>(width * height * 4));
    for (int y = 0; y < height; ++y) {
        const auto* src = static_cast<const std::uint8_t*>(mapped.pData) + static_cast<std::size_t>(y) * mapped.RowPitch;
        auto* dst = rgba.data() + static_cast<std::size_t>(y * width * 4);
        for (int x = 0; x < width; ++x) {
            const std::size_t srcIdx = static_cast<std::size_t>(x * 4);
            const std::size_t dstIdx = static_cast<std::size_t>(x * 4);
            dst[dstIdx + 0] = src[srcIdx + 2];
            dst[dstIdx + 1] = src[srcIdx + 1];
            dst[dstIdx + 2] = src[srcIdx + 0];
            dst[dstIdx + 3] = src[srcIdx + 3];
        }
    }

    context_->Unmap(staging.Get(), 0);
    return true;
}

void RendererD3D11::EnsureVertexCapacity(std::size_t vertexCount) {
    const std::size_t bytes = std::max<std::size_t>(vertexCount * sizeof(Vertex), 4096);
    if (vertexBuffer_) {
        D3D11_BUFFER_DESC desc{};
        vertexBuffer_->GetDesc(&desc);
        if (desc.ByteWidth >= bytes) return;
    }
    vertexBuffer_.Reset();
    D3D11_BUFFER_DESC desc{};
    desc.ByteWidth = static_cast<UINT>(bytes);
    desc.BindFlags = D3D11_BIND_VERTEX_BUFFER;
    desc.Usage = D3D11_USAGE_DYNAMIC;
    desc.CPUAccessFlags = D3D11_CPU_ACCESS_WRITE;
    device_->CreateBuffer(&desc, nullptr, &vertexBuffer_);
}

void RendererD3D11::AppendQuad(std::vector<Vertex>& vertices, float x1, float y1, float x2, float y2,
                               float u1, float v1, float u2, float v2,
                               float r, float g, float b, float a,
                               float rectX, float rectY, float rectW, float rectH,
                               float drawType, float glow, float isGlass, float radius,
                               float shadowOffsetX, float shadowOffsetY, float shadowSoftness, float padding) {
    Vertex quad[6] = {
        {x1, y1, u1, v1, r, g, b, a, rectX, rectY, rectW, rectH, drawType, glow, isGlass, radius, shadowOffsetX, shadowOffsetY, shadowSoftness, padding},
        {x2, y1, u2, v1, r, g, b, a, rectX, rectY, rectW, rectH, drawType, glow, isGlass, radius, shadowOffsetX, shadowOffsetY, shadowSoftness, padding},
        {x1, y2, u1, v2, r, g, b, a, rectX, rectY, rectW, rectH, drawType, glow, isGlass, radius, shadowOffsetX, shadowOffsetY, shadowSoftness, padding},
        {x1, y2, u1, v2, r, g, b, a, rectX, rectY, rectW, rectH, drawType, glow, isGlass, radius, shadowOffsetX, shadowOffsetY, shadowSoftness, padding},
        {x2, y1, u2, v1, r, g, b, a, rectX, rectY, rectW, rectH, drawType, glow, isGlass, radius, shadowOffsetX, shadowOffsetY, shadowSoftness, padding},
        {x2, y2, u2, v2, r, g, b, a, rectX, rectY, rectW, rectH, drawType, glow, isGlass, radius, shadowOffsetX, shadowOffsetY, shadowSoftness, padding},
    };
    vertices.insert(vertices.end(), std::begin(quad), std::end(quad));
}

void RendererD3D11::AppendLineQuad(std::vector<Vertex>& vertices, float x1, float y1, float x2, float y2, float thickness,
                                   float r, float g, float b, float a) {
    const float dx = x2 - x1;
    const float dy = y2 - y1;
    const float len = std::sqrt(dx * dx + dy * dy);
    if (len <= 0.001f) return;
    const float nx = -dy / len;
    const float ny = dx / len;
    const float hw = thickness * 0.5f;
    const float rx1 = std::min(x1 - hw, x2 - hw);
    const float ry1 = std::min(y1 - hw, y2 - hw);
    AppendQuad(vertices,
               x1 + nx * hw, y1 + ny * hw,
               x2 + nx * hw, y2 + ny * hw,
               0, 0, 1, 0,
               r, g, b, a,
               rx1, ry1, len, thickness,
               0.0f, 0.0f, 0.0f, 0.0f,
               0.0f, 0.0f, 0.0f, 0.0f);
    AppendQuad(vertices,
               x1 - nx * hw, y1 - ny * hw,
               x2 - nx * hw, y2 - ny * hw,
               0, 1, 1, 1,
               r, g, b, a,
               rx1, ry1, len, thickness,
               0.0f, 0.0f, 0.0f, 0.0f,
               0.0f, 0.0f, 0.0f, 0.0f);
}

void RendererD3D11::AppendRect(std::vector<Vertex>& vertices, float x1, float y1, float x2, float y2,
                               float r, float g, float b, float a,
                               float drawType, float glow, float isGlass, float radius,
                               float shadowOffsetX, float shadowOffsetY, float shadowSoftness, float padding) {
    AppendQuad(vertices, x1, y1, x2, y2, 0, 0, 1, 1, r, g, b, a, x1, y1, x2 - x1, y2 - y1,
               drawType, glow, isGlass, radius, shadowOffsetX, shadowOffsetY, shadowSoftness, padding);
}

void RendererD3D11::AppendTextGlyph(std::vector<Vertex>& vertices, float x1, float y1, float x2, float y2,
                                    float u1, float v1, float u2, float v2, float r, float g, float b, float a) {
    AppendQuad(vertices, x1, y1, x2, y2, u1, v1, u2, v2, r, g, b, a, x1, y1, x2 - x1, y2 - y1,
               1.0f, 0.0f, 0.0f, 0.0f, 0.0f, 0.0f, 0.0f, 0.0f);
}

void RendererD3D11::AppendImageQuad(std::vector<Vertex>& vertices, float x1, float y1, float x2, float y2) {
    AppendQuad(vertices, x1, y1, x2, y2, 0, 0, 1, 1, 1, 1, 1, 1, x1, y1, x2 - x1, y2 - y1,
               2.0f, 0.0f, 0.0f, 0.0f, 0.0f, 0.0f, 0.0f, 0.0f);
}

void RendererD3D11::UpdateRaycasterMap(const std::string& mapText) {
    if (mapText.size() != mapCells_.size() || mapText == mapSignature_) {
        return;
    }
    for (std::size_t i = 0; i < mapCells_.size(); ++i) {
        switch (mapText[i]) {
        case '0': mapCells_[i] = 0; break;
        case '2': mapCells_[i] = 2; break;
        default: mapCells_[i] = 1; break;
        }
    }
    mapSignature_ = mapText;
    context_->UpdateSubresource(mapBuffer_.Get(), 0, nullptr, mapCells_.data(), 0, 0);
}

void RendererD3D11::AppendRaycasterQuad(std::vector<Vertex>& vertices, const protocol::DrawCommand& cmd, float x1, float y1, float x2, float y2,
                                        RaycasterState& state) {
    UpdateRaycasterMap(cmd.text);
    state.valid = true;
    state.viewportX1 = x1;
    state.viewportY1 = y1;
    state.viewportX2 = x2;
    state.viewportY2 = y2;
    state.playerX = cmd.val1;
    state.playerY = cmd.val2;
    state.playerAngle = cmd.val3;
    AppendRect(vertices, x1, y1, x2, y2,
               cmd.r / 255.0f, cmd.g / 255.0f, cmd.b / 255.0f, 1.0f,
               0.0f, 0.0f, 0.0f, static_cast<float>(cmd.radius),
               cmd.val1, cmd.val2, cmd.val3, 0.0f);
}

void RendererD3D11::AppendBillboardQuad(std::vector<Vertex>& vertices, const protocol::DrawCommand& cmd, const RaycasterState& state) {
    if (!state.valid) {
        return;
    }

    const float viewportX1 = static_cast<float>(cmd.x1);
    const float viewportY1 = static_cast<float>(cmd.y1);
    const float viewportX2 = static_cast<float>(cmd.x2);
    const float viewportY2 = static_cast<float>(cmd.y2);
    const float viewportW = viewportX2 - viewportX1;
    const float viewportH = viewportY2 - viewportY1;
    const float yCenter = viewportY1 + viewportH * 0.5f + 30.0f;

    const float dirX = std::cos(state.playerAngle);
    const float dirY = std::sin(state.playerAngle);
    const float planeX = -dirY * 0.66f;
    const float planeY = dirX * 0.66f;
    const float spriteX = cmd.val1 - state.playerX;
    const float spriteY = cmd.val2 - state.playerY;
    const float invDet = 1.0f / (planeX * dirY - dirX * planeY);
    const float transformX = invDet * (dirY * spriteX - dirX * spriteY);
    const float transformY = invDet * (-planeY * spriteX + planeX * spriteY);
    if (transformY <= 0.1f) {
        return;
    }

    const float spriteScreenX = ((viewportW * 0.5f) * (1.0f + transformX / transformY)) + viewportX1;

    float factor = 1.05f;
    switch (cmd.radius) {
    case -998: factor = 0.38f; break;
    case -997: factor = 0.72f; break;
    case -996: factor = 0.92f; break;
    case -995: factor = 0.32f; break;
    case -994: factor = 0.88f; break;
    case -993: factor = 0.58f; break;
    case -992: factor = 0.64f; break;
    case -991: factor = 1.05f; break;
    default: break;
    }

    const float spriteH = std::fabs(viewportH * factor / transformY);
    const float spriteW = spriteH;
    const float x1 = spriteScreenX - spriteW * 0.5f;
    const float y1 = yCenter - spriteH * 0.5f;
    const float x2 = spriteScreenX + spriteW * 0.5f;
    const float y2 = yCenter + spriteH * 0.5f;

    AppendQuad(vertices, x1, y1, x2, y2,
               state.playerX, state.playerY, state.playerX, state.playerY,
               cmd.r / 255.0f, cmd.g / 255.0f, cmd.b / 255.0f, 1.0f,
               viewportX1, viewportY1, viewportW, viewportH,
               0.0f, 0.0f, 0.0f, static_cast<float>(cmd.radius),
               0.0f, 0.0f, state.playerAngle, transformY);
}

void RendererD3D11::BuildGeometry(const protocol::RenderFrame& frame, std::vector<Vertex>& vertices, std::vector<DrawRange>& ranges) {
    const float scaleX = (frame.width > 0) ? (static_cast<float>(width_) / static_cast<float>(frame.width)) : 1.0f;
    const float scaleY = (frame.height > 0) ? (static_cast<float>(height_) / static_cast<float>(frame.height)) : 1.0f;
    const float scale = std::min(scaleX, scaleY);
    float currentGlow = 0.0f;
    float currentGlass = 0.0f;
    float shadowOx = 0.0f;
    float shadowOy = 0.0f;
    float shadowBlur = 0.0f;
    bool clipEnabled = false;
    D3D11_RECT clipRect{0, 0, width_, height_};
    float offsetX = 0.0f;
    float offsetY = 0.0f;
    RaycasterState raycasterState{};

    auto pushRange = [&](std::uint32_t start, std::uint32_t count, bool usesImage, int imageWidth, int imageHeight, const std::vector<std::uint8_t>& imageBytes) {
        DrawRange range;
        range.start = start;
        range.count = count;
        range.scissor = clipEnabled ? clipRect : D3D11_RECT{0, 0, width_, height_};
        range.usesImage = usesImage;
        range.imageWidth = imageWidth;
        range.imageHeight = imageHeight;
        if (usesImage) {
            range.imageBytes = imageBytes;
        }
        ranges.push_back(std::move(range));
    };

    for (const auto& cmd : frame.commands) {
        switch (cmd.type) {
        case protocol::DrawCommandType::SetGlow:
            currentGlow = cmd.val1 * scale;
            continue;
        case protocol::DrawCommandType::SetGlass:
            currentGlass = cmd.flag ? 1.0f : 0.0f;
            continue;
        case protocol::DrawCommandType::SetShadow:
            shadowOx = cmd.val1 * scaleX;
            shadowOy = cmd.val2 * scaleY;
            shadowBlur = cmd.val3 * scale;
            continue;
        case protocol::DrawCommandType::SetOffset:
            offsetX = cmd.val1 * scaleX;
            offsetY = cmd.val2 * scaleY;
            continue;
        case protocol::DrawCommandType::SetClip:
            clipEnabled = cmd.flag;
            if (clipEnabled) {
                clipRect.left = static_cast<LONG>(cmd.x1 * scaleX);
                clipRect.top = static_cast<LONG>(cmd.y1 * scaleY);
                clipRect.right = static_cast<LONG>((cmd.x1 + cmd.w) * scaleX);
                clipRect.bottom = static_cast<LONG>((cmd.y1 + cmd.h) * scaleY);
            }
            continue;
        default:
            break;
        }

        const auto start = static_cast<std::uint32_t>(vertices.size());
        bool mapFallback = false;
		bool retainedMap = false;
        const float cr = cmd.r / 255.0f;
        const float cg = cmd.g / 255.0f;
        const float cb = cmd.b / 255.0f;
        const float ca = cmd.a / 255.0f;
        const float x1 = cmd.x1 * scaleX + offsetX;
        const float y1 = cmd.y1 * scaleY + offsetY;
        const float x2 = cmd.x2 * scaleX + offsetX;
        const float y2 = cmd.y2 * scaleY + offsetY;

        if ((shadowOx != 0.0f || shadowOy != 0.0f) && cmd.type != protocol::DrawCommandType::DrawText && cmd.radius >= 0) {
            AppendRect(vertices, x1 + shadowOx, y1 + shadowOy, x2 + shadowOx, y2 + shadowOy,
                       0.0f, 0.0f, 0.0f, ca * 0.25f,
                       0.0f, 0.0f, 0.0f, static_cast<float>(cmd.radius) * scale,
                       0.0f, 0.0f, std::max(1.0f, shadowBlur), 0.0f);
        }
        if (currentGlow > 0.0f && cmd.type != protocol::DrawCommandType::DrawText && cmd.radius >= 0) {
            AppendRect(vertices, x1 - currentGlow, y1 - currentGlow, x2 + currentGlow, y2 + currentGlow,
                       cr, cg, cb, ca * 0.15f,
                       0.0f, currentGlow, 0.0f, static_cast<float>(cmd.radius) * scale + currentGlow,
                       0.0f, 0.0f, 0.0f, 0.0f);
        }

        switch (cmd.type) {
        case protocol::DrawCommandType::DrawRoundedRect:
        case protocol::DrawCommandType::FillRect:
            if (cmd.radius == -999) {
                AppendRaycasterQuad(vertices, cmd, x1, y1, x2, y2, raycasterState);
            } else if (cmd.radius <= -991 && cmd.radius >= -998) {
                AppendBillboardQuad(vertices, cmd, raycasterState);
            } else {
                const float radius = (cmd.type == protocol::DrawCommandType::FillRect) ? 0.0f : static_cast<float>(std::max(0, cmd.radius)) * scale;
                AppendRect(vertices, x1, y1, x2, y2, cr, cg, cb, ca,
                           0.0f, currentGlow, currentGlass, radius,
                           shadowOx, shadowOy, shadowBlur, 0.0f);
            }
            break;
        case protocol::DrawCommandType::DrawMapScene:
			if (const auto scene = mapScenes_.find(cmd.text); scene != mapScenes_.end() && !scene->second.draws.empty()) {
				retainedMap = true;
			} else if (cmd.w > 0 && cmd.h > 0 && !cmd.bytes.empty()) {
                AppendImageQuad(vertices, x1, y1, x2, y2);
                mapFallback = true;
            }
            break;
        case protocol::DrawCommandType::DrawImage:
            AppendImageQuad(vertices, x1, y1, x2, y2);
            break;
        case protocol::DrawCommandType::DrawLine:
            AppendLineQuad(vertices, x1, y1, x2, y2, 1.5f * scale, cr, cg, cb, ca);
            break;
        case protocol::DrawCommandType::DrawText: {
            float penX = x1;
            const float baselineY = y1;
            const float textScale = cmd.val1 > 0.0f ? cmd.val1 : 1.0f;
            for (const auto codepoint : DecodeUtf8(cmd.text)) {
                auto it = glyphs_.find(codepoint);
                if (it == glyphs_.end()) {
                    it = glyphs_.find(static_cast<std::uint32_t>('?'));
                    if (it == glyphs_.end()) {
                        penX += 8.0f * scaleX * textScale;
                        continue;
                    }
                }
                const auto& glyph = it->second;
                const float gx1 = penX;
                const float glyphWidth = glyph.width * scaleX * textScale;
                const float glyphHeight = glyph.height * scaleY * textScale;
                const float glyphAdvance = glyph.advance * scaleX * textScale;
                const float gy1 = baselineY - glyphHeight * 0.78f;
                const float gx2 = gx1 + glyphWidth;
                const float gy2 = gy1 + glyphHeight;
                AppendTextGlyph(vertices, gx1, gy1, gx2, gy2, glyph.u1, glyph.v1, glyph.u2, glyph.v2, cr, cg, cb, ca);
                penX += glyphAdvance;
            }
            break;
        }
        default:
            break;
        }

        const auto end = static_cast<std::uint32_t>(vertices.size());
		if (retainedMap) {
			DrawRange range;
			range.scissor = clipEnabled ? clipRect : D3D11_RECT{0, 0, width_, height_};
			range.mapViewportID = cmd.text;
			range.mapLeft = x1; range.mapTop = y1; range.mapRight = x2; range.mapBottom = y2;
			range.mapScale = cmd.val1 > 0 ? cmd.val1 : 1;
			ranges.push_back(std::move(range));
		} else if (end > start) {
            pushRange(start, end - start, cmd.type == protocol::DrawCommandType::DrawImage || mapFallback, cmd.w, cmd.h, cmd.bytes);
        }
    }
}

void RendererD3D11::Render(const protocol::RenderFrame& frame) {
    if (!device_ || !context_ || !swapchain_ || !rtv_) return;

    std::vector<Vertex> vertices;
    std::vector<DrawRange> ranges;
    BuildGeometry(frame, vertices, ranges);
    EnsureVertexCapacity(vertices.size());

    D3D11_MAPPED_SUBRESOURCE mapped{};
    if (SUCCEEDED(context_->Map(vertexBuffer_.Get(), 0, D3D11_MAP_WRITE_DISCARD, 0, &mapped))) {
        if (!vertices.empty()) {
            std::memcpy(mapped.pData, vertices.data(), vertices.size() * sizeof(Vertex));
        }
        context_->Unmap(vertexBuffer_.Get(), 0);
    }

    if (SUCCEEDED(context_->Map(constantBuffer_.Get(), 0, D3D11_MAP_WRITE_DISCARD, 0, &mapped))) {
        ConstantBuffer cb{static_cast<float>(width_), static_cast<float>(height_), {0, 0}};
        std::memcpy(mapped.pData, &cb, sizeof(cb));
        context_->Unmap(constantBuffer_.Get(), 0);
    }

    const float clear[4] = {0.04f, 0.05f, 0.07f, 1.0f};
    context_->OMSetRenderTargets(1, rtv_.GetAddressOf(), depthView_.Get());
    context_->ClearRenderTargetView(rtv_.Get(), clear);
    if (depthView_) {
        // The map depth convention is "bigger = closer" (GREATER_EQUAL), so
        // far is 0. UI draws with depth disabled and is never occluded.
        context_->ClearDepthStencilView(depthView_.Get(), D3D11_CLEAR_DEPTH | D3D11_CLEAR_STENCIL, 0.0f, 0);
    }
    context_->OMSetDepthStencilState(uiDepthState_.Get(), 0);

    D3D11_VIEWPORT vp{};
    vp.Width = static_cast<float>(width_);
    vp.Height = static_cast<float>(height_);
    vp.MinDepth = 0.0f;
    vp.MaxDepth = 1.0f;
    context_->RSSetViewports(1, &vp);
    context_->RSSetState(rasterizer_.Get());

    UINT stride = sizeof(Vertex);
    UINT offset = 0;
    ID3D11Buffer* vb = vertexBuffer_.Get();
    context_->IASetVertexBuffers(0, 1, &vb, &stride, &offset);
    context_->IASetInputLayout(inputLayout_.Get());
    context_->IASetPrimitiveTopology(D3D11_PRIMITIVE_TOPOLOGY_TRIANGLELIST);
    context_->VSSetShader(vertexShader_.Get(), nullptr, 0);
    context_->VSSetConstantBuffers(0, 1, constantBuffer_.GetAddressOf());
    context_->PSSetShader(pixelShader_.Get(), nullptr, 0);
    context_->PSSetConstantBuffers(0, 1, constantBuffer_.GetAddressOf());
    ID3D11ShaderResourceView* srvs[3] = {atlasSrv_.Get(), mapSrv_.Get(), imageSrv_.Get()};
    context_->PSSetShaderResources(0, 3, srvs);
    context_->PSSetSamplers(0, 1, sampler_.GetAddressOf());
    const float blendFactor[4] = {0, 0, 0, 0};
    context_->OMSetBlendState(blendState_.Get(), blendFactor, 0xFFFFFFFF);

    for (std::size_t i = 0; i < ranges.size(); ++i) {
        auto& range = ranges[i];
		if (!range.mapViewportID.empty()) {
			const auto sceneIt = mapScenes_.find(range.mapViewportID);
			if (sceneIt == mapScenes_.end()) continue;
			auto& scene = sceneIt->second;
			context_->RSSetScissorRects(1, &range.scissor);
			// Ground and extrusions: GPU-projected from the retained buffers
			// with depth testing. Restores the UI pipeline before returning.
			DrawGpuMapBatches(scene, range);
			// Dots, lines and label quads: CPU-composited overlay on top.
			if (EnsureMapGeometryBuffer(range.mapViewportID, range.mapLeft, range.mapTop, range.mapRight, range.mapBottom, range.mapScale) && scene.vertexCount > 0) {
				ID3D11Buffer* mapVB = scene.vertexBuffer.Get();
				context_->IASetVertexBuffers(0, 1, &mapVB, &stride, &offset);
				for (const auto& geometryRange : scene.geometryRanges) {
					ID3D11ShaderResourceView* texture = atlasSrv_.Get();
					if (!geometryRange.textureHash.empty()) {
						if (!EnsureMapTexture(scene, geometryRange.textureHash)) continue;
						texture = scene.textures.at(geometryRange.textureHash).Get();
					}
					ID3D11ShaderResourceView* mapRangeSrvs[3] = {texture, mapSrv_.Get(), nullptr};
					context_->PSSetShaderResources(0, 3, mapRangeSrvs);
					context_->Draw(geometryRange.count, geometryRange.start);
				}
			}
			// The GPU path bound its own buffers; restore the UI stream either way.
			context_->IASetVertexBuffers(0, 1, &vb, &stride, &offset);
			continue;
		}
        if (range.usesImage) {
            if (!EnsureImageTexture(range.imageWidth, range.imageHeight, range.imageBytes)) {
                continue;
            }
        }
        ID3D11ShaderResourceView* rangeSrvs[3] = {atlasSrv_.Get(), mapSrv_.Get(), range.usesImage ? imageSrv_.Get() : nullptr};
        context_->PSSetShaderResources(0, 3, rangeSrvs);
        context_->RSSetScissorRects(1, &range.scissor);
        context_->Draw(range.count, range.start);
    }

    const char* vsyncEnv = std::getenv("POEM_VSYNC");
    const UINT syncInterval = (vsyncEnv != nullptr && std::strcmp(vsyncEnv, "1") == 0) ? 1 : 0;
    swapchain_->Present(syncInterval, 0);
}

} // namespace poem
