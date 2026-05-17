//go:build gpu

package backend

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"unsafe"

	"github.com/go-gl/gl/v4.1-core/gl"
	"go_native_gpu_gui/internal/win32"
	"go_native_gpu_gui/pkg/render/types"
)

const vertexShaderSource = `
#version 410 core
layout (location = 0) in vec2 aPos;
layout (location = 1) in vec2 aUV;
layout (location = 2) in vec4 aColor;
layout (location = 3) in vec4 aRectParams; // [x, y, w, h]
layout (location = 4) in float aRadius;
layout (location = 5) in float aDrawType; // 0=Shape, 1=Text
layout (location = 6) in float aGlow;
layout (location = 7) in float aIsGlass;
layout (location = 8) in vec2 aShadowOffset;
layout (location = 9) in float aShadowSoftness;

out vec2 FragPos;
out vec2 FragUV;
out vec4 FragColor;
out vec4 RectParams;
out float Radius;
out float DrawType;
out float Glow;
out float IsGlass;
out vec2 ShadowOffset;
out float ShadowSoftness;

uniform mat4 projection;

void main() {
    gl_Position = projection * vec4(aPos, 0.0, 1.0);
    FragPos = aPos;
    FragUV = aUV;
    FragColor = aColor;
    RectParams = aRectParams;
    Radius = aRadius;
    DrawType = aDrawType;
    Glow = aGlow;
    IsGlass = aIsGlass;
    ShadowOffset = aShadowOffset;
    ShadowSoftness = aShadowSoftness;
}
` + "\x00"

const fragmentShaderSource = `
#version 410 core
in vec2 FragPos;
in vec2 FragUV;
in vec4 FragColor;
in vec4 RectParams;
in float Radius;
in float DrawType;
in float Glow;
in float IsGlass;
in vec2 ShadowOffset;
in float ShadowSoftness;

out vec4 color;

uniform sampler2D textAtlas;
uniform sampler2D blurredBg;
uniform vec2 screenSize;

float sdRoundedRect(vec2 p, vec2 b, float r) {
    vec2 q = abs(p) - b + r;
    return length(max(q, 0.0)) + min(max(q.x, q.y), 0.0) - r;
}

void main() {
    if (DrawType < 0.5) { // SHAPE
        vec2 b = RectParams.zw * 0.5;
        vec2 center = RectParams.xy + b;
        vec2 p = FragPos - center;
        float d = sdRoundedRect(p, b, Radius);
        
        // Shadow Calculation
        vec2 shadowP = FragPos - (center + ShadowOffset);
        float shadowD = sdRoundedRect(shadowP, b, Radius);
        float shadowAlpha = 1.0 - smoothstep(-ShadowSoftness, ShadowSoftness, shadowD);
        shadowAlpha *= 0.6; // Shadow intensity

        float shapeAlpha = 1.0 - smoothstep(-1.0, 1.0, d);
        float glowAlpha = exp(-max(0.0, d) * (10.0 - Glow)) * (Glow / 10.0);
        float finalAlpha = max(shapeAlpha, glowAlpha);

        vec3 baseColor = FragColor.rgb;
        if (IsGlass > 0.5) {
            vec2 screenUV = gl_FragCoord.xy / screenSize;
            vec3 blurred = texture(blurredBg, screenUV).rgb;
            baseColor = mix(blurred, baseColor, FragColor.a);
            color = mix(vec4(0.0, 0.0, 0.0, shadowAlpha), vec4(baseColor, 1.0), shapeAlpha);
        } else {
            vec4 shadowCol = vec4(0.0, 0.0, 0.0, shadowAlpha * 0.8);
            vec4 shapeCol = vec4(baseColor, FragColor.a * finalAlpha);
            color = mix(shadowCol, shapeCol, shapeAlpha);
        }
    } else { // TEXT
        float alpha = texture(textAtlas, FragUV).r;
        color = vec4(FragColor.rgb, FragColor.a * alpha);
    }
}
` + "\x00"

const blurShaderSource = `
#version 410 core
out vec4 FragColor;
in vec2 TexCoords;
uniform sampler2D image;
uniform bool horizontal;
uniform float weight[5] = float[] (0.227027, 0.1945946, 0.1216216, 0.054054, 0.016216);
void main() {
    vec2 tex_offset = 1.0 / textureSize(image, 0);
    vec3 result = texture(image, TexCoords).rgb * weight[0];
    if(horizontal) {
        for(int i = 1; i < 5; ++i) {
            result += texture(image, TexCoords + vec2(tex_offset.x * i, 0.0)).rgb * weight[i];
            result += texture(image, TexCoords - vec2(tex_offset.x * i, 0.0)).rgb * weight[i];
        }
    } else {
        for(int i = 1; i < 5; ++i) {
            result += texture(image, TexCoords + vec2(0.0, tex_offset.y * i)).rgb * weight[i];
            result += texture(image, TexCoords - vec2(0.0, tex_offset.y * i)).rgb * weight[i];
        }
    }
    FragColor = vec4(result, 1.0);
}
` + "\x00"

const screenQuadShaderSource = `
#version 410 core
layout (location = 0) in vec2 aPos;
layout (location = 1) in vec2 aTexCoords;
out vec2 TexCoords;
void main() {
    gl_Position = vec4(aPos.x, aPos.y, 0.0, 1.0);
    TexCoords = aTexCoords;
}
` + "\x00"

type vertex struct {
	Pos            [2]float32
	UV             [2]float32
	Color          [4]float32
	Params         [4]float32 // x, y, w, h
	Radius         float32
	DrawType       float32 // 0: Shape, 1: Text
	Glow           float32
	IsGlass        float32
	ShadowOffset   [2]float32
	ShadowSoftness float32
	Padding        float32
}

type GPUEngine struct {
	program     uint32
	blurProgram uint32
	vao         uint32
	vbo         uint32
	atlas       *FontAtlas
	projection  [16]float32

	// Glassmorphism Buffers
	fbo         uint32
	fboTex      uint32
	pingpongFbo [2]uint32
	pingpongTex [2]uint32
	quadVao     uint32

	batch           []vertex
	currentDrawType float32
	currentGlow     float32
	currentIsGlass  float32
	batchIsGlass    float32
	batchDrawType   float32

	// Animation offsets
	offsetX float32
	offsetY float32

	// Shadow state
	currentShadowOffset [2]float32
	currentShadowBlur   float32
}

func New(hdc uintptr) (types.UIRenderer, error) {
	fmt.Println("🚀 Factory: Spawning Hardware-Accelerated GPU Engine")

	pfd := win32.PIXELFORMATDESCRIPTOR{Flags: 4 | 32, PixelType: 0, ColorBits: 32, DepthBits: 24}
	pfd.Size = uint16(unsafe.Sizeof(pfd))
	pfd.Version = 1

	pf, err := win32.ChoosePixelFormat(hdc, &pfd)
	if err != nil {
		return nil, fmt.Errorf("failed to choose pixel format: %v", err)
	}

	err = win32.SetPixelFormat(hdc, pf, &pfd)
	if err != nil {
		return nil, fmt.Errorf("failed to set pixel format: %v", err)
	}

	hglrc, err := win32.WglCreateContext(hdc)
	if err != nil {
		return nil, fmt.Errorf("failed to create wgl context: %v", err)
	}

	err = win32.WglMakeCurrent(hdc, hglrc)
	if err != nil {
		return nil, fmt.Errorf("failed to make wgl context current: %v", err)
	}

	return &GPUEngine{}, nil
}

func (g *GPUEngine) Setup(hdc uintptr) error {
	if err := gl.Init(); err != nil {
		return err
	}
	fmt.Printf("Successfully initialized: Hardware GPU Engine (%s)\n", gl.GoStr(gl.GetString(gl.RENDERER)))

	var err error
	g.program, err = newProgram(vertexShaderSource, fragmentShaderSource)
	if err != nil {
		return err
	}

	g.atlas = NewFontAtlas()

	gl.GenVertexArrays(1, &g.vao)
	gl.GenBuffers(1, &g.vbo)

	gl.BindVertexArray(g.vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, g.vbo)

	stride := int32(unsafe.Sizeof(vertex{}))
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointer(0, 2, gl.FLOAT, false, stride, gl.PtrOffset(0))
	gl.EnableVertexAttribArray(1)
	gl.VertexAttribPointer(1, 2, gl.FLOAT, false, stride, gl.PtrOffset(8))
	gl.EnableVertexAttribArray(2)
	gl.VertexAttribPointer(2, 4, gl.FLOAT, false, stride, gl.PtrOffset(16))
	gl.EnableVertexAttribArray(3)
	gl.VertexAttribPointer(3, 4, gl.FLOAT, false, stride, gl.PtrOffset(32))
	gl.EnableVertexAttribArray(4)
	gl.VertexAttribPointer(4, 1, gl.FLOAT, false, stride, gl.PtrOffset(48))
	gl.EnableVertexAttribArray(5)
	gl.VertexAttribPointer(5, 1, gl.FLOAT, false, stride, gl.PtrOffset(52))
	gl.EnableVertexAttribArray(6)
	gl.VertexAttribPointer(6, 1, gl.FLOAT, false, stride, gl.PtrOffset(56))
	gl.EnableVertexAttribArray(7)
	gl.VertexAttribPointer(7, 1, gl.FLOAT, false, stride, gl.PtrOffset(60))
	gl.EnableVertexAttribArray(8)
	gl.VertexAttribPointer(8, 2, gl.FLOAT, false, stride, gl.PtrOffset(64))
	gl.EnableVertexAttribArray(9)
	gl.VertexAttribPointer(9, 1, gl.FLOAT, false, stride, gl.PtrOffset(72))

	g.blurProgram, err = newProgram(screenQuadShaderSource, blurShaderSource)
	if err != nil {
		return err
	}

	g.setupFbos()
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

	g.SetSize(types.Width, types.Height)
	return nil
}

func (g *GPUEngine) SetGlass(enabled bool) {
	if enabled {
		g.currentIsGlass = 1.0
	} else {
		g.currentIsGlass = 0.0
	}
}

func (g *GPUEngine) setupFbos() {
	// Create main FBO to capture the background
	gl.GenFramebuffers(1, &g.fbo)
	gl.BindFramebuffer(gl.FRAMEBUFFER, g.fbo)

	gl.GenTextures(1, &g.fboTex)
	gl.BindTexture(gl.TEXTURE_2D, g.fboTex)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGB, int32(types.Width), int32(types.Height), 0, gl.RGB, gl.UNSIGNED_BYTE, nil)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, g.fboTex, 0)

	// Create Ping-Pong FBOs for Gaussian Blur
	gl.GenFramebuffers(2, &g.pingpongFbo[0])
	gl.GenTextures(2, &g.pingpongTex[0])
	for i := 0; i < 2; i++ {
		gl.BindFramebuffer(gl.FRAMEBUFFER, g.pingpongFbo[i])
		gl.BindTexture(gl.TEXTURE_2D, g.pingpongTex[i])
		gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGB, int32(types.Width), int32(types.Height), 0, gl.RGB, gl.UNSIGNED_BYTE, nil)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
		gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, g.pingpongTex[i], 0)
	}

	// Full-screen quad for blur
	quadVertices := []float32{
		-1.0, 1.0, 0.0, 1.0,
		-1.0, -1.0, 0.0, 0.0,
		1.0, 1.0, 1.0, 1.0,
		1.0, -1.0, 1.0, 0.0,
	}
	var quadVbo uint32
	gl.GenVertexArrays(1, &g.quadVao)
	gl.GenBuffers(1, &quadVbo)
	gl.BindVertexArray(g.quadVao)
	gl.BindBuffer(gl.ARRAY_BUFFER, quadVbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(quadVertices)*4, gl.Ptr(quadVertices), gl.STATIC_DRAW)
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointer(0, 2, gl.FLOAT, false, 16, gl.PtrOffset(0))
	gl.EnableVertexAttribArray(1)
	gl.VertexAttribPointer(1, 2, gl.FLOAT, false, 16, gl.PtrOffset(8))

	gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
}

func (g *GPUEngine) SetSize(w, h int) {
	gl.Viewport(0, 0, int32(w), int32(h))

	// Orthographic projection matrix
	left, right := float32(0), float32(w)
	bottom, top := float32(h), float32(0) // Top-down coordinate system
	near, far := float32(-1), float32(1)

	g.projection = [16]float32{
		2 / (right - left), 0, 0, 0,
		0, 2 / (top - bottom), 0, 0,
		0, 0, -2 / (far - near), 0,
		-(right + left) / (right - left), -(top + bottom) / (top - bottom), -(far + near) / (far - near), 1,
	}
}

func (g *GPUEngine) DrawRoundedRect(r image.Rectangle, radius int, col color.RGBA) {
	g.drawQuad(r, float32(radius), col, 0, [4]float32{0, 0, 0, 0})
}

func (g *GPUEngine) FillRect(r image.Rectangle, col color.RGBA) {
	g.drawQuad(r, 0, col, 0, [4]float32{0, 0, 0, 0})
}

func (g *GPUEngine) DrawLine(x1, y1, x2, y2 int, col color.RGBA) {
	// Optimize horizontal lines
	if y1 == y2 {
		minX, maxX := x1, x2
		if minX > maxX {
			minX, maxX = maxX, minX
		}
		g.FillRect(image.Rect(minX, y1, maxX+1, y1+1), col)
		return
	}

	// Optimize vertical lines
	if x1 == x2 {
		minY, maxY := y1, y2
		if minY > maxY {
			minY, maxY = maxY, minY
		}
		g.FillRect(image.Rect(x1, minY, x1+1, maxY+1), col)
		return
	}

	// Draw diagonal lines using Bresenham's Line Algorithm
	dx := int(math.Abs(float64(x2 - x1)))
	dy := int(math.Abs(float64(y2 - y1)))
	sx, sy := 1, 1
	if x1 >= x2 {
		sx = -1
	}
	if y1 >= y2 {
		sy = -1
	}
	err := dx - dy

	for {
		// Draw 1px pixel block
		g.FillRect(image.Rect(x1, y1, x1+1, y1+1), col)

		if x1 == x2 && y1 == y2 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x1 += sx
		}
		if e2 < dx {
			err += dx
			y1 += sy
		}
	}
}

func (g *GPUEngine) DrawText(s string, x, y int, col color.RGBA) {
	currX := x
	for _, r := range s {
		char, ok := g.atlas.CharMap[r]
		if !ok {
			continue
		}

		rect := image.Rect(currX, y, currX+char.Width, y+char.Height)
		g.drawQuad(rect, 0, col, 1, [4]float32{char.U1, char.V1, char.U2, char.V2})
		currX += char.Advance
	}
}

func (g *GPUEngine) SetGlow(strength float32) {
	g.currentGlow = strength
}

func (g *GPUEngine) SetShadow(ox, oy, blur float32) {
	g.currentShadowOffset = [2]float32{ox, oy}
	g.currentShadowBlur = blur
}

func (g *GPUEngine) SetOffset(x, y float32) {
	g.offsetX = x
	g.offsetY = y
}

func (g *GPUEngine) SetClip(r image.Rectangle) {
	g.Flush()
	if r.Empty() {
		gl.Disable(gl.SCISSOR_TEST)
	} else {
		gl.Enable(gl.SCISSOR_TEST)
		// OpenGL is bottom-up; translate top-down bounds
		gl.Scissor(
			int32(r.Min.X),
			int32(types.Height-r.Max.Y),
			int32(r.Dx()),
			int32(r.Dy()),
		)
	}
}

func (g *GPUEngine) Flush() {
	if len(g.batch) == 0 {
		return
	}

	gl.UseProgram(g.program)

	projLoc := gl.GetUniformLocation(g.program, gl.Str("projection\x00"))
	gl.UniformMatrix4fv(projLoc, 1, false, &g.projection[0])

	gl.Uniform2f(gl.GetUniformLocation(g.program, gl.Str("screenSize\x00")), float32(types.Width), float32(types.Height))

	if g.batchDrawType > 0.5 {
		gl.ActiveTexture(gl.TEXTURE0)
		gl.BindTexture(gl.TEXTURE_2D, g.atlas.TextureID)
		gl.Uniform1i(gl.GetUniformLocation(g.program, gl.Str("textAtlas\x00")), 0)
	} else if g.batchIsGlass > 0.5 {
		gl.ActiveTexture(gl.TEXTURE1)
		gl.BindTexture(gl.TEXTURE_2D, g.pingpongTex[1]) // Final blurred texture
		gl.Uniform1i(gl.GetUniformLocation(g.program, gl.Str("blurredBg\x00")), 1)
	}

	gl.BindVertexArray(g.vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, g.vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(g.batch)*int(unsafe.Sizeof(vertex{})), gl.Ptr(g.batch), gl.STREAM_DRAW)

	gl.DrawArrays(gl.TRIANGLES, 0, int32(len(g.batch)))
	g.batch = g.batch[:0]
}

func (g *GPUEngine) drawQuad(r image.Rectangle, radius float32, col color.RGBA, drawType float32, uv [4]float32) {
	if (drawType != g.batchDrawType || g.currentIsGlass != g.batchIsGlass) && len(g.batch) > 0 {
		g.Flush()
	}
	g.batchDrawType = drawType
	g.batchIsGlass = g.currentIsGlass

	c := [4]float32{float32(col.R) / 255, float32(col.G) / 255, float32(col.B) / 255, float32(col.A) / 255}
	params := [4]float32{float32(r.Min.X) + g.offsetX, float32(r.Min.Y) + g.offsetY, float32(r.Dx()), float32(r.Dy())}

	glow := g.currentGlow
	if drawType > 0.5 {
		glow = 0
	}

	isGlass := g.currentIsGlass

	shOff := g.currentShadowOffset
	shBlur := g.currentShadowBlur

	g.batch = append(g.batch,
		vertex{[2]float32{float32(r.Min.X) + g.offsetX, float32(r.Min.Y) + g.offsetY}, [2]float32{uv[0], uv[1]}, c, params, radius, drawType, glow, isGlass, shOff, shBlur, 0},
		vertex{[2]float32{float32(r.Max.X) + g.offsetX, float32(r.Min.Y) + g.offsetY}, [2]float32{uv[2], uv[1]}, c, params, radius, drawType, glow, isGlass, shOff, shBlur, 0},
		vertex{[2]float32{float32(r.Max.X) + g.offsetX, float32(r.Max.Y) + g.offsetY}, [2]float32{uv[2], uv[3]}, c, params, radius, drawType, glow, isGlass, shOff, shBlur, 0},

		vertex{[2]float32{float32(r.Min.X) + g.offsetX, float32(r.Min.Y) + g.offsetY}, [2]float32{uv[0], uv[1]}, c, params, radius, drawType, glow, isGlass, shOff, shBlur, 0},
		vertex{[2]float32{float32(r.Max.X) + g.offsetX, float32(r.Max.Y) + g.offsetY}, [2]float32{uv[2], uv[3]}, c, params, radius, drawType, glow, isGlass, shOff, shBlur, 0},
		vertex{[2]float32{float32(r.Min.X) + g.offsetX, float32(r.Max.Y) + g.offsetY}, [2]float32{uv[0], uv[3]}, c, params, radius, drawType, glow, isGlass, shOff, shBlur, 0},
	)
}

func (g *GPUEngine) Paint(hdc uintptr, state *types.ApplicationState) {
	// Pass 1: Draw particles to FBO
	gl.BindFramebuffer(gl.FRAMEBUFFER, g.fbo)
	gl.ClearColor(0.05, 0.05, 0.08, 1.0)
	gl.Clear(gl.COLOR_BUFFER_BIT)

	if state.Particles != nil {
		state.Particles.Draw(g, state)
	}
	g.Flush()

	// Pass 2: Blur the FBO texture
	horizontal := true
	firstIteration := true
	gl.UseProgram(g.blurProgram)
	for i := 0; i < 4; i++ { // 2 iterations (horizontal + vertical)
		gl.BindFramebuffer(gl.FRAMEBUFFER, g.pingpongFbo[b2i(!horizontal)])
		gl.Uniform1i(gl.GetUniformLocation(g.blurProgram, gl.Str("horizontal\x00")), int32(b2i(horizontal)))

		var tex uint32
		if firstIteration {
			tex = g.fboTex
			firstIteration = false
		} else {
			tex = g.pingpongTex[b2i(horizontal)]
		}
		gl.BindTexture(gl.TEXTURE_2D, tex)

		gl.BindVertexArray(g.quadVao)
		gl.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)
		horizontal = !horizontal
	}

	// Pass 3: Draw the rest of the UI to screen
	gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
	gl.ClearColor(0.05, 0.05, 0.08, 1.0)
	gl.Clear(gl.COLOR_BUFFER_BIT)

	// Reset batch state for UI
	g.batchDrawType = -1
	g.batchIsGlass = -1
	g.currentGlow = 0
	g.currentIsGlass = 0

	// Use the shared render pipeline
	types.RenderPipeline(g, state)

	// Final flush
	g.Flush()

	win32.SwapBuffers(hdc)
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func newProgram(vSrc, fSrc string) (uint32, error) {
	vShader, err := compileShader(vSrc, gl.VERTEX_SHADER)
	if err != nil {
		return 0, err
	}
	fShader, err := compileShader(fSrc, gl.FRAGMENT_SHADER)
	if err != nil {
		return 0, err
	}

	program := gl.CreateProgram()
	gl.AttachShader(program, vShader)
	gl.AttachShader(program, fShader)
	gl.LinkProgram(program)

	var status int32
	gl.GetProgramiv(program, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetProgramiv(program, gl.INFO_LOG_LENGTH, &logLength)
		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetProgramInfoLog(program, logLength, nil, gl.Str(log))
		return 0, fmt.Errorf("failed to link program: %v", log)
	}

	gl.DeleteShader(vShader)
	gl.DeleteShader(fShader)

	return program, nil
}

func compileShader(source string, shaderType uint32) (uint32, error) {
	shader := gl.CreateShader(shaderType)
	csources, free := gl.Strs(source)
	gl.ShaderSource(shader, 1, csources, nil)
	free()
	gl.CompileShader(shader)

	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLength)
		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetShaderInfoLog(shader, logLength, nil, gl.Str(log))
		return 0, fmt.Errorf("failed to compile %v: %v", shaderType, log)
	}

	return shader, nil
}
