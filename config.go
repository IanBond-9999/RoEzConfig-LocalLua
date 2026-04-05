package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// 单条配置表信息 -- Ian
type SheetConfig struct {
	Name     string `json:"name"`     // 配置表名称
	Token    string `json:"token"`    // 飞书 spreadsheetToken
	Path     string `json:"path"`     // Studio 工程目标路径
	FileName string `json:"fileName"` // 生成的 ModuleScript 名称
}

// 全局配置结构 -- Ian
type Config struct {
	AppID      string        `json:"appId"`      // 飞书 App ID
	AppSecret  string        `json:"appSecret"`  // 飞书 App Secret
	Port       int           `json:"port"`       // 插件轮询端口
	Sheets     []SheetConfig `json:"sheets"`     // 配置表列表
}

// 配置管理器 -- Ian
type ConfigManager struct {
	mu       sync.RWMutex // 读写锁，防止并发冲突
	config   Config
	filePath string
}

// 全局单例 -- Ian
var configManager *ConfigManager

// 初始化配置管理器，自动寻找 config.json -- Ian
func initConfig() error {
	// 获取 exe 所在目录 -- Ian
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	dir := filepath.Dir(exePath)
	filePath := filepath.Join(dir, "config.json")

	configManager = &ConfigManager{
		filePath: filePath,
		config: Config{
			Port:   11451,        // 默认端口 -- Ian
			Sheets: []SheetConfig{},
		},
	}

	// 如果文件存在则读取，不存在则使用默认值 -- Ian
	if _, err := os.Stat(filePath); err == nil {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, &configManager.config); err != nil {
			return err
		}
	}

	return nil
}

// 将当前配置写入 config.json -- Ian
func (cm *ConfigManager) save() error {
	cm.mu.RLock()
	data, err := json.MarshalIndent(cm.config, "", "  ")
	cm.mu.RUnlock()
	if err != nil {
		return err
	}
	return os.WriteFile(cm.filePath, data, 0644)
}

// 获取完整配置 -- Ian
func (cm *ConfigManager) GetConfig() Config {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.config
}

// 更新飞书凭据 -- Ian
func (cm *ConfigManager) SaveCredentials(appId, appSecret string) error {
	cm.mu.Lock()
	cm.config.AppID = appId
	cm.config.AppSecret = appSecret
	cm.mu.Unlock()
	return cm.save()
}

// 更新端口 -- Ian
func (cm *ConfigManager) SavePort(port int) error {
	cm.mu.Lock()
	cm.config.Port = port
	cm.mu.Unlock()
	return cm.save()
}

// 获取所有配置表 -- Ian
func (cm *ConfigManager) GetSheets() []SheetConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	result := make([]SheetConfig, len(cm.config.Sheets))
	copy(result, cm.config.Sheets)
	return result
}

// 添加配置表，名称不能重复 -- Ian
func (cm *ConfigManager) AddSheet(sheet SheetConfig) error {
	cm.mu.Lock()
	for _, s := range cm.config.Sheets {
		if s.Name == sheet.Name {
			cm.mu.Unlock()
			return fmt.Errorf("已存在同名配置「%s」", sheet.Name)
		}
	}
	cm.config.Sheets = append(cm.config.Sheets, sheet)
	cm.mu.Unlock()
	return cm.save()
}

// 更新配置表 -- Ian
func (cm *ConfigManager) UpdateSheet(originalName string, newSheet SheetConfig) error {
	cm.mu.Lock()
	for i, s := range cm.config.Sheets {
		if s.Name == originalName {
			cm.config.Sheets[i] = newSheet
			cm.mu.Unlock()
			return cm.save()
		}
	}
	cm.mu.Unlock()
	return fmt.Errorf("找不到配置「%s」", originalName)
}

// 删除配置表 -- Ian
func (cm *ConfigManager) DeleteSheet(name string) error {
	cm.mu.Lock()
	for i, s := range cm.config.Sheets {
		if s.Name == name {
			cm.config.Sheets = append(cm.config.Sheets[:i], cm.config.Sheets[i+1:]...)
			cm.mu.Unlock()
			return cm.save()
		}
	}
	cm.mu.Unlock()
	return fmt.Errorf("找不到配置「%s」", name)
}