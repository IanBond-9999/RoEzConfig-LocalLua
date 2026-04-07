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
	Path     string `json:"path"`     // 本地文件夹路径（绝对路径）
	FileName string `json:"fileName"` // 生成的 .lua 文件名（不含扩展名）
}

// 项目配置，每个页签对应一个项目 -- Ian
type Project struct {
	Name      string        `json:"name"`      // 项目名称
	AppID     string        `json:"appId"`     // 飞书 App ID
	AppSecret string        `json:"appSecret"` // 飞书 App Secret
	Sheets    []SheetConfig `json:"sheets"`    // 配置表列表
}

// 全局配置结构 -- Ian
type Config struct {
	Projects []Project `json:"projects"` // 项目列表
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
			Projects: []Project{},
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

// 查找项目索引，返回 -1 表示未找到（调用方需持锁） -- Ian
func (cm *ConfigManager) findProject(name string) int {
	for i, p := range cm.config.Projects {
		if p.Name == name {
			return i
		}
	}
	return -1
}

// 获取项目 -- Ian
func (cm *ConfigManager) GetProject(name string) *Project {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	idx := cm.findProject(name)
	if idx < 0 {
		return nil
	}
	p := cm.config.Projects[idx]
	return &p
}

// 添加项目 -- Ian
func (cm *ConfigManager) AddProject(name string) error {
	cm.mu.Lock()
	if cm.findProject(name) >= 0 {
		cm.mu.Unlock()
		return fmt.Errorf("已存在同名项目「%s」", name)
	}
	cm.config.Projects = append(cm.config.Projects, Project{
		Name:   name,
		Sheets: []SheetConfig{},
	})
	cm.mu.Unlock()
	return cm.save()
}

// 重命名项目 -- Ian
func (cm *ConfigManager) RenameProject(oldName, newName string) error {
	cm.mu.Lock()
	if cm.findProject(newName) >= 0 {
		cm.mu.Unlock()
		return fmt.Errorf("已存在同名项目「%s」", newName)
	}
	idx := cm.findProject(oldName)
	if idx < 0 {
		cm.mu.Unlock()
		return fmt.Errorf("找不到项目「%s」", oldName)
	}
	cm.config.Projects[idx].Name = newName
	cm.mu.Unlock()
	return cm.save()
}

// 删除项目 -- Ian
func (cm *ConfigManager) DeleteProject(name string) error {
	cm.mu.Lock()
	idx := cm.findProject(name)
	if idx < 0 {
		cm.mu.Unlock()
		return fmt.Errorf("找不到项目「%s」", name)
	}
	cm.config.Projects = append(cm.config.Projects[:idx], cm.config.Projects[idx+1:]...)
	cm.mu.Unlock()
	return cm.save()
}

// 更新飞书凭据 -- Ian
func (cm *ConfigManager) SaveCredentials(projectName, appId, appSecret string) error {
	cm.mu.Lock()
	idx := cm.findProject(projectName)
	if idx < 0 {
		cm.mu.Unlock()
		return fmt.Errorf("找不到项目「%s」", projectName)
	}
	cm.config.Projects[idx].AppID = appId
	cm.config.Projects[idx].AppSecret = appSecret
	cm.mu.Unlock()
	return cm.save()
}

// 获取项目的所有配置表 -- Ian
func (cm *ConfigManager) GetSheets(projectName string) []SheetConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	idx := cm.findProject(projectName)
	if idx < 0 {
		return nil
	}
	result := make([]SheetConfig, len(cm.config.Projects[idx].Sheets))
	copy(result, cm.config.Projects[idx].Sheets)
	return result
}

// 添加配置表，名称不能重复 -- Ian
func (cm *ConfigManager) AddSheet(projectName string, sheet SheetConfig) error {
	cm.mu.Lock()
	idx := cm.findProject(projectName)
	if idx < 0 {
		cm.mu.Unlock()
		return fmt.Errorf("找不到项目「%s」", projectName)
	}
	for _, s := range cm.config.Projects[idx].Sheets {
		if s.Name == sheet.Name {
			cm.mu.Unlock()
			return fmt.Errorf("已存在同名配置「%s」", sheet.Name)
		}
	}
	cm.config.Projects[idx].Sheets = append(cm.config.Projects[idx].Sheets, sheet)
	cm.mu.Unlock()
	return cm.save()
}

// 更新配置表 -- Ian
func (cm *ConfigManager) UpdateSheet(projectName, originalName string, newSheet SheetConfig) error {
	cm.mu.Lock()
	idx := cm.findProject(projectName)
	if idx < 0 {
		cm.mu.Unlock()
		return fmt.Errorf("找不到项目「%s」", projectName)
	}
	for i, s := range cm.config.Projects[idx].Sheets {
		if s.Name == originalName {
			cm.config.Projects[idx].Sheets[i] = newSheet
			cm.mu.Unlock()
			return cm.save()
		}
	}
	cm.mu.Unlock()
	return fmt.Errorf("找不到配置「%s」", originalName)
}

// 删除配置表 -- Ian
func (cm *ConfigManager) DeleteSheet(projectName, name string) error {
	cm.mu.Lock()
	idx := cm.findProject(projectName)
	if idx < 0 {
		cm.mu.Unlock()
		return fmt.Errorf("找不到项目「%s」", projectName)
	}
	sheets := cm.config.Projects[idx].Sheets
	for i, s := range sheets {
		if s.Name == name {
			cm.config.Projects[idx].Sheets = append(sheets[:i], sheets[i+1:]...)
			cm.mu.Unlock()
			return cm.save()
		}
	}
	cm.mu.Unlock()
	return fmt.Errorf("找不到配置「%s」", name)
}
