package onnx

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"math"

	_ "golang.org/x/image/webp"
)

// ImagePreprocessConfig 图像预处理配置
type ImagePreprocessConfig struct {
	Width       int        // 目标宽度（默认 224 for CLIP）
	Height      int        // 目标高度（默认 224 for CLIP）
	Mean        [3]float32 // RGB 均值（ImageNet: [0.48145466, 0.4578275, 0.40821073]）
	Std         [3]float32 // RGB 标准差（ImageNet: [0.26862954, 0.26130258, 0.27577711]）
	Normalize   bool       // 是否归一化
	ToTensor    bool       // 是否转换为 tensor 格式（NCHW）
	Interpolate string     // 插值方法：bilinear/nearest（默认 bilinear）
}

// DefaultCLIPPreprocessConfig 返回 CLIP 默认预处理配置
func DefaultCLIPPreprocessConfig() ImagePreprocessConfig {
	return ImagePreprocessConfig{
		Width:       224,
		Height:      224,
		Mean:        [3]float32{0.48145466, 0.4578275, 0.40821073},
		Std:         [3]float32{0.26862954, 0.26130258, 0.27577711},
		Normalize:   true,
		ToTensor:    true,
		Interpolate: "bilinear",
	}
}

// ImagePreprocessor 图像预处理器
type ImagePreprocessor struct {
	Config ImagePreprocessConfig
}

// NewImagePreprocessor 创建图像预处理器
func NewImagePreprocessor(config ImagePreprocessConfig) *ImagePreprocessor {
	if config.Width <= 0 {
		config.Width = 224
	}
	if config.Height <= 0 {
		config.Height = 224
	}
	return &ImagePreprocessor{Config: config}
}

// Preprocess 预处理图像（从 io.Reader）
// 返回 NCHW 格式的 float32 数组：[1, 3, H, W]
func (p *ImagePreprocessor) Preprocess(r io.Reader) ([]float32, error) {
	img, format, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("onnx: failed to decode image: %w", err)
	}
	_ = format // 记录格式但不需要特殊处理

	return p.PreprocessImage(img)
}

// PreprocessBytes 预处理图像（从字节数组）
func (p *ImagePreprocessor) PreprocessBytes(data []byte) ([]float32, error) {
	return p.Preprocess(bytes.NewReader(data))
}

// PreprocessImage 预处理已解码的图像
func (p *ImagePreprocessor) PreprocessImage(img image.Image) ([]float32, error) {
	if img == nil {
		return nil, fmt.Errorf("onnx: nil image")
	}

	// 1. 调整大小
	resized := p.resize(img, p.Config.Width, p.Config.Height)

	// 2. 转换为 RGB float32 数组
	pixels := p.toRGBFloat32(resized)

	// 3. 归一化
	if p.Config.Normalize {
		pixels = p.normalize(pixels)
	}

	// 4. 转换为 NCHW 格式
	if p.Config.ToTensor {
		pixels = p.toNCHW(pixels, p.Config.Width, p.Config.Height)
	}

	return pixels, nil
}

// resize 调整图像大小（双线性插值）
func (p *ImagePreprocessor) resize(img image.Image, width, height int) image.Image {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	if srcW == width && srcH == height {
		return img
	}

	// 创建目标图像
	dst := image.NewRGBA(image.Rect(0, 0, width, height))

	// 双线性插值
	xRatio := float64(srcW) / float64(width)
	yRatio := float64(srcH) / float64(height)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			srcX := float64(x) * xRatio
			srcY := float64(y) * yRatio

			// 采样
			c := p.sampleBilinear(img, srcX, srcY)
			dst.Set(x, y, c)
		}
	}

	return dst
}

// sampleBilinear 双线性采样
func (p *ImagePreprocessor) sampleBilinear(img image.Image, x, y float64) color.Color {
	bounds := img.Bounds()
	x0 := int(math.Floor(x))
	y0 := int(math.Floor(y))
	x1 := x0 + 1
	y1 := y0 + 1

	// 边界检查
	if x0 < bounds.Min.X {
		x0 = bounds.Min.X
	}
	if y0 < bounds.Min.Y {
		y0 = bounds.Min.Y
	}
	if x1 > bounds.Max.X-1 {
		x1 = bounds.Max.X - 1
	}
	if y1 > bounds.Max.Y-1 {
		y1 = bounds.Max.Y - 1
	}

	// 获取四个角的颜色
	c00 := img.At(x0, y0)
	c01 := img.At(x0, y1)
	c10 := img.At(x1, y0)
	c11 := img.At(x1, y1)

	// 插值
	dx := x - float64(x0)
	dy := y - float64(y0)

	r00, g00, b00, _ := c00.RGBA()
	r01, g01, b01, _ := c01.RGBA()
	r10, g10, b10, _ := c10.RGBA()
	r11, g11, b11, _ := c11.RGBA()

	// 归一化到 [0, 1]
	r00f, g00f, b00f := float64(r00)/65535.0, float64(g00)/65535.0, float64(b00)/65535.0
	r01f, g01f, b01f := float64(r01)/65535.0, float64(g01)/65535.0, float64(b01)/65535.0
	r10f, g10f, b10f := float64(r10)/65535.0, float64(g10)/65535.0, float64(b10)/65535.0
	r11f, g11f, b11f := float64(r11)/65535.0, float64(g11)/65535.0, float64(b11)/65535.0

	rf := r00f*(1-dx)*(1-dy) + r01f*(1-dx)*dy + r10f*dx*(1-dy) + r11f*dx*dy
	gf := g00f*(1-dx)*(1-dy) + g01f*(1-dx)*dy + g10f*dx*(1-dy) + g11f*dx*dy
	bf := b00f*(1-dx)*(1-dy) + b01f*(1-dx)*dy + b10f*dx*(1-dy) + b11f*dx*dy

	return color.NRGBA{
		R: uint8(rf * 255),
		G: uint8(gf * 255),
		B: uint8(bf * 255),
		A: 255,
	}
}

// toRGBFloat32 将图像转换为 RGB float32 数组（HWC 格式）
func (p *ImagePreprocessor) toRGBFloat32(img image.Image) []float32 {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	pixels := make([]float32, width*height*3)

	idx := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			// RGBA() 返回 16-bit 值，转换为 [0, 1]
			pixels[idx] = float32(r) / 65535.0
			pixels[idx+1] = float32(g) / 65535.0
			pixels[idx+2] = float32(b) / 65535.0
			idx += 3
		}
	}

	return pixels
}

// normalize 归一化（减去均值，除以标准差）
func (p *ImagePreprocessor) normalize(pixels []float32) []float32 {
	mean := p.Config.Mean
	std := p.Config.Std

	for i := 0; i < len(pixels); i += 3 {
		pixels[i] = (pixels[i] - mean[0]) / std[0]     // R
		pixels[i+1] = (pixels[i+1] - mean[1]) / std[1] // G
		pixels[i+2] = (pixels[i+2] - mean[2]) / std[2] // B
	}

	return pixels
}

// toNCHW 将 HWC 格式转换为 NCHW 格式
// 输入: [H, W, 3] 输出: [1, 3, H, W]
func (p *ImagePreprocessor) toNCHW(pixels []float32, width, height int) []float32 {
	nchw := make([]float32, 1*3*height*width)

	// 重新排列：NCHW[c, h, w] = HWC[h, w, c]
	for h := 0; h < height; h++ {
		for w := 0; w < width; w++ {
			for c := 0; c < 3; c++ {
				hwcIdx := (h*width+w)*3 + c
				nchwIdx := c*height*width + h*width + w
				nchw[nchwIdx] = pixels[hwcIdx]
			}
		}
	}

	return nchw
}

// GetImageInfo 获取图像信息（用于调试）
func GetImageInfo(img image.Image) map[string]int {
	bounds := img.Bounds()
	return map[string]int{
		"width":  bounds.Dx(),
		"height": bounds.Dy(),
	}
}

// DecodeImage 解码图像（支持 JPEG/PNG/WebP）
func DecodeImage(data []byte) (image.Image, string, error) {
	return image.Decode(bytes.NewReader(data))
}

// EncodeToJPEG 将图像编码为 JPEG（用于调试/缓存）
func EncodeToJPEG(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// EncodeToPNG 将图像编码为 PNG
func EncodeToPNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
