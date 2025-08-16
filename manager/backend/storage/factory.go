package storage

import (
	"fmt"

	"xiaozhi/manager/backend/config"
	"xiaozhi/manager/backend/storage/mysql"
)

// StorageType 存储类型
type StorageType string

const (
	StorageTypeMySQL StorageType = "mysql"
)

// Factory 存储工厂
type Factory struct{}

// NewFactory 创建存储工厂
func NewFactory() *Factory {
	return &Factory{}
}

// CreateStorage 创建存储实例
func CreateStorage(dbConfig config.DatabaseConfig) (*StorageAdapter, error) {
	// 只支持MySQL，直接创建MySQL存储
	// 验证MySQL配置
	if err := mysql.ValidateConfig(dbConfig); err != nil {
		return nil, fmt.Errorf("invalid MySQL config: %w", err)
	}
	// 创建MySQL配置
	mysqlConfig := mysql.NewConfigFromDatabase(dbConfig)
	// 创建MySQL存储
	mysqlStorage, err := mysql.NewStorage(mysqlConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create MySQL storage: %w", err)
	}
	// 创建基础存储
	baseStorage := NewGormBaseStorage(mysqlStorage.DB)
	// 返回适配器
	return NewStorageAdapter(baseStorage), nil
}

// GetSupportedTypes 获取支持的存储类型
func (f *Factory) GetSupportedTypes() []StorageType {
	return []StorageType{
		StorageTypeMySQL,
	}
}

// ValidateConfig 验证存储配置
func ValidateConfig(storageType string, dbConfig config.DatabaseConfig) error {
	switch StorageType(storageType) {
	case StorageTypeMySQL:
		return mysql.ValidateConfig(dbConfig)
	default:
		return fmt.Errorf("unsupported storage type: %s", storageType)
	}
}