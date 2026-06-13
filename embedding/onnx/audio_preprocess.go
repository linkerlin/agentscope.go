package onnx

import (
	"fmt"
	"math"
)

// AudioPreprocessConfig 音频预处理配置
type AudioPreprocessConfig struct {
	SampleRate     int     // 目标采样率（默认 16000 for Whisper）
	NumSamples     int     // 目标样本数（默认 480000 for 30s）
	NFFT           int     // FFT 窗口大小（默认 400）
	HopLength      int     // 帧移（默认 160）
	NMels          int     // Mel 滤波器数量（默认 80）
	NFrames        int     // 目标帧数（默认 3000）
	Normalize      bool    // 是否归一化
	PadToMaxLength bool    // 是否填充到最大长度
}

// DefaultWhisperPreprocessConfig 返回 Whisper 默认预处理配置
func DefaultWhisperPreprocessConfig() AudioPreprocessConfig {
	return AudioPreprocessConfig{
		SampleRate:     16000,
		NumSamples:     480000, // 30s * 16000
		NFFT:           400,
		HopLength:      160,
		NMels:          80,
		NFrames:        3000,
		Normalize:      true,
		PadToMaxLength: true,
	}
}

// AudioPreprocessor 音频预处理器
type AudioPreprocessor struct {
	Config AudioPreprocessConfig
}

// NewAudioPreprocessor 创建音频预处理器
func NewAudioPreprocessor(config AudioPreprocessConfig) *AudioPreprocessor {
	if config.SampleRate <= 0 {
		config.SampleRate = 16000
	}
	if config.NumSamples <= 0 {
		config.NumSamples = 480000
	}
	return &AudioPreprocessor{Config: config}
}

// Preprocess 预处理音频（PCM float32 数组）
// 输入: 原始 PCM 样本（任意采样率）
// 输出: Mel 频谱图 [1, n_mels, n_frames]
func (p *AudioPreprocessor) Preprocess(samples []float32, originalSampleRate int) ([]float32, error) {
	if len(samples) == 0 {
		return nil, fmt.Errorf("onnx: empty audio samples")
	}

	// 1. 重采样到目标采样率
	resampled := p.resample(samples, originalSampleRate, p.Config.SampleRate)

	// 2. 填充/截断到目标长度
	padded := p.padOrTruncate(resampled, p.Config.NumSamples)

	// 3. 计算 STFT
	stft := p.stft(padded, p.Config.NFFT, p.Config.HopLength)

	// 4. 计算功率谱
	power := p.powerSpectrum(stft)

	// 5. 计算 Mel 频谱图
	mel := p.melFilterbank(power, p.Config.SampleRate, p.Config.NFFT, p.Config.NMels)

	// 6. 对数压缩
	logMel := p.logCompress(mel)

	// 7. 归一化
	if p.Config.Normalize {
		logMel = p.normalize(logMel)
	}

	// 8. 重塑为 [1, n_mels, n_frames]
	result := p.toNCHW(logMel, p.Config.NMels, p.Config.NFrames)

	return result, nil
}

// resample 重采样（线性插值）
func (p *AudioPreprocessor) resample(samples []float32, fromRate, toRate int) []float32 {
	if fromRate == toRate {
		return samples
	}

	ratio := float64(toRate) / float64(fromRate)
	newLen := int(float64(len(samples)) * ratio)
	result := make([]float32, newLen)

	for i := 0; i < newLen; i++ {
		srcIdx := float64(i) / ratio
		srcIdx0 := int(math.Floor(srcIdx))
		srcIdx1 := srcIdx0 + 1

		if srcIdx0 >= len(samples)-1 {
			result[i] = samples[len(samples)-1]
			continue
		}

		dx := float32(srcIdx - float64(srcIdx0))
		result[i] = samples[srcIdx0]*(1-dx) + samples[srcIdx1]*dx
	}

	return result
}

// padOrTruncate 填充或截断到目标长度
func (p *AudioPreprocessor) padOrTruncate(samples []float32, targetLength int) []float32 {
	if len(samples) == targetLength {
		return samples
	}

	if len(samples) > targetLength {
		return samples[:targetLength]
	}

	// 填充零
	result := make([]float32, targetLength)
	copy(result, samples)
	return result
}

// stft 短时傅里叶变换（简化版）
func (p *AudioPreprocessor) stft(samples []float32, nFFT, hopLength int) [][]complex64 {
	numFrames := (len(samples) - nFFT) / hopLength + 1
	result := make([][]complex64, numFrames)

	// Hanning 窗口
	window := make([]float32, nFFT)
	for i := 0; i < nFFT; i++ {
		window[i] = 0.5 * (1 - float32(math.Cos(2*math.Pi*float64(i)/float64(nFFT-1))))
	}

	for frame := 0; frame < numFrames; frame++ {
		start := frame * hopLength
		frameData := make([]complex64, nFFT/2+1)

		// 应用窗口并计算 FFT（简化：使用 DFT）
		for k := 0; k <= nFFT/2; k++ {
			var real, imag float32
			for n := 0; n < nFFT; n++ {
				if start+n < len(samples) {
					sample := samples[start+n] * window[n]
					angle := -2 * math.Pi * float64(k*n) / float64(nFFT)
					real += sample * float32(math.Cos(angle))
					imag += sample * float32(math.Sin(angle))
				}
			}
			frameData[k] = complex(real, imag)
		}

		result[frame] = frameData
	}

	return result
}

// powerSpectrum 计算功率谱
func (p *AudioPreprocessor) powerSpectrum(stft [][]complex64) [][]float32 {
	numFrames := len(stft)
	numBins := len(stft[0])
	result := make([][]float32, numFrames)

	for i := 0; i < numFrames; i++ {
		result[i] = make([]float32, numBins)
		for j := 0; j < numBins; j++ {
			val := stft[i][j]
			result[i][j] = float32(real(val)*real(val) + imag(val)*imag(val))
		}
	}

	return result
}

// melFilterbank 计算 Mel 滤波器组
func (p *AudioPreprocessor) melFilterbank(power [][]float32, sampleRate, nFFT, nMels int) [][]float32 {
	numFrames := len(power)
	result := make([][]float32, numFrames)

	// Mel 滤波器中心频率
	melFilters := p.createMelFilters(sampleRate, nFFT, nMels)

	for i := 0; i < numFrames; i++ {
		result[i] = make([]float32, nMels)
		for j := 0; j < nMels; j++ {
			var sum float32
			for k := 0; k < len(melFilters[j]); k++ {
				if k < len(power[i]) {
					sum += power[i][k] * melFilters[j][k]
				}
			}
			result[i][j] = sum
		}
	}

	return result
}

// createMelFilters 创建 Mel 滤波器
func (p *AudioPreprocessor) createMelFilters(sampleRate, nFFT, nMels int) [][]float32 {
	numBins := nFFT/2 + 1
	filters := make([][]float32, nMels)

	// 频率到 Mel 的转换
	hzToMel := func(hz float64) float64 {
		return 2595 * math.Log10(1+hz/700)
	}
	melToHz := func(mel float64) float64 {
		return 700 * (math.Pow(10, mel/2595) - 1)
	}

	minMel := hzToMel(0)
	maxMel := hzToMel(float64(sampleRate) / 2)
	melStep := (maxMel - minMel) / float64(nMels+1)

	for i := 0; i < nMels; i++ {
		filters[i] = make([]float32, numBins)

		centerMel := minMel + melStep*float64(i+1)
		leftMel := centerMel - melStep
		rightMel := centerMel + melStep

		centerHz := melToHz(centerMel)
		leftHz := melToHz(leftMel)
		rightHz := melToHz(rightMel)

		centerBin := int(centerHz / float64(sampleRate) * float64(nFFT))
		leftBin := int(leftHz / float64(sampleRate) * float64(nFFT))
		rightBin := int(rightHz / float64(sampleRate) * float64(nFFT))

		for j := leftBin; j <= rightBin && j < numBins; j++ {
			if j <= centerBin && centerBin > leftBin {
				filters[i][j] = float32(j-leftBin) / float32(centerBin-leftBin)
			} else if j > centerBin && rightBin > centerBin {
				filters[i][j] = float32(rightBin-j) / float32(rightBin-centerBin)
			}
		}
	}

	return filters
}

// logCompress 对数压缩
func (p *AudioPreprocessor) logCompress(mel [][]float32) [][]float32 {
	result := make([][]float32, len(mel))
	for i := 0; i < len(mel); i++ {
		result[i] = make([]float32, len(mel[i]))
		for j := 0; j < len(mel[i]); j++ {
			// log(1 + x) 避免 log(0)
			result[i][j] = float32(math.Log10(float64(1 + mel[i][j])))
		}
	}
	return result
}

// normalize 归一化（减去均值，除以标准差）
func (p *AudioPreprocessor) normalize(mel [][]float32) [][]float32 {
	// 计算全局均值和标准差
	var sum, sumSq float32
	var count int

	for i := 0; i < len(mel); i++ {
		for j := 0; j < len(mel[i]); j++ {
			sum += mel[i][j]
			sumSq += mel[i][j] * mel[i][j]
			count++
		}
	}

	if count == 0 {
		return mel
	}

	mean := sum / float32(count)
	variance := sumSq/float32(count) - mean*mean
	std := float32(math.Sqrt(float64(variance)))
	if std < 1e-8 {
		std = 1
	}

	result := make([][]float32, len(mel))
	for i := 0; i < len(mel); i++ {
		result[i] = make([]float32, len(mel[i]))
		for j := 0; j < len(mel[i]); j++ {
			result[i][j] = (mel[i][j] - mean) / std
		}
	}

	return result
}

// toNCHW 将 [frames, mels] 转换为 [1, mels, frames]
func (p *AudioPreprocessor) toNCHW(mel [][]float32, nMels, nFrames int) []float32 {
	result := make([]float32, 1*nMels*nFrames)

	for i := 0; i < len(mel) && i < nFrames; i++ {
		for j := 0; j < len(mel[i]) && j < nMels; j++ {
			idx := j*nFrames + i
			result[idx] = mel[i][j]
		}
	}

	return result
}

// GetAudioInfo 获取音频信息
func GetAudioInfo(samples []float32, sampleRate int) map[string]any {
	return map[string]any{
		"sample_rate":   sampleRate,
		"num_samples":   len(samples),
		"duration_sec":  float64(len(samples)) / float64(sampleRate),
	}
}
