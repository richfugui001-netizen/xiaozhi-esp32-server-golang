package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
	"xiaozhi-esp32-server-golang/internal/app/server/auth"
	redisdb "xiaozhi-esp32-server-golang/internal/db/redis"
	user_config "xiaozhi-esp32-server-golang/internal/domain/config"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	logrus "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func Init(configFile string) error {
	//init config
	err := initConfig(configFile)
	if err != nil {
		fmt.Printf("initConfig err: %+v", err)
		os.Exit(1)
		return err
	}

	//init log
	initLog()

	// 从接口获取配置并更新
	if err := updateConfigFromAPI(); err != nil {
		fmt.Printf("从接口获取配置失败，使用本地配置: %v\n", err)
	}

	//init vad
	//initVad()

	//init redis
	initRedis()

	//init auth
	err = initAuthManager()
	if err != nil {
		fmt.Printf("initAuthManager err: %+v", err)
		os.Exit(1)
		return err
	}

	return nil
}

func initConfig(configFile string) error {
	viper.SetConfigFile(configFile)

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		return err
	}

	return nil
}

// updateConfigFromAPI 从接口获取配置并更新viper配置
func updateConfigFromAPI() error {
	configProviderType := viper.GetString("config_provider.type")

	fmt.Printf("获取系统配置, config_provider.type: %s\n", configProviderType)

	// 从配置文件获取后端管理系统地址
	configProvider, err := user_config.GetProvider(configProviderType)
	if err != nil {
		return fmt.Errorf("获取配置提供者失败: %v", err)
	}

	// 创建上下文
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 获取系统配置JSON字符串
	configJSON, err := configProvider.GetSystemConfig(ctx)
	if err != nil {
		return fmt.Errorf("获取系统配置失败: %v", err)
	}

	if configJSON == "" {
		return nil
	}

	// 解析JSON为map
	var configMap map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &configMap); err != nil {
		return fmt.Errorf("解析配置JSON失败: %v", err)
	}

	// 使用viper.MergeConfigMap设置到viper
	if err := viper.MergeConfigMap(configMap); err != nil {
		return fmt.Errorf("合并配置到viper失败: %v", err)
	}

	return nil
}

func initLog() error {
	// 输出到文件
	binPath, _ := os.Executable()
	baseDir := filepath.Dir(binPath)
	logPath := fmt.Sprintf("%s/%s%s", baseDir, viper.GetString("log.path"), viper.GetString("log.file"))
	/* 日志轮转相关函数
	`WithLinkName` 为最新的日志建立软连接
	`WithRotationTime` 设置日志分割的时间，隔多久分割一次
	WithMaxAge 和 WithRotationCount二者只能设置一个
		`WithMaxAge` 设置文件清理前的最长保存时间
		`WithRotationCount` 设置文件清理前最多保存的个数
	*/
	// 下面配置日志每隔 1 分钟轮转一个新文件，保留最近 3 分钟的日志文件，多余的自动清理掉。
	writer, err := rotatelogs.New(
		logPath+".%Y%m%d",
		rotatelogs.WithLinkName(logPath),
		rotatelogs.WithRotationCount(uint(viper.GetInt("log.max_age"))),
		rotatelogs.WithRotationTime(time.Duration(86400)*time.Second),
	)
	if err != nil {
		fmt.Printf("init log error: %v\n", err)
		os.Exit(1)
		return err
	}

	// 根据配置决定输出目标
	if viper.GetBool("log.stdout") {
		// 同时输出到文件和标准输出
		multiWriter := io.MultiWriter(writer, os.Stdout)
		logrus.SetOutput(multiWriter)
		logrus.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05.000", //时间格式化，添加毫秒
			ForceColors:     true,                      // 标准输出启用颜色
		})
	} else {
		// 只输出到文件
		logrus.SetOutput(writer)
		logrus.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05.000", //时间格式化，添加毫秒
			ForceColors:     false,                     // 文件输出不启用颜色
		})
	}

	// 禁用默认的调用者报告，使用自定义的caller字段
	logrus.SetReportCaller(false)
	logLevel, _ := logrus.ParseLevel(viper.GetString("log.level"))
	logrus.SetLevel(logLevel)

	return nil
}

/*
	func initVad() error {
		err := vad.InitVAD()
		if err != nil {
			fmt.Printf("initVad error: %v\n", err)
			os.Exit(1)
			return err
		}
		return nil
	}
*/
func initRedis() error {
	// 初始化我们的统一Redis模块
	redisConfig := &redisdb.Config{
		Host:     viper.GetString("redis.host"),
		Port:     viper.GetInt("redis.port"),
		Password: viper.GetString("redis.password"),
		DB:       viper.GetInt("redis.db"),
	}

	err := redisdb.Init(redisConfig)
	if err != nil {
		fmt.Printf("init redis error: %v\n", err)
		return err
	}

	return nil
}

func initAuthManager() error {
	return auth.Init()
}
