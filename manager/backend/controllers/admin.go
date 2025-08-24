package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"xiaozhi/manager/backend/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AdminController struct {
	DB *gorm.DB
}

// 通用配置管理
// GetDeviceConfigs 根据设备ID获取设备关联的配置信息
// 如果设备不存在，则返回全局默认配置
func (ac *AdminController) GetDeviceConfigs(c *gin.Context) {
	deviceID := c.Query("device_id")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_id parameter is required"})
		return
	}

	// 构建配置响应
	type ConfigResponse struct {
		VAD    models.Config `json:"vad"`
		ASR    models.Config `json:"asr"`
		LLM    models.Config `json:"llm"`
		TTS    models.Config `json:"tts"`
		Prompt string        `json:"prompt"`
	}

	var response ConfigResponse

	// 查找设备
	var device models.Device
	var agent models.Agent
	var deviceFound bool

	if err := ac.DB.Where("device_name = ?", deviceID).First(&device).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 设备不存在，使用全局默认配置
			deviceFound = false
			log.Printf("设备 %s 不存在，使用全局默认配置", deviceID)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query device"})
			return
		}
	} else {
		// 设备存在，查找智能体
		deviceFound = true
		log.Printf("设备 %s 存在，AgentID: %d", deviceID, device.AgentID)
		if err := ac.DB.First(&agent, device.AgentID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// 智能体不存在，使用默认配置
				deviceFound = false
				log.Printf("智能体 %d 不存在，使用全局默认配置", device.AgentID)
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query agent"})
				return
			}
		} else {
			response.Prompt = agent.CustomPrompt
			log.Printf("智能体 %d 存在，使用自定义提示词", device.AgentID)
		}
	}

	// 如果设备不存在，使用默认提示词
	if !deviceFound {
		// 查找默认全局角色作为提示词
		var defaultRole models.GlobalRole
		if err := ac.DB.Where("is_default = ?", true).First(&defaultRole).Error; err == nil {
			response.Prompt = defaultRole.Prompt
		} else {
			// 如果没有默认角色，使用空提示词
			response.Prompt = ""
		}
	}

	// 获取VAD默认配置
	if err := ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "vad", true, true).First(&response.VAD).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get default VAD config"})
		return
	}

	// 获取ASR默认配置
	if err := ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "asr", true, true).First(&response.ASR).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get default ASR config"})
		return
	}

	// 获取LLM配置
	if deviceFound && agent.ID != 0 && agent.LLMConfigID != nil && *agent.LLMConfigID != "" {
		// 如果智能体指定了LLM配置，尝试使用它
		if err := ac.DB.Where("config_id = ? AND type = ? AND enabled = ?", *agent.LLMConfigID, "llm", true).First(&response.LLM).Error; err != nil {
			// 如果指定的配置获取失败，回退到默认配置
			if err := ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "llm", true, true).First(&response.LLM).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get default LLM config"})
				return
			}
		}
	} else {
		// 使用默认LLM配置
		if err := ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "llm", true, true).First(&response.LLM).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get default LLM config"})
			return
		}
	}

	// 获取TTS配置
	if deviceFound && agent.ID != 0 && agent.TTSConfigID != nil && *agent.TTSConfigID != "" {
		// 如果智能体指定了TTS配置，尝试使用它
		if err := ac.DB.Where("config_id = ? AND type = ? AND enabled = ?", *agent.TTSConfigID, "tts", true).First(&response.TTS).Error; err != nil {
			// 如果指定的配置获取失败，回退到默认配置
			if err := ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "tts", true, true).First(&response.TTS).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get default TTS config"})
				return
			}
		}
	} else {
		// 使用默认TTS配置
		if err := ac.DB.Where("type = ? AND is_default = ? AND enabled = ?", "tts", true, true).First(&response.TTS).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get default TTS config"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": response})
}

// GetSystemConfigs 获取系统配置信息，包括mqtt, mqtt_server, udp, ota
func (ac *AdminController) GetSystemConfigs(c *gin.Context) {
	// 一次性获取所有相关配置（包括启用和未启用的）
	var allConfigs []models.Config
	if err := ac.DB.Where("type IN (?)", []string{"mqtt", "mqtt_server", "udp", "ota"}).Find(&allConfigs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get system configs"})
		return
	}

	// 按类型分组配置
	configsByType := make(map[string][]models.Config)
	for _, config := range allConfigs {
		configsByType[config.Type] = append(configsByType[config.Type], config)
	}

	// 为每种类型选择最佳配置并解析json_data
	selectAndParseConfig := func(configs []models.Config) interface{} {
		var selectedConfig models.Config
		// 优先选择默认配置
		for _, config := range configs {
			if config.IsDefault {
				selectedConfig = config
				break
			}
		}

		// 如果没有默认配置，选择第一个配置
		if selectedConfig.ID == 0 {
			selectedConfig = configs[0]
		}

		// 解析json_data
		if selectedConfig.JsonData != "" {
			var parsedData interface{}
			if err := json.Unmarshal([]byte(selectedConfig.JsonData), &parsedData); err != nil {
				// 如果解析失败，返回原始json_data字符串
				result := gin.H{
					"name": selectedConfig.Name,
					"type": selectedConfig.Type,
					"data": selectedConfig.JsonData,
				}
				return result
			}

			// 将解析后的数据包装在正确的格式中
			result := gin.H{
				"name": selectedConfig.Name,
				"type": selectedConfig.Type,
			}
			if parsedData != nil {
				// 如果解析的数据是map类型，直接合并
				if dataMap, ok := parsedData.(map[string]interface{}); ok {
					for k, v := range dataMap {
						result[k] = v
					}
				} else {
					// 否则作为data字段
					result["data"] = parsedData
				}
			}
			return result
		}

		// 如果没有json_data，返回基本配置信息
		return gin.H{
			"name": selectedConfig.Name,
			"type": selectedConfig.Type,
		}
	}

	// 构建响应数据
	response := gin.H{}

	// 只有当配置存在时才添加到响应中
	if configs, exists := configsByType["mqtt"]; exists && len(configs) > 0 {
		response["mqtt"] = selectAndParseConfig(configs)
	}
	if configs, exists := configsByType["mqtt_server"]; exists && len(configs) > 0 {
		response["mqtt_server"] = selectAndParseConfig(configs)
	}
	if configs, exists := configsByType["udp"]; exists && len(configs) > 0 {
		response["udp"] = selectAndParseConfig(configs)
	}
	if configs, exists := configsByType["ota"]; exists && len(configs) > 0 {
		response["ota"] = selectAndParseConfig(configs)
	}

	c.JSON(http.StatusOK, gin.H{"data": response})
}

// GetConfigs 获取所有配置列表
func (ac *AdminController) GetConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取配置列表失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

// GetConfig 获取单个配置
func (ac *AdminController) GetConfig(c *gin.Context) {
	id := c.Param("id")
	var config models.Config
	if err := ac.DB.First(&config, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Config not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get config"})
		}
		return
	}
	c.JSON(http.StatusOK, config)
}

func (ac *AdminController) GetConfigByID(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var config models.Config

	if err := ac.DB.First(&config, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": config})
}

func (ac *AdminController) CreateConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 如果设置为默认配置，先取消其他同类型的默认配置
	if config.IsDefault {
		ac.DB.Model(&models.Config{}).Where("type = ? AND is_default = ?", config.Type, true).Update("is_default", false)
	}

	if err := ac.DB.Create(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建配置失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": config})
}

func (ac *AdminController) UpdateConfig(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var config models.Config

	if err := ac.DB.First(&config, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
		return
	}

	var updateData models.Config
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 如果设置为默认配置，先取消其他同类型的默认配置
	if updateData.IsDefault {
		ac.DB.Model(&models.Config{}).Where("type = ? AND is_default = ? AND id != ?", config.Type, true, id).Update("is_default", false)
	}

	// 更新配置
	config.Name = updateData.Name
	config.Provider = updateData.Provider
	config.JsonData = updateData.JsonData
	config.Enabled = updateData.Enabled
	config.IsDefault = updateData.IsDefault

	if err := ac.DB.Save(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新配置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": config})
}

func (ac *AdminController) DeleteConfig(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := ac.DB.Delete(&models.Config{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除配置失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// 设置默认配置
func (ac *AdminController) SetDefaultConfig(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var config models.Config

	if err := ac.DB.First(&config, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
		return
	}

	// 先取消其他同类型的默认配置
	ac.DB.Model(&models.Config{}).Where("type = ? AND is_default = ?", config.Type, true).Update("is_default", false)

	// 设置当前配置为默认
	config.IsDefault = true
	if err := ac.DB.Save(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "设置默认配置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "设置默认配置成功", "data": config})
}

// 获取默认配置
func (ac *AdminController) GetDefaultConfig(c *gin.Context) {
	configType := c.Param("type")
	var config models.Config

	if err := ac.DB.Where("type = ? AND is_default = ?", configType, true).First(&config).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "默认配置不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": config})
}

// GlobalRole管理
func (ac *AdminController) GetGlobalRoles(c *gin.Context) {
	var roles []models.GlobalRole
	if err := ac.DB.Find(&roles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取全局角色失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": roles})
}

func (ac *AdminController) CreateGlobalRole(c *gin.Context) {
	var role models.GlobalRole
	if err := c.ShouldBindJSON(&role); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := ac.DB.Create(&role).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建全局角色失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": role})
}

func (ac *AdminController) UpdateGlobalRole(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var role models.GlobalRole

	if err := ac.DB.First(&role, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "全局角色不存在"})
		return
	}

	if err := c.ShouldBindJSON(&role); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := ac.DB.Save(&role).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新全局角色失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": role})
}

func (ac *AdminController) DeleteGlobalRole(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := ac.DB.Delete(&models.GlobalRole{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除全局角色失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// 用户管理
func (ac *AdminController) GetUsers(c *gin.Context) {
	var users []models.User
	if err := ac.DB.Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户列表失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": users})
}

func (ac *AdminController) CreateUser(c *gin.Context) {
	// 添加明显的调试标记
	log.Println("=== [CreateUser] 方法开始执行 ===")
	log.Println("=== [CreateUser] 这是CreateUser方法的开始 ===")

	// 由于User模型的Password字段使用了json:"-"标签，需要手动解析
	var requestData struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}

	// 直接尝试绑定到map以查看原始数据
	var rawMap map[string]interface{}
	if err := c.ShouldBindJSON(&rawMap); err != nil {
		log.Printf("[CreateUser] 绑定到map失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "JSON解析失败"})
		return
	}
	log.Printf("[CreateUser] 原始JSON数据: %+v", rawMap)

	// 手动提取字段
	username, _ := rawMap["username"].(string)
	email, _ := rawMap["email"].(string)
	password, _ := rawMap["password"].(string)
	role, _ := rawMap["role"].(string)

	// 更新requestData
	requestData.Username = username
	requestData.Email = email
	requestData.Password = password
	requestData.Role = role

	// 验证必要字段
	if requestData.Username == "" || requestData.Email == "" || requestData.Password == "" {
		log.Printf("[CreateUser] 缺少必要字段: username=%s, email=%s, password长度=%d",
			requestData.Username, requestData.Email, len(requestData.Password))
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名、邮箱和密码为必填项"})
		return
	}

	log.Printf("[CreateUser] 接收到用户创建请求 - 用户名: %s, 邮箱: %s, 角色: %s", requestData.Username, requestData.Email, requestData.Role)
	log.Printf("[CreateUser] 原始密码长度: %d", len(requestData.Password))
	log.Printf("[CreateUser] 原始密码内容: %s", requestData.Password)

	// 检查用户名是否已存在
	var existingUser models.User
	err := ac.DB.Where("username = ?", requestData.Username).First(&existingUser).Error
	if err == nil {
		// 用户名已存在
		log.Printf("[CreateUser] 用户名 %s 已存在", requestData.Username)
		c.JSON(http.StatusConflict, gin.H{"error": "用户名已存在"})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		// 数据库查询出错
		log.Printf("[CreateUser] 数据库查询失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建用户失败"})
		return
	}

	// 用户不存在，创建新用户
	log.Printf("[CreateUser] 创建新用户: %s", requestData.Username)
	var user models.User
	user.Username = requestData.Username
	user.Email = requestData.Email
	user.Role = requestData.Role

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(requestData.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[CreateUser] 密码加密失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}
	user.Password = string(hashedPassword)
	log.Printf("[CreateUser] 密码加密成功 - 哈希长度: %d, 哈希前缀: %s", len(user.Password), user.Password[:10])

	if err := ac.DB.Create(&user).Error; err != nil {
		log.Printf("[CreateUser] 数据库创建用户失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建用户失败"})
		return
	}

	log.Printf("[CreateUser] 用户创建成功 - ID: %d, 用户名: %s", user.ID, user.Username)

	// 不返回密码
	user.Password = ""
	c.JSON(http.StatusCreated, gin.H{"data": user})
}

func (ac *AdminController) UpdateUser(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var user models.User

	if err := ac.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	var updateData map[string]interface{}
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 如果更新密码，需要加密
	if password, ok := updateData["password"]; ok && password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password.(string)), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
			return
		}
		updateData["password"] = string(hashedPassword)
	}

	if err := ac.DB.Model(&user).Updates(updateData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新用户失败"})
		return
	}

	// 重新查询用户信息（不包含密码）
	ac.DB.First(&user, id)
	user.Password = ""
	c.JSON(http.StatusOK, gin.H{"data": user})
}

func (ac *AdminController) DeleteUser(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := ac.DB.Delete(&models.User{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除用户失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// 重置用户密码
func (ac *AdminController) ResetUserPassword(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	var requestData struct {
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入有效的新密码（至少6位）"})
		return
	}

	// 查找用户
	var user models.User
	if err := ac.DB.First(&user, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查找用户失败"})
		}
		return
	}

	// 加密新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(requestData.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[ResetUserPassword] 密码加密失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}

	// 更新用户密码
	if err := ac.DB.Model(&user).Update("password", string(hashedPassword)).Error; err != nil {
		log.Printf("[ResetUserPassword] 更新密码失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "重置密码失败"})
		return
	}

	log.Printf("[ResetUserPassword] 管理员重置用户密码成功 - 用户ID: %d, 用户名: %s", user.ID, user.Username)
	c.JSON(http.StatusOK, gin.H{
		"message": "密码重置成功",
		"data": gin.H{
			"user_id":  user.ID,
			"username": user.Username,
		},
	})
}

// 设备管理
func (ac *AdminController) GetDevices(c *gin.Context) {
	var devices []models.Device
	if err := ac.DB.Find(&devices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取设备列表失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": devices})
}

func (ac *AdminController) CreateDevice(c *gin.Context) {
	var req struct {
		UserID     uint   `json:"user_id" binding:"required"`
		DeviceCode string `json:"device_code" binding:"required"`
		DeviceName string `json:"device_name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 检查用户是否存在
	var user models.User
	if err := ac.DB.First(&user, req.UserID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "指定的用户不存在"})
		return
	}

	// 检查设备代码是否已存在
	var existingDevice models.Device
	if err := ac.DB.Where("device_code = ?", req.DeviceCode).First(&existingDevice).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "设备代码已存在"})
		return
	}

	// 创建设备
	device := models.Device{
		UserID:     req.UserID,
		DeviceCode: req.DeviceCode,
		DeviceName: req.DeviceName,
		AgentID:    0,     // 新创建的设备暂不绑定智能体
		Activated:  false, // 新创建的设备默认未激活
	}

	if err := ac.DB.Create(&device).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建设备失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": device})
}

func (ac *AdminController) UpdateDevice(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var device models.Device

	if err := ac.DB.First(&device, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "设备不存在"})
		return
	}

	if err := c.ShouldBindJSON(&device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := ac.DB.Save(&device).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新设备失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": device})
}

func (ac *AdminController) DeleteDevice(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := ac.DB.Delete(&models.Device{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除设备失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// 智能体管理
func (ac *AdminController) GetAgents(c *gin.Context) {
	var agents []models.Agent
	if err := ac.DB.Find(&agents).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取智能体列表失败"})
		return
	}

	// 手动加载关联的配置信息
	type AgentWithConfigs struct {
		models.Agent
		LLMConfig *models.Config `json:"llm_config,omitempty"`
		TTSConfig *models.Config `json:"tts_config,omitempty"`
	}

	var result []AgentWithConfigs
	for _, agent := range agents {
		agentWithConfig := AgentWithConfigs{Agent: agent}

		// 加载LLM配置
		if agent.LLMConfigID != nil && *agent.LLMConfigID != "" {
			var llmConfig models.Config
			if err := ac.DB.Where("config_id = ? AND type = ?", *agent.LLMConfigID, "llm").First(&llmConfig).Error; err == nil {
				agentWithConfig.LLMConfig = &llmConfig
			}
		}

		// 加载TTS配置
		if agent.TTSConfigID != nil && *agent.TTSConfigID != "" {
			var ttsConfig models.Config
			if err := ac.DB.Where("config_id = ? AND type = ?", *agent.TTSConfigID, "tts").First(&ttsConfig).Error; err == nil {
				agentWithConfig.TTSConfig = &ttsConfig
			}
		}

		result = append(result, agentWithConfig)
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (ac *AdminController) CreateAgent(c *gin.Context) {
	var agent models.Agent
	if err := c.ShouldBindJSON(&agent); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := ac.DB.Create(&agent).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建智能体失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": agent})
}

func (ac *AdminController) UpdateAgent(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var agent models.Agent

	if err := ac.DB.First(&agent, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "智能体不存在"})
		return
	}

	if err := c.ShouldBindJSON(&agent); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := ac.DB.Save(&agent).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新智能体失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": agent})
}

func (ac *AdminController) DeleteAgent(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := ac.DB.Delete(&models.Agent{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除智能体失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// VAD配置管理（兼容前端）
func (ac *AdminController) GetVADConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "vad").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VAD configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateVADConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.Type = "vad"
	ac.createConfigWithType(c, &config)
}

func (ac *AdminController) UpdateVADConfig(c *gin.Context) {
	ac.updateConfigWithType(c, "vad")
}

func (ac *AdminController) DeleteVADConfig(c *gin.Context) {
	ac.deleteConfigWithType(c, "vad")
}

// ASR配置管理（兼容前端）
func (ac *AdminController) GetASRConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "asr").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get ASR configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateASRConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.Type = "asr"
	ac.createConfigWithType(c, &config)
}

func (ac *AdminController) UpdateASRConfig(c *gin.Context) {
	ac.updateConfigWithType(c, "asr")
}

func (ac *AdminController) DeleteASRConfig(c *gin.Context) {
	ac.deleteConfigWithType(c, "asr")
}

// LLM配置管理（兼容前端）
func (ac *AdminController) GetLLMConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "llm").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get LLM configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateLLMConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.Type = "llm"
	ac.createConfigWithType(c, &config)
}

func (ac *AdminController) UpdateLLMConfig(c *gin.Context) {
	ac.updateConfigWithType(c, "llm")
}

func (ac *AdminController) DeleteLLMConfig(c *gin.Context) {
	ac.deleteConfigWithType(c, "llm")
}

// TTS配置管理（兼容前端）
func (ac *AdminController) GetTTSConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "tts").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get TTS configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateTTSConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.Type = "tts"
	ac.createConfigWithType(c, &config)
}

func (ac *AdminController) UpdateTTSConfig(c *gin.Context) {
	ac.updateConfigWithType(c, "tts")
}

func (ac *AdminController) DeleteTTSConfig(c *gin.Context) {
	ac.deleteConfigWithType(c, "tts")
}

// VLLM配置管理（兼容前端）
func (ac *AdminController) GetVLLMConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "vllm").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get VLLM configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateVLLMConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.Type = "vllm"
	ac.createConfigWithType(c, &config)
}

func (ac *AdminController) UpdateVLLMConfig(c *gin.Context) {
	ac.updateConfigWithType(c, "vllm")
}

func (ac *AdminController) DeleteVLLMConfig(c *gin.Context) {
	ac.deleteConfigWithType(c, "vllm")
}

// OTA配置管理（兼容前端）
func (ac *AdminController) GetOTAConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "ota").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get OTA configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateOTAConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.Type = "ota"
	ac.createConfigWithType(c, &config)
}

func (ac *AdminController) UpdateOTAConfig(c *gin.Context) {
	ac.updateConfigWithType(c, "ota")
}

func (ac *AdminController) DeleteOTAConfig(c *gin.Context) {
	ac.deleteConfigWithType(c, "ota")
}

// MQTT配置管理（兼容前端）
func (ac *AdminController) GetMQTTConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "mqtt").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get MQTT configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateMQTTConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.Type = "mqtt"
	ac.createConfigWithType(c, &config)
}

func (ac *AdminController) UpdateMQTTConfig(c *gin.Context) {
	ac.updateConfigWithType(c, "mqtt")
}

func (ac *AdminController) DeleteMQTTConfig(c *gin.Context) {
	ac.deleteConfigWithType(c, "mqtt")
}

// MQTT Server配置管理（兼容前端）
func (ac *AdminController) GetMQTTServerConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "mqtt_server").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get MQTT Server configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateMQTTServerConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.Type = "mqtt_server"
	ac.createConfigWithType(c, &config)
}

func (ac *AdminController) UpdateMQTTServerConfig(c *gin.Context) {
	ac.updateConfigWithType(c, "mqtt_server")
}

func (ac *AdminController) DeleteMQTTServerConfig(c *gin.Context) {
	ac.deleteConfigWithType(c, "mqtt_server")
}

// UDP配置管理（兼容前端）
func (ac *AdminController) GetUDPConfigs(c *gin.Context) {
	var configs []models.Config
	if err := ac.DB.Where("type = ?", "udp").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get UDP configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func (ac *AdminController) CreateUDPConfig(c *gin.Context) {
	var config models.Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.Type = "udp"
	ac.createConfigWithType(c, &config)
}

func (ac *AdminController) UpdateUDPConfig(c *gin.Context) {
	ac.updateConfigWithType(c, "udp")
}

func (ac *AdminController) DeleteUDPConfig(c *gin.Context) {
	ac.deleteConfigWithType(c, "udp")
}

// ToggleConfigEnable 切换配置的启用状态
func (ac *AdminController) ToggleConfigEnable(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid config ID"})
		return
	}

	var config models.Config
	if err := ac.DB.First(&config, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询配置失败"})
		}
		return
	}

	// 切换启用状态
	config.Enabled = !config.Enabled
	if err := ac.DB.Save(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新配置状态失败"})
		return
	}

	status := "禁用"
	if config.Enabled {
		status = "启用"
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("配置已%s", status),
		"data":    config,
	})
}

// 辅助方法
func (ac *AdminController) createConfigWithType(c *gin.Context, config *models.Config) {
	// 如果没有提供config_id，自动生成一个
	if config.ConfigID == "" {
		// 使用类型_名称_时间戳的格式生成唯一ID
		timestamp := time.Now().Unix()
		safeName := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(config.Name, " ", "_"), "-", "_"))
		config.ConfigID = fmt.Sprintf("%s_%s_%d", config.Type, safeName, timestamp)
	}

	// 如果设置为默认配置，先取消其他同类型的默认配置
	if config.IsDefault {
		ac.DB.Model(&models.Config{}).Where("type = ? AND is_default = ?", config.Type, true).Update("is_default", false)
	}

	if err := ac.DB.Create(config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建配置失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": *config})
}

func (ac *AdminController) updateConfigWithType(c *gin.Context, configType string) {
	id, _ := strconv.Atoi(c.Param("id"))
	var config models.Config

	if err := ac.DB.Where("id = ? AND type = ?", id, configType).First(&config).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
		return
	}

	var updateData models.Config
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 如果设置为默认配置，先取消其他同类型的默认配置
	if updateData.IsDefault {
		ac.DB.Model(&models.Config{}).Where("type = ? AND is_default = ? AND id != ?", configType, true, id).Update("is_default", false)
	}

	// 更新配置
	config.Name = updateData.Name
	config.Provider = updateData.Provider
	config.JsonData = updateData.JsonData
	config.Enabled = updateData.Enabled
	config.IsDefault = updateData.IsDefault

	// 如果提供了新的config_id，则更新它
	if updateData.ConfigID != "" {
		config.ConfigID = updateData.ConfigID
	}

	if err := ac.DB.Save(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新配置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": config})
}

func (ac *AdminController) deleteConfigWithType(c *gin.Context, configType string) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := ac.DB.Where("id = ? AND type = ?", id, configType).Delete(&models.Config{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除配置失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}
