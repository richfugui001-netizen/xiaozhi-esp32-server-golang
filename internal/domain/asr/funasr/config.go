package funasr

import (
	"log"
	"os"

	"github.com/spf13/viper"
)

// LoadConfig 从配置文件加载FunASR配置
func LoadConfig(configPath string) FunasrConfig {
	// 默认配置
	config := DefaultConfig

	// 初始化 Viper
	v := viper.New()

	// 设置配置文件类型
	v.SetConfigType("json")

	// 如果未指定配置文件路径，尝试查找默认路径
	if configPath == "" {
		// 尝试多个可能的路径
		possiblePaths := []string{
			"config/config.json",
			"xiaozhi-esp32-server-golang/config/config.json",
			"../config/config.json",
			"../../config/config.json",
		}

		found := false
		for _, path := range possiblePaths {
			if _, err := os.Stat(path); err == nil {
				configPath = path
				found = true
				break
			}
		}

		if !found {
			log.Printf("未找到配置文件，使用默认FunASR配置")
			return config
		}
	}

	// 设置配置文件路径
	v.SetConfigFile(configPath)

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		log.Printf("读取配置文件失败: %v，使用默认FunASR配置", err)
		return config
	}

	// 检查是否存在ASR配置
	if !v.IsSet("asr.funasr") {
		log.Printf("配置文件中未找到ASR.FunASR部分，使用默认FunASR配置")
		return config
	}

	// 从配置中获取FunASR配置
	if v.IsSet("asr.funasr.host") {
		config.Host = v.GetString("asr.funasr.host")
	}
	if v.IsSet("asr.funasr.port") {
		config.Port = v.GetString("asr.funasr.port")
	}
	if v.IsSet("asr.funasr.mode") {
		config.Mode = v.GetString("asr.funasr.mode")
	}
	if v.IsSet("asr.funasr.sample_rate") {
		config.SampleRate = v.GetInt("asr.funasr.sample_rate")
	}
	if v.IsSet("asr.funasr.chunk_interval") {
		config.ChunkInterval = v.GetInt("asr.funasr.chunk_interval")
	}
	if v.IsSet("asr.funasr.max_connections") {
		config.MaxConnections = v.GetInt("asr.funasr.max_connections")
	}
	if v.IsSet("asr.funasr.timeout") {
		config.Timeout = v.GetInt("asr.funasr.timeout")
	}

	log.Printf("已加载FunASR配置: %+v", config)
	return config
}
