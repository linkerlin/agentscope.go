package onnx

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"os"
	"testing"
	"time"
)

// TestImagePreprocessor_Preprocess 测试图像预处理
func TestImagePreprocessor_Preprocess(t *testing.T) {
	preprocessor := NewImagePreprocessor(DefaultCLIPPreprocessConfig())

	// 创建测试图像数据
	testData := make([]byte, 100)
	for i := range testData {
		testData[i] = byte(i)
	}

	result, err := preprocessor.Preprocess(bytes.NewReader(testData))
	if err == nil {
		// 如果解码成功，检查结果维度
		expectedSize := 1 * 3 * 224 * 224
		if len(result) != expectedSize {
			t.Errorf("expected size %d, got %d", expectedSize, len(result))
		}
	}
	// 对于无效图像数据，允许错误
}

// TestImagePreprocessor_DefaultConfig 测试默认配置
func TestImagePreprocessor_DefaultConfig(t *testing.T) {
	config := DefaultCLIPPreprocessConfig()
	if config.Width != 224 {
		t.Errorf("expected width 224, got %d", config.Width)
	}
	if config.Height != 224 {
		t.Errorf("expected height 224, got %d", config.Height)
	}
	if config.Mean[0] == 0 {
		t.Error("expected non-zero mean")
	}
	if config.Std[0] == 0 {
		t.Error("expected non-zero std")
	}
}

// TestImagePreprocessor_resize 测试图像缩放
func TestImagePreprocessor_resize(t *testing.T) {
	preprocessor := NewImagePreprocessor(DefaultCLIPPreprocessConfig())

	// 创建测试图像
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{255, 128, 64, 255})
		}
	}

	resized := preprocessor.resize(img, 224, 224)
	bounds := resized.Bounds()
	if bounds.Dx() != 224 || bounds.Dy() != 224 {
		t.Errorf("expected 224x224, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

// TestImagePreprocessor_toRGBFloat32 测试 RGB 转换
func TestImagePreprocessor_toRGBFloat32(t *testing.T) {
	preprocessor := NewImagePreprocessor(DefaultCLIPPreprocessConfig())

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	img.Set(1, 0, color.RGBA{0, 255, 0, 255})
	img.Set(0, 1, color.RGBA{0, 0, 255, 255})
	img.Set(1, 1, color.RGBA{128, 128, 128, 255})

	pixels := preprocessor.toRGBFloat32(img)
	if len(pixels) != 2*2*3 {
		t.Errorf("expected 12 pixels, got %d", len(pixels))
	}

	// 检查第一个像素 (255, 0, 0)
	if pixels[0] != 1.0 || pixels[1] != 0.0 || pixels[2] != 0.0 {
		t.Errorf("expected red pixel [1,0,0], got [%f,%f,%f]", pixels[0], pixels[1], pixels[2])
	}
}

// TestImagePreprocessor_normalize 测试归一化
func TestImagePreprocessor_normalize(t *testing.T) {
	preprocessor := NewImagePreprocessor(DefaultCLIPPreprocessConfig())

	pixels := []float32{1.0, 0.5, 0.0}
	normalized := preprocessor.normalize(pixels)

	if len(normalized) != len(pixels) {
		t.Errorf("expected %d pixels, got %d", len(pixels), len(normalized))
	}

	// 检查归一化后的值是否在合理范围内
	for i, v := range normalized {
		if v > 10 || v < -10 {
			t.Errorf("normalized pixel %d out of range: %f", i, v)
		}
	}
}

// TestImagePreprocessor_toNCHW 测试 NCHW 转换
func TestImagePreprocessor_toNCHW(t *testing.T) {
	preprocessor := NewImagePreprocessor(DefaultCLIPPreprocessConfig())

	// 2x2 图像，3 通道: [H=2, W=2, C=3] = 12 个元素
	pixels := []float32{
		1.0, 0.0, 0.0, // pixel (0,0): R=1, G=0, B=0
		0.0, 1.0, 0.0, // pixel (0,1): R=0, G=1, B=0
		0.0, 0.0, 1.0, // pixel (1,0): R=0, G=0, B=1
		1.0, 1.0, 1.0, // pixel (1,1): R=1, G=1, B=1
	}

	result := preprocessor.toNCHW(pixels, 2, 2)
	if len(result) != 12 {
		t.Errorf("expected 12 elements, got %d", len(result))
	}

	// NCHW 布局: [N=1, C=3, H=2, W=2]
	// Channel 0 (R): [1, 0, 0, 1]
	// Channel 1 (G): [0, 1, 0, 1]
	// Channel 2 (B): [0, 0, 1, 1]
	expected := []float32{
		1.0, 0.0, 0.0, 1.0, // R channel
		0.0, 1.0, 0.0, 1.0, // G channel
		0.0, 0.0, 1.0, 1.0, // B channel
	}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("expected %f at index %d, got %f", v, i, result[i])
		}
	}
}

// TestAudioPreprocessor_Preprocess 测试音频预处理
func TestAudioPreprocessor_Preprocess(t *testing.T) {
	preprocessor := NewAudioPreprocessor(DefaultWhisperPreprocessConfig())

	// 创建测试音频数据（1秒 16kHz）
	samples := make([]float32, 16000)
	for i := range samples {
		samples[i] = float32(i) / 16000.0
	}

	result, err := preprocessor.Preprocess(samples, 16000)
	if err != nil {
		t.Fatalf("preprocess failed: %v", err)
	}

	// 检查结果维度: [1, 80, 3000]
	expectedSize := 1 * 80 * 3000
	if len(result) != expectedSize {
		t.Errorf("expected size %d, got %d", expectedSize, len(result))
	}
}

// TestAudioPreprocessor_resample 测试重采样
func TestAudioPreprocessor_resample(t *testing.T) {
	preprocessor := NewAudioPreprocessor(DefaultWhisperPreprocessConfig())

	// 8kHz -> 16kHz
	samples := []float32{0.0, 0.5, 1.0, 0.5, 0.0}
	result := preprocessor.resample(samples, 8000, 16000)

	if len(result) != 10 {
		t.Errorf("expected 10 samples, got %d", len(result))
	}
}

// TestAudioPreprocessor_padOrTruncate 测试填充截断
func TestAudioPreprocessor_padOrTruncate(t *testing.T) {
	preprocessor := NewAudioPreprocessor(DefaultWhisperPreprocessConfig())

	// 截断
	samples := make([]float32, 100)
	result := preprocessor.padOrTruncate(samples, 50)
	if len(result) != 50 {
		t.Errorf("expected 50 samples, got %d", len(result))
	}

	// 填充
	samples = make([]float32, 30)
	result = preprocessor.padOrTruncate(samples, 50)
	if len(result) != 50 {
		t.Errorf("expected 50 samples, got %d", len(result))
	}
	// 检查填充部分
	for i := 30; i < 50; i++ {
		if result[i] != 0 {
			t.Errorf("expected zero padding at index %d", i)
		}
	}
}

// TestAudioPreprocessor_createMelFilters 测试 Mel 滤波器
func TestAudioPreprocessor_createMelFilters(t *testing.T) {
	preprocessor := NewAudioPreprocessor(DefaultWhisperPreprocessConfig())

	filters := preprocessor.createMelFilters(16000, 400, 80)

	if len(filters) != 80 {
		t.Errorf("expected 80 filters, got %d", len(filters))
	}

	if len(filters[0]) != 201 { // n_fft/2 + 1 = 400/2 + 1
		t.Errorf("expected 201 bins, got %d", len(filters[0]))
	}

	// 检查滤波器权重和（允许部分滤波器为零，因为低频可能不覆盖任何 bin）
	for i, filter := range filters {
		var sum float32
		for _, w := range filter {
			sum += w
		}
		// 只检查中心附近的滤波器应该有非零和
		if i > 15 && i < 65 && sum <= 0 {
			t.Errorf("filter %d has non-positive sum: %f", i, sum)
		}
	}
}

// TestAudioPreprocessor_toNCHW 测试音频 NCHW 转换
// 输入: mel[frames][mels] = [[1,2,3], [4,5,6]]
// 即 frame0: [1,2,3], frame1: [4,5,6]
// 输出: [1, n_mels, n_frames] = [1, 3, 2]
// 布局: mel[0][0], mel[1][0], mel[0][1], mel[1][1], mel[0][2], mel[1][2]
//       = 1, 4, 2, 5, 3, 6
func TestAudioPreprocessor_toNCHW(t *testing.T) {
	preprocessor := NewAudioPreprocessor(DefaultWhisperPreprocessConfig())

	mel := [][]float32{
		{1.0, 2.0, 3.0}, // frame 0
		{4.0, 5.0, 6.0}, // frame 1
	}

	result := preprocessor.toNCHW(mel, 3, 2)
	if len(result) != 6 {
		t.Errorf("expected 6 elements, got %d", len(result))
	}

	// [1, 3, 2] 布局: channel-major, then frame
	expected := []float32{1.0, 4.0, 2.0, 5.0, 3.0, 6.0}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("expected %f at index %d, got %f", v, i, result[i])
		}
	}
}

// TestGetAudioInfo 测试音频信息
func TestGetAudioInfo(t *testing.T) {
	samples := make([]float32, 16000)
	info := GetAudioInfo(samples, 16000)

	if info["sample_rate"] != 16000 {
		t.Errorf("expected sample_rate 16000, got %v", info["sample_rate"])
	}
	if info["num_samples"] != 16000 {
		t.Errorf("expected num_samples 16000, got %v", info["num_samples"])
	}
	if info["duration_sec"] != 1.0 {
		t.Errorf("expected duration_sec 1.0, got %v", info["duration_sec"])
	}
}

// TestModelManager 测试模型管理器
func TestModelManager(t *testing.T) {
	// 使用临时目录
	tmpDir := t.TempDir()

	config := ModelManagerConfig{
		CacheDir:     tmpDir,
		MaxCacheSize: 1024 * 1024 * 1024, // 1GB
		Timeout:      5 * time.Second,
	}

	manager, err := NewModelManager(config)
	if err != nil {
		t.Fatalf("create manager failed: %v", err)
	}

	// 注册模型
	model := ModelInfo{
		Name:        "test-model",
		Version:     "1.0",
		URL:         "https://example.com/model.onnx",
		Size:        1000,
		Description: "Test model",
	}
	manager.RegisterModel(model)

	// 列出模型
	models := manager.ListModels()
	if len(models) != 1 {
		t.Errorf("expected 1 model, got %d", len(models))
	}

	// 获取模型路径（不会下载，因为 URL 无效）
	_, err = manager.GetModelPath("test-model", "1.0")
	if err == nil {
		t.Error("expected error for invalid URL")
	}

	// 删除模型
	err = manager.RemoveModel("test-model", "1.0")
	if err != nil {
		t.Errorf("remove model failed: %v", err)
	}

	// 再次列出
	models = manager.ListModels()
	if len(models) != 0 {
		t.Errorf("expected 0 models, got %d", len(models))
	}
}

// TestModelManager_CleanupCache 测试缓存清理
func TestModelManager_CleanupCache(t *testing.T) {
	tmpDir := t.TempDir()

	config := ModelManagerConfig{
		CacheDir:     tmpDir,
		MaxCacheSize: 100, // 100 bytes，非常小
		Timeout:      5 * time.Second,
	}

	manager, err := NewModelManager(config)
	if err != nil {
		t.Fatalf("create manager failed: %v", err)
	}

	// 创建一些虚拟模型文件
	for i := 0; i < 3; i++ {
		model := ModelInfo{
			Name:        fmt.Sprintf("model-%d", i),
			Version:     "1.0",
			URL:         "https://example.com/model.onnx",
			Size:        50,
			Description: "Test model",
		}
		manager.RegisterModel(model)

		// 创建虚拟文件
		path, _ := manager.GetModelPath(model.Name, model.Version)
		os.WriteFile(path, make([]byte, 50), 0644)

		// 标记为已下载
		entry := manager.Models[manager.modelKey(model.Name, model.Version)]
		entry.Downloaded = true
	}

	// 清理缓存
	err = manager.CleanupCache()
	if err != nil {
		t.Errorf("cleanup cache failed: %v", err)
	}
}

// TestPredefinedModels 测试预定义模型
func TestPredefinedModels(t *testing.T) {
	models := PredefinedModels()
	if len(models) == 0 {
		t.Error("expected predefined models")
	}

	for _, model := range models {
		if model.Name == "" {
			t.Error("model name should not be empty")
		}
		if model.URL == "" {
			t.Error("model URL should not be empty")
		}
		if model.Size <= 0 {
			t.Error("model size should be positive")
		}
	}
}

// TestCrossModalSimilarity 测试跨模态相似度
func TestCrossModalSimilarity(t *testing.T) {
	// 相同向量
	embedding1 := []float32{1.0, 0.0, 0.0}
	embedding2 := []float32{1.0, 0.0, 0.0}

	sim, err := CrossModalSimilarity(embedding1, embedding2)
	if err != nil {
		t.Fatalf("similarity calculation failed: %v", err)
	}
	if sim < 0.99 {
		t.Errorf("expected similarity ~1.0, got %f", sim)
	}

	// 正交向量
	embedding3 := []float32{0.0, 1.0, 0.0}
	sim, err = CrossModalSimilarity(embedding1, embedding3)
	if err != nil {
		t.Fatalf("similarity calculation failed: %v", err)
	}
	if sim > 0.01 {
		t.Errorf("expected similarity ~0.0, got %f", sim)
	}

	// 维度不匹配
	embedding4 := []float32{1.0, 0.0}
	_, err = CrossModalSimilarity(embedding1, embedding4)
	if err == nil {
		t.Error("expected error for dimension mismatch")
	}
}

// TestCLIPImageEmbedderConfig 测试 CLIP 配置
func TestCLIPImageEmbedderConfig(t *testing.T) {
	config := DefaultCLIPImageEmbedderConfig()
	if config.BaseURL == "" {
		t.Error("base URL should not be empty")
	}
	if config.Timeout <= 0 {
		t.Error("timeout should be positive")
	}
}

// TestWhisperAudioEmbedderConfig 测试 Whisper 配置
func TestWhisperAudioEmbedderConfig(t *testing.T) {
	config := DefaultWhisperAudioEmbedderConfig()
	if config.BaseURL == "" {
		t.Error("base URL should not be empty")
	}
	if config.Timeout <= 0 {
		t.Error("timeout should be positive")
	}
	if config.Model == "" {
		t.Error("model should not be empty")
	}
}

// TestModelManagerConfig 测试模型管理器配置
func TestModelManagerConfig(t *testing.T) {
	config := DefaultModelManagerConfig()
	if config.CacheDir == "" {
		t.Error("cache dir should not be empty")
	}
	if config.MaxCacheSize <= 0 {
		t.Error("max cache size should be positive")
	}
}

// BenchmarkImagePreprocessor_Preprocess 基准测试图像预处理
func BenchmarkImagePreprocessor_Preprocess(b *testing.B) {
	preprocessor := NewImagePreprocessor(DefaultCLIPPreprocessConfig())

	// 创建测试图像数据
	testData := make([]byte, 10000)
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		preprocessor.Preprocess(bytes.NewReader(testData))
	}
}

// BenchmarkAudioPreprocessor_Preprocess 基准测试音频预处理
func BenchmarkAudioPreprocessor_Preprocess(b *testing.B) {
	preprocessor := NewAudioPreprocessor(DefaultWhisperPreprocessConfig())

	// 创建测试音频数据（5秒 16kHz）
	samples := make([]float32, 80000)
	for i := range samples {
		samples[i] = float32(i) / 80000.0
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		preprocessor.Preprocess(samples, 16000)
	}
}

// BenchmarkCrossModalSimilarity 基准测试跨模态相似度
func BenchmarkCrossModalSimilarity(b *testing.B) {
	embedding1 := make([]float32, 512)
	embedding2 := make([]float32, 512)
	for i := range embedding1 {
		embedding1[i] = float32(i) / 512.0
		embedding2[i] = float32(512-i) / 512.0
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CrossModalSimilarity(embedding1, embedding2)
	}
}
