package main

import (
	"log"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	// 测试密码: simple123
	testPassword := "simple123"
	log.Printf("原始密码: '%s', 长度: %d", testPassword, len(testPassword))
	
	// 使用bcrypt加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("密码加密失败: %v", err)
	}
	
	hashedStr := string(hashedPassword)
	log.Printf("加密后密码: '%s'", hashedStr)
	log.Printf("哈希长度: %d, 前缀: %s", len(hashedStr), hashedStr[:10])
	
	// 验证密码
	err = bcrypt.CompareHashAndPassword(hashedPassword, []byte(testPassword))
	if err != nil {
		log.Printf("❌ 密码验证失败: %v", err)
	} else {
		log.Printf("✅ 密码验证成功")
	}
	
	// 测试错误密码
	wrongPassword := "wrong123"
	err = bcrypt.CompareHashAndPassword(hashedPassword, []byte(wrongPassword))
	if err != nil {
		log.Printf("❌ 错误密码验证失败 (预期): %v", err)
	} else {
		log.Printf("⚠️ 错误密码验证成功 (异常)")
	}
	
	// 测试数据库中的实际哈希
	// 从日志中看到的哈希: $2a$10$iR3PEaQqTI4vIUqeKMqKbO7ETChQh8Pqdshh4QZeNWoa2vDMM7UlK
	dbHash := "$2a$10$iR3PEaQqTI4vIUqeKMqKbO7ETChQh8Pqdshh4QZeNWoa2vDMM7UlK"
	log.Printf("\n=== 测试数据库中的实际哈希 ===")
	log.Printf("数据库哈希: %s", dbHash)
	
	// 测试各种可能的密码
	testPasswords := []string{"simple123", "testpassword123", "password", "admin"}
	
	for _, pwd := range testPasswords {
		err = bcrypt.CompareHashAndPassword([]byte(dbHash), []byte(pwd))
		if err != nil {
			log.Printf("❌ 密码 '%s' 验证失败: %v", pwd, err)
		} else {
			log.Printf("✅ 密码 '%s' 验证成功！", pwd)
		}
	}
}