package database

import (
	"fmt"
	"log"
	"strings"
	"xiaozhi/manager/backend/config"
	"xiaozhi/manager/backend/models"

	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func Init(cfg config.DatabaseConfig) *gorm.DB {
	var db *gorm.DB
	var err error
	
	if cfg.Database == "sqlite" {
		// SQLite 数据库连接
		log.Println("使用SQLite数据库:", cfg.Host)
		db, err = gorm.Open(sqlite.Open(cfg.Host), &gorm.Config{
			DisableForeignKeyConstraintWhenMigrating: true,
		})
	} else {
		// MySQL 数据库连接
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
		
		// 配置GORM以避免外键约束问题
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
			DisableForeignKeyConstraintWhenMigrating: true,
		})
	}

	if err != nil {
		log.Println("数据库连接失败:", err)
		log.Println("将使用fallback模式运行（硬编码用户验证）")
		return nil
	}

	log.Println("已禁用迁移时的外键约束检查")

	// 使用安全的迁移方式，逐个迁移表
	err = safeAutoMigrate(db)
	if err != nil {
		log.Fatal("数据库迁移失败:", err)
	}

	// 创建默认管理员用户
	createDefaultAdmin(db)

	return db
}

func Close(db *gorm.DB) {
	sqlDB, err := db.DB()
	if err != nil {
		log.Println("获取数据库连接失败:", err)
		return
	}
	sqlDB.Close()
}

// 安全的数据库迁移函数
// 逐个迁移表，避免GORM的外键约束问题
func safeAutoMigrate(db *gorm.DB) error {
	log.Println("开始安全数据库迁移...")
	
	// 所有需要迁移的表
	tables := []interface{}{
		&models.User{},
		&models.Config{},
		&models.GlobalRole{},
		&models.Device{},
		&models.Agent{},
	}
	
	// 对每个表都使用安全迁移
	for _, table := range tables {
		if err := safeTableMigration(db, table); err != nil {
			return fmt.Errorf("迁移表 %T 失败: %v", table, err)
		}
	}
	
	log.Println("数据库迁移完成")
	return nil
}

// 安全的表迁移函数
// 处理GORM错误识别uniqueIndex为外键约束的问题
func safeTableMigration(db *gorm.DB, table interface{}) error {
	tableName := fmt.Sprintf("%T", table)
	log.Printf("安全迁移表: %s", tableName)
	
	// 检查表是否存在
	if !db.Migrator().HasTable(table) {
		log.Printf("%s表不存在，直接创建", tableName)
		return db.AutoMigrate(table)
	}
	
	// 表存在，使用安全迁移模式
	log.Printf("%s表已存在，使用安全迁移模式", tableName)
	
	// 禁用外键检查，强制执行迁移（仅MySQL需要）
	if db.Dialector.Name() == "mysql" {
		db.Exec("SET FOREIGN_KEY_CHECKS = 0")
	}
	err := db.AutoMigrate(table)
	if db.Dialector.Name() == "mysql" {
		db.Exec("SET FOREIGN_KEY_CHECKS = 1")
	}
	
	if err != nil {
		log.Printf("%s表迁移出现错误: %v", tableName, err)
		// 忽略GORM错误识别uniqueIndex为外键约束的错误
		if strings.Contains(err.Error(), "Can't DROP") && strings.Contains(err.Error(), "check that column/key exists") {
			log.Printf("忽略%s表的约束删除错误（GORM已知问题）", tableName)
			return nil
		}
		return err
	}
	
	log.Printf("%s表迁移成功", tableName)
	return nil
}

func createDefaultAdmin(db *gorm.DB) {
	var count int64
	db.Model(&models.User{}).Where("role = ?", "admin").Count(&count)

	if count == 0 {
		admin := models.User{
			Username: "admin",
			Password: "$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi", // password
			Role:     "admin",
			Email:    "admin@xiaozhi.com",
		}
		db.Create(&admin)
		log.Println("默认管理员用户已创建: admin/password")
	}
}
