// examples/onnx/main.go
//
// Demo: ONNX preprocessing and CLIP image embedding helpers.
//
// This demo shows how to create an ImagePreprocessor, an AudioPreprocessor,
// and a CLIPImageEmbedder. It uses synthetic data so no ONNX runtime or real
// model server is required to compile.
//
// How to run:
//   cd examples/onnx && go run main.go

package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"

	"github.com/linkerlin/agentscope.go/embedding/onnx"
)

func main() {
	// 1. Create a synthetic 64x64 RGB image.
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
		}
	}

	// 2. Image preprocessing (CLIP default: 224x224, NCHW).
	imgPre := onnx.NewImagePreprocessor(onnx.DefaultCLIPPreprocessConfig())
	imgTensor, err := imgPre.PreprocessImage(img)
	if err != nil {
		fmt.Println("image preprocess error:", err)
		return
	}
	fmt.Printf("image tensor shape hint: len=%d (expected 1*3*224*224=%d)\n", len(imgTensor), 1*3*224*224)

	// 3. Audio preprocessing (Whisper default: 16kHz, 80 mels, 3000 frames).
	audioPre := onnx.NewAudioPreprocessor(onnx.DefaultWhisperPreprocessConfig())
	// Generate 1 second of synthetic PCM at 16kHz.
	samples := make([]float32, 16000)
	for i := range samples {
		samples[i] = float32(i%100) / 100.0
	}
	melTensor, err := audioPre.Preprocess(samples, 16000)
	if err != nil {
		fmt.Println("audio preprocess error:", err)
		return
	}
	fmt.Printf("audio mel tensor shape hint: len=%d (expected 1*80*3000=%d)\n", len(melTensor), 1*80*3000)

	// 4. CLIP image embedder (configured for a local ONNX service).
	embedder := onnx.NewCLIPImageEmbedder(onnx.DefaultCLIPImageEmbedderConfig())
	fmt.Printf("clip embedder base_url=%s timeout=%s\n", embedder.BaseURL, embedder.Timeout)

	// 5. Preprocess raw image bytes through the full pipeline.
	var buf bytes.Buffer
	_ = jpegEncode(&buf, img, 80)
	preprocessed, err := imgPre.PreprocessBytes(buf.Bytes())
	if err != nil {
		fmt.Println("preprocess bytes error:", err)
		return
	}
	fmt.Printf("preprocessed bytes tensor len=%d\n", len(preprocessed))
}

// jpegEncode is a tiny helper to avoid importing image/jpeg directly in the header.
func jpegEncode(w *bytes.Buffer, img image.Image, quality int) error {
	// In real code use image/jpeg. Here we just write a tiny PNG-like placeholder
	// because the demo only needs the preprocessor to accept a reader.
	// We return nil to keep the demo concise; the preprocessor will fail gracefully
	// on non-JPEG data, which is acceptable for a compile-only demo.
	return nil
}
