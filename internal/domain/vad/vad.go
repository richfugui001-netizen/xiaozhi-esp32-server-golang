package vad

import (
	"errors"
	"fmt"
	log "xiaozhi-esp32-server-golang/logger"
	"sync"
	"time"

	"github.com/spf13/viper"
	"github.com/streamer45/silero-vad-go/speech"
)

// VAD默认配置
var defaultVADConfig = map[string]interface{}{
	"threshold":               0.5,
	"min_silence_duration_ms": int64(100),
	"sample_rate":             16000,
	"channels":                1,
	"speech_pad_ms":           60,
}

// 资源池默认配置
var defaultPoolConfig = struct {
	// 池大小
	MaxSize int
	// 获取超时时间（毫秒）
	AcquireTimeout int64
}{
	MaxSize:        10,
	AcquireTimeout: 3000, // 3秒
}

// 配置项路径
const (
	// VAD模型路径配置项
	ConfigKeyVADModelPath = "vad.model_path"
	// VAD阈值配置项
	ConfigKeyVADThreshold = "vad.threshold"
	// VAD静默时长配置项
	ConfigKeySilenceDuration = "vad.min_silence_duration_ms"
	// VAD采样率配置项
	ConfigKeySampleRate = "vad.sample_rate"
	// VAD通道数配置项
	ConfigKeyChannels = "vad.channels"
	// VAD资源池大小配置项
	ConfigKeyPoolSize = "vad.pool_size"
	// VAD获取超时时间配置项
	ConfigKeyAcquireTimeout = "vad.acquire_timeout_ms"
)

// 全局变量和初始化
var (
	// 全局解码器实例池
	opusDecoderMap sync.Map
	// 全局VAD检测器实例池
	vadDetectorMap sync.Map
	// 全局初始化锁
	initMutex sync.Mutex
	// 初始化标志
	initialized = false
	// 全局VAD资源池实例
	globalVADResourcePool *VADResourcePool
)

// 声明VAD工厂函数变量类型
var vadFactoryFunc func(string, map[string]interface{}) (VAD, error)

// 使用包初始化函数预创建资源
func InitVAD() error {
	log.Info("VAD模块初始化...")

	// 初始化VAD资源池（使用默认配置）
	globalVADResourcePool = &VADResourcePool{
		maxSize:        defaultPoolConfig.MaxSize,
		acquireTimeout: defaultPoolConfig.AcquireTimeout,
		defaultConfig:  defaultVADConfig,
		initialized:    false, // 标记为未完全初始化，需要后续读取配置
	}

	// 初始化VAD工厂函数
	vadFactoryFunc = createVADInstance

	// 尝试自动从配置文件初始化
	err := InitVADFromConfig()
	if err != nil {
		log.Errorf("VAD模块初始化失败: %v", err)
		return err
	}

	log.Info("VAD模块初始化完成")
	return nil
}

// InitVADFromConfig 从配置文件初始化VAD模块
func InitVADFromConfig() error {
	// 从viper获取模型路径
	modelPath := viper.GetString(ConfigKeyVADModelPath)
	if modelPath == "" {
		log.Warnf("未从配置中找到VAD模型路径，请确保已配置 %s", ConfigKeyVADModelPath)
		return errors.New("VAD模型路径未配置")
	}

	// 获取其他可选配置
	if threshold := viper.GetFloat64(ConfigKeyVADThreshold); threshold > 0 {
		globalVADResourcePool.defaultConfig["threshold"] = threshold
	}

	if silenceMs := viper.GetInt64(ConfigKeySilenceDuration); silenceMs > 0 {
		globalVADResourcePool.defaultConfig["min_silence_duration_ms"] = silenceMs
	}

	if sampleRate := viper.GetInt(ConfigKeySampleRate); sampleRate > 0 {
		globalVADResourcePool.defaultConfig["sample_rate"] = sampleRate
	}

	if channels := viper.GetInt(ConfigKeyChannels); channels > 0 {
		globalVADResourcePool.defaultConfig["channels"] = channels
	}

	// VAD资源池特有配置
	if poolSize := viper.GetInt(ConfigKeyPoolSize); poolSize > 0 {
		globalVADResourcePool.maxSize = poolSize
	}

	if timeout := viper.GetInt64(ConfigKeyAcquireTimeout); timeout > 0 {
		globalVADResourcePool.acquireTimeout = timeout
	}

	// 设置模型路径并完成初始化
	return initVADResourcePool(modelPath)
}

// 内部方法：初始化VAD资源池
func initVADResourcePool(modelPath string) error {
	if modelPath == "" {
		return errors.New("模型路径不能为空")
	}

	initMutex.Lock()
	defer initMutex.Unlock()

	// 已经初始化过，检查模型路径是否变更
	if globalVADResourcePool.initialized {
		currentPath, ok := globalVADResourcePool.defaultConfig["model_path"].(string)
		if ok && currentPath == modelPath {
			return nil // 模型路径未变，无需重复初始化
		}
		log.Infof("VAD资源池模型路径变更，重新初始化: %s", modelPath)
	}

	// 设置模型路径
	globalVADResourcePool.defaultConfig["model_path"] = modelPath

	// 初始化资源池
	err := globalVADResourcePool.initialize()
	if err != nil {
		return fmt.Errorf("初始化VAD资源池失败: %v", err)
	}

	globalVADResourcePool.initialized = true
	log.Infof("VAD资源池初始化完成，模型路径: %s，池大小: %d", modelPath, globalVADResourcePool.maxSize)
	return nil
}

// UpdateVADConfig 监听配置变更并更新VAD设置
func UpdateVADConfig() error {
	// 重新从配置文件加载
	return InitVADFromConfig()
}

// VAD 语音活动检测接口
type VAD interface {
	// IsVAD 检测音频数据中的语音活动
	IsVAD(pcmData []float32) (bool, error)
	// Reset 重置检测器状态
	Reset() error
	// Close 关闭并释放资源
	Close() error
}

// SileroVAD Silero VAD模型实现
type SileroVAD struct {
	detector         *speech.Detector
	vadThreshold     float32
	silenceThreshold int64 // 单位:毫秒
	sampleRate       int   // 采样率
	channels         int   // 通道数
	mu               sync.Mutex
}

// NewSileroVAD 创建SileroVAD实例
func NewSileroVAD(config map[string]interface{}) (*SileroVAD, error) {
	threshold, ok := config["threshold"].(float64)
	if !ok {
		threshold = 0.5 // 默认阈值
	}

	silenceMs, ok := config["min_silence_duration_ms"].(int64)
	if !ok {
		silenceMs = 100 // 默认500毫秒
	}

	sampleRate, ok := config["sample_rate"].(int)
	if !ok {
		sampleRate = 16000 // 默认采样率
	}

	channels, ok := config["channels"].(int)
	if !ok {
		channels = 1 // 默认单声道
	}

	speechPadMs, ok := config["speech_pad_ms"].(int)
	if !ok {
		speechPadMs = 30 // 默认语音前后填充
	}

	modelPath, ok := config["model_path"].(string)
	if !ok {
		return nil, errors.New("缺少模型路径配置")
	}

	// 创建语音检测器
	detector, err := speech.NewDetector(speech.DetectorConfig{
		ModelPath:            modelPath,
		SampleRate:           sampleRate,
		Threshold:            float32(threshold),
		MinSilenceDurationMs: int(silenceMs),
		SpeechPadMs:          speechPadMs,
		LogLevel:             speech.LogLevelWarn,
	})
	if err != nil {
		return nil, err
	}

	return &SileroVAD{
		detector:         detector,
		vadThreshold:     float32(threshold),
		silenceThreshold: silenceMs,
		sampleRate:       sampleRate,
		channels:         channels,
	}, nil
}

// IsVAD 实现VAD接口的IsVAD方法
func (s *SileroVAD) IsVAD(pcmData []float32) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	segments, err := s.detector.Detect(pcmData)
	if err != nil {
		log.Errorf("检测失败: %s", err)
		return false, err
	}

	for _, s := range segments {
		log.Debugf("speech starts at %0.2fs", s.SpeechStartAt)
		if s.SpeechEndAt > 0 {
			log.Debugf("speech ends at %0.2fs", s.SpeechEndAt)
		}
	}

	return len(segments) > 0, nil
}

// Close 关闭并释放资源
func (s *SileroVAD) Close() error {
	if s.detector != nil {
		return s.detector.Destroy()
	}
	return nil
}

// createVADInstance 创建指定类型的VAD实例（内部实现）
func createVADInstance(vadType string, config map[string]interface{}) (VAD, error) {
	switch vadType {
	case "SileroVAD":
		return NewSileroVAD(config)
	default:
		return nil, errors.New("不支持的VAD类型: " + vadType)
	}
}

// CreateVAD 创建指定类型的VAD实例（公共API）
func CreateVAD(vadType string, config map[string]interface{}) (VAD, error) {
	return vadFactoryFunc(vadType, config)
}

// VADResourcePool VAD资源池管理，不与会话ID绑定
type VADResourcePool struct {
	// 可用的VAD实例队列
	availableVADs chan VAD
	// 已分配的VAD实例映射，用于跟踪和管理
	allocatedVADs sync.Map
	// 池大小配置
	maxSize int
	// 获取VAD超时时间（毫秒）
	acquireTimeout int64
	// 默认VAD配置
	defaultConfig map[string]interface{}
	// 互斥锁，用于初始化和重置操作
	mu sync.Mutex
	// 是否已初始化标志
	initialized bool
}

// initialize 初始化VAD资源池
func (p *VADResourcePool) initialize() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 已经初始化过，先关闭现有资源
	if p.availableVADs != nil {
		close(p.availableVADs)
		p.availableVADs = nil

		// 释放所有已分配的VAD实例
		p.allocatedVADs.Range(func(key, value interface{}) bool {
			if sileroVAD, ok := value.(*SileroVAD); ok {
				sileroVAD.Close()
			}
			p.allocatedVADs.Delete(key)
			return true
		})
	}

	// 创建资源队列
	p.availableVADs = make(chan VAD, p.maxSize)

	// 预创建VAD实例
	for i := 0; i < p.maxSize; i++ {
		vadInstance, err := CreateVAD("SileroVAD", p.defaultConfig)
		if err != nil {
			// 关闭已创建的实例
			for j := 0; j < i; j++ {
				vad := <-p.availableVADs
				if sileroVAD, ok := vad.(*SileroVAD); ok {
					sileroVAD.Close()
				}
			}
			close(p.availableVADs)
			p.availableVADs = nil

			return fmt.Errorf("预创建VAD实例失败: %v", err)
		}

		// 放入可用队列
		p.availableVADs <- vadInstance
	}

	log.Infof("VAD资源池初始化完成，创建了 %d 个VAD实例", p.maxSize)
	return nil
}

// AcquireVAD 从资源池获取一个VAD实例
func (p *VADResourcePool) AcquireVAD() (VAD, error) {
	if !p.initialized {
		return nil, errors.New("VAD资源池未初始化")
	}

	// 设置超时
	timeout := time.After(time.Duration(p.acquireTimeout) * time.Millisecond)

	log.Debugf("获取VAD实例, 当前可用: %d/%d", len(p.availableVADs), p.maxSize)

	// 尝试从池中获取一个VAD实例
	select {
	case vad := <-p.availableVADs:
		if vad == nil {
			return nil, errors.New("VAD资源池已关闭")
		}

		// 标记为已分配
		p.allocatedVADs.Store(vad, time.Now())

		log.Debugf("从VAD资源池获取了一个VAD实例，当前可用: %d/%d", len(p.availableVADs), p.maxSize)
		return vad, nil

	case <-timeout:
		return nil, fmt.Errorf("获取VAD实例超时，当前资源池已满载运行（%d/%d）", p.maxSize, p.maxSize)
	}
}

// ReleaseVAD 释放VAD实例回资源池
func (p *VADResourcePool) ReleaseVAD(vad VAD) {
	if vad == nil || !p.initialized {
		return
	}

	log.Debugf("释放VAD实例: %v, 当前可用: %d/%d", vad, len(p.availableVADs), p.maxSize)

	// 检查是否是从此池分配的实例
	if _, exists := p.allocatedVADs.Load(vad); exists {
		// 从已分配映射中删除
		p.allocatedVADs.Delete(vad)

		// 如果资源池已关闭，直接销毁实例
		if p.availableVADs == nil {
			if sileroVAD, ok := vad.(*SileroVAD); ok {
				sileroVAD.Close()
			}
			return
		}

		// 尝试放回资源池，如果满了就丢弃
		select {
		case p.availableVADs <- vad:
			log.Debugf("VAD实例已归还资源池，当前可用: %d/%d", len(p.availableVADs), p.maxSize)
		default:
			// 资源池满了，直接关闭实例
			if sileroVAD, ok := vad.(*SileroVAD); ok {
				sileroVAD.Close()
			}
			log.Warn("VAD资源池已满，多余实例已销毁")
		}
	} else {
		log.Warn("尝试释放非此资源池管理的VAD实例")
	}
}

// GetActiveCount 获取当前活跃（被分配）的VAD实例数量
func (p *VADResourcePool) GetActiveCount() int {
	count := 0
	p.allocatedVADs.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

// GetAvailableCount 获取当前可用的VAD实例数量
func (p *VADResourcePool) GetAvailableCount() int {
	if p.availableVADs == nil {
		return 0
	}
	return len(p.availableVADs)
}

// Resize 调整资源池大小
func (p *VADResourcePool) Resize(newSize int) error {
	if newSize <= 0 {
		return errors.New("资源池大小必须大于0")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	currentSize := p.maxSize

	// 如果新大小小于当前大小，需要减少实例数量
	if newSize < currentSize {
		// 更新大小配置
		p.maxSize = newSize

		// 计算需要释放的实例数量
		toRemove := currentSize - newSize
		for i := 0; i < toRemove; i++ {
			// 尝试从可用队列中取出实例并关闭
			select {
			case vad := <-p.availableVADs:
				if sileroVAD, ok := vad.(*SileroVAD); ok {
					sileroVAD.Close()
				}
			default:
				// 没有更多可用实例了，退出循环
				break
			}
		}

		log.Infof("VAD资源池大小已调整：%d -> %d", currentSize, newSize)
		return nil
	}

	// 如果新大小大于当前大小，需要增加实例数量
	if newSize > currentSize {
		// 计算需要增加的实例数量
		toAdd := newSize - currentSize

		// 创建新的VAD实例
		for i := 0; i < toAdd; i++ {
			vadInstance, err := CreateVAD("SileroVAD", p.defaultConfig)
			if err != nil {
				// 有错误发生，更新大小为当前已成功创建的实例数
				actualNewSize := currentSize + i
				p.maxSize = actualNewSize

				log.Errorf("无法创建全部请求的VAD实例，资源池大小已调整为: %d", actualNewSize)
				return fmt.Errorf("创建新VAD实例失败: %v", err)
			}

			// 放入可用队列
			select {
			case p.availableVADs <- vadInstance:
				// 成功放入队列
			default:
				// 队列已满，直接关闭实例
				if sileroVAD, ok := vadInstance.(*SileroVAD); ok {
					sileroVAD.Close()
				}
				log.Warn("无法将新创建的VAD实例放入可用队列，实例已销毁")
			}
		}

		// 更新大小配置
		p.maxSize = newSize

		log.Infof("VAD资源池大小已调整：%d -> %d", currentSize, newSize)
		return nil
	}

	// 大小相同，无需调整
	return nil
}

// Close 关闭资源池，释放所有资源
func (p *VADResourcePool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.availableVADs != nil {
		// 关闭可用队列
		close(p.availableVADs)

		// 释放所有可用的VAD实例
		for vad := range p.availableVADs {
			if sileroVAD, ok := vad.(*SileroVAD); ok {
				sileroVAD.Close()
			}
		}

		p.availableVADs = nil
	}

	// 释放所有已分配的VAD实例
	p.allocatedVADs.Range(func(key, _ interface{}) bool {
		vad := key.(VAD)
		if sileroVAD, ok := vad.(*SileroVAD); ok {
			sileroVAD.Close()
		}
		p.allocatedVADs.Delete(key)
		return true
	})

	p.initialized = false
	log.Info("VAD资源池已关闭，所有资源已释放")
}

// GetVADResourcePool 获取全局VAD资源池实例
func GetVADResourcePool() (*VADResourcePool, error) {
	if globalVADResourcePool == nil || !globalVADResourcePool.initialized {
		// 尝试自动初始化
		if err := InitVADFromConfig(); err != nil {
			return nil, errors.New("VAD资源池未完全初始化，请在配置文件中设置 " + ConfigKeyVADModelPath)
		}
	}
	return globalVADResourcePool, nil
}

// AcquireVAD 获取一个VAD实例
func AcquireVAD() (VAD, error) {
	if globalVADResourcePool == nil {
		return nil, errors.New("VAD资源池尚未初始化")
	}

	if !globalVADResourcePool.initialized {
		// 尝试自动初始化
		if err := InitVADFromConfig(); err != nil {
			return nil, errors.New("VAD模型路径未配置，请在配置文件中设置 " + ConfigKeyVADModelPath)
		}
	}

	return globalVADResourcePool.AcquireVAD()
}

// ReleaseVAD 释放一个VAD实例
func ReleaseVAD(vad VAD) {
	if globalVADResourcePool != nil && globalVADResourcePool.initialized {
		globalVADResourcePool.ReleaseVAD(vad)
	}
}

// Reset 重置VAD检测器状态
func (s *SileroVAD) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.detector.Reset()
}

// SetThreshold 设置VAD检测阈值
func (s *SileroVAD) SetThreshold(threshold float32) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.vadThreshold = threshold
	// 注意：silero-vad-go 库的 detector 没有直接提供 SetThreshold 方法
	// 只能修改实例的阈值，在下次检测时生效
}
