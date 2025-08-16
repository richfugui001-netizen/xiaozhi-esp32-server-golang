package controllers

import (
	"net/http"
	"strconv"
	"xiaozhi/manager/backend/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UserController struct {
	DB *gorm.DB
}

// 设备管理
func (uc *UserController) GetDevices(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var devices []models.Device
	if err := uc.DB.Where("user_id = ?", userID).Find(&devices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取设备列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": devices})
}

func (uc *UserController) CreateDevice(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req struct {
		Code string `json:"code" binding:"required,len=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "验证码格式错误"})
		return
	}

	// 验证设备验证码
	var existingDevice models.Device
	if err := uc.DB.Where("device_code = ? AND user_id IS NULL", req.Code).First(&existingDevice).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "验证码无效或设备已被绑定"})
		return
	}

	// 绑定设备到用户
	existingDevice.UserID = userID.(uint)
	if err := uc.DB.Save(&existingDevice).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "设备绑定失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": existingDevice})
}

func (uc *UserController) UpdateDevice(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id, _ := strconv.Atoi(c.Param("id"))

	var device models.Device
	if err := uc.DB.Where("id = ? AND user_id = ?", id, userID).First(&device).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "设备不存在"})
		return
	}

	if err := c.ShouldBindJSON(&device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := uc.DB.Save(&device).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新设备失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": device})
}

func (uc *UserController) DeleteDevice(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id, _ := strconv.Atoi(c.Param("id"))

	if err := uc.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&models.Device{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除设备失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// 智能体管理
func (uc *UserController) GetAgents(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var agents []models.Agent
	if err := uc.DB.Where("user_id = ?", userID).Find(&agents).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取智能体列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": agents})
}

func (uc *UserController) CreateAgent(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req struct {
		Name string `json:"name" binding:"required,min=2,max=50"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "智能体名称格式错误"})
		return
	}

	agent := models.Agent{
		UserID: userID.(uint),
		Name:   req.Name,
		Status: "active",
	}

	if err := uc.DB.Create(&agent).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建智能体失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": agent})
}

func (uc *UserController) GetAgent(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id, _ := strconv.Atoi(c.Param("id"))

	var agent models.Agent
	if err := uc.DB.Preload("LLMConfig").Preload("TTSConfig").Where("id = ? AND user_id = ?", id, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "智能体不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": agent})
}

func (uc *UserController) UpdateAgent(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := c.Param("id")

	var agent models.Agent
	if err := uc.DB.Where("id = ? AND user_id = ?", id, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "智能体不存在"})
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required,min=2,max=50"`
		Description string `json:"description"`
		Config      string `json:"config"`
		LLMConfigID *uint  `json:"llm_config_id"`
		TTSConfigID *uint  `json:"tts_config_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	agent.Name = req.Name
	agent.Description = req.Description
	agent.Config = req.Config
	agent.LLMConfigID = req.LLMConfigID
	agent.TTSConfigID = req.TTSConfigID

	if err := uc.DB.Save(&agent).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新智能体失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": agent})
}

func (uc *UserController) DeleteAgent(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := c.Param("id")

	var agent models.Agent
	if err := uc.DB.Where("id = ? AND user_id = ?", id, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "智能体不存在"})
		return
	}

	if err := uc.DB.Delete(&agent).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除智能体失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// 获取智能体关联的设备
func (uc *UserController) GetAgentDevices(c *gin.Context) {
	userID, _ := c.Get("user_id")
	agentID := c.Param("id")

	// 首先验证智能体是否存在且属于当前用户
	var agent models.Agent
	if err := uc.DB.Where("id = ? AND user_id = ?", agentID, userID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "智能体不存在"})
		return
	}

	// 获取用户的所有设备（因为当前模型中智能体和设备都属于用户）
	var devices []models.Device
	if err := uc.DB.Where("user_id = ?", userID).Find(&devices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取设备列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": devices})
}

// 获取角色模板
func (uc *UserController) GetRoleTemplates(c *gin.Context) {
	var roles []models.GlobalRole
	if err := uc.DB.Find(&roles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取角色模板失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": roles})
}

// 获取音色选项
func (uc *UserController) GetVoiceOptions(c *gin.Context) {
	var configs []models.Config
	if err := uc.DB.Where("type = ? AND enabled = ?", "tts", true).Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取音色选项失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": configs})
}

// 获取LLM配置列表
func (uc *UserController) GetLLMConfigs(c *gin.Context) {
	var configs []models.Config
	if err := uc.DB.Where("type = ? AND enabled = ?", "llm", true).Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取LLM配置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": configs})
}

// 获取TTS配置列表
func (uc *UserController) GetTTSConfigs(c *gin.Context) {
	var configs []models.Config
	if err := uc.DB.Where("type = ? AND enabled = ?", "tts", true).Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取TTS配置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": configs})
}
