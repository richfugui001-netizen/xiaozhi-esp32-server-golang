package models

import (
	"time"

	"gorm.io/gorm"
)

// 用户模型
type User struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	Username  string         `json:"username" gorm:"type:varchar(50);uniqueIndex;not null"`
	Password  string         `json:"-" gorm:"type:varchar(255);not null"`
	Email     string         `json:"email" gorm:"type:varchar(100);uniqueIndex"`
	Role      string         `json:"role" gorm:"type:varchar(20);not null;default:'user'"` // admin, user
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// 设备模型
type Device struct {
	ID           uint           `json:"id" gorm:"primarykey"`
	UserID       uint           `json:"user_id" gorm:"not null"`
	DeviceCode   string         `json:"device_code" gorm:"type:varchar(100);uniqueIndex;not null"`
	DeviceName   string         `json:"device_name" gorm:"type:varchar(100)"`
	Status       string         `json:"status" gorm:"type:varchar(20);default:'offline'"` // online, offline
	LastActiveAt *time.Time     `json:"last_active_at"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

// 智能体模型
type Agent struct {
	ID           uint           `json:"id" gorm:"primarykey"`
	UserID       uint           `json:"user_id" gorm:"not null"`
	Name         string         `json:"name" gorm:"type:varchar(100);not null"`
	Description  string         `json:"description" gorm:"type:text"`
	Config       string         `json:"config" gorm:"type:text"`                         // JSON配置
	LLMConfigID  *uint          `json:"llm_config_id" gorm:"index"`                      // 关联的LLM配置ID
	TTSConfigID  *uint          `json:"tts_config_id" gorm:"index"`                      // 关联的TTS配置ID
	Status       string         `json:"status" gorm:"type:varchar(20);default:'active'"` // active, inactive
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
	
	// 关联关系
	LLMConfig    *Config        `json:"llm_config,omitempty" gorm:"foreignKey:LLMConfigID"`
	TTSConfig    *Config        `json:"tts_config,omitempty" gorm:"foreignKey:TTSConfigID"`
}

// 通用配置模型
type Config struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	Type      string         `json:"type" gorm:"type:varchar(50);not null;index"` // vad, asr, llm, tts, ota, mqtt, udp, mqtt_server, vllm
	Name      string         `json:"name" gorm:"type:varchar(100);not null"`
	Provider  string         `json:"provider" gorm:"type:varchar(50)"`              // 某些配置类型需要provider字段
	JsonData  string         `json:"json_data" gorm:"type:text"`                    // JSON配置数据
	Enabled   bool           `json:"enabled" gorm:"default:true"`
	IsDefault bool           `json:"is_default" gorm:"default:false"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}



// 全局角色模型
type GlobalRole struct {
	ID          uint           `json:"id" gorm:"primarykey"`
	Name        string         `json:"name" gorm:"type:varchar(100);not null"`
	Description string         `json:"description" gorm:"type:text"`
	Prompt      string         `json:"prompt" gorm:"type:text"`
	IsDefault   bool           `json:"is_default" gorm:"default:false"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}
