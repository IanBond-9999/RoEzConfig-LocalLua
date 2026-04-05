package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

// 写入结果记录 -- Ian
type WriteResult struct {
	Done    bool     `json:"done"`
	Success int      `json:"success"`
	Fail    int      `json:"fail"`
	Errors  []string `json:"errors"`
}

var (
	lastResult *WriteResult
	resultMu   sync.Mutex
)

// 待推送给 Studio 插件的任务队列 -- Ian
type PendingTask struct {
	Path     string `json:"path"`
	FileName string `json:"fileName"`
	Code     string `json:"code"`
}

var (
	pendingTasks []PendingTask
	taskMu       sync.Mutex
)

// 添加待推送任务 -- Ian
func addPendingTasks(tasks []PendingTask) {
	taskMu.Lock()
	defer taskMu.Unlock()
	pendingTasks = append(pendingTasks, tasks...)
}

// 清空待推送任务 -- Ian
func clearPendingTasks() {
	taskMu.Lock()
	defer taskMu.Unlock()
	pendingTasks = nil
}

// 统一的 JSON 响应helper -- Ian
func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(data)
}

// 统一的错误响应 -- Ian
func writeError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// 执行单张表导入，返回任务列表 -- Ian
func importSheet(sheet SheetConfig) ([]PendingTask, error) {
	// 第一步：获取 sheet 列表 -- Ian
	sheets, err := feiShuClient.GetSheetInfo(sheet.Token)
	if err != nil {
		return nil, fmt.Errorf("获取 sheet 列表失败: %w", err)
	}

	// 第二步：收集 sheetID 列表 -- Ian
	var sheetIDs []string
	for _, s := range sheets {
		if id, ok := s["sheet_id"].(string); ok {
			sheetIDs = append(sheetIDs, id)
		}
	}

	// 第三步：批量获取数据 -- Ian
	valueRanges, err := feiShuClient.GetSheetRangeData(sheet.Token, sheetIDs)
	if err != nil {
		return nil, fmt.Errorf("获取数据失败: %w", err)
	}

	// 第四步：转换数据 -- Ian
	var sheetInfos []SheetInfo
	for i, vr := range valueRanges {
		title := ""
		if i < len(sheets) {
			if t, ok := sheets[i]["title"].(string); ok {
				title = t
			}
		}
		if title == "" {
			title = fmt.Sprintf("Sheet%d", i+1)
		}

		// 提取 values -- Ian
		var values [][]any
		if v, ok := vr["values"].([]any); ok {
			for _, row := range v {
				if r, ok := row.([]any); ok {
					values = append(values, r)
				}
			}
		}

		sheetInfos = append(sheetInfos, SheetInfo{
			Title:  title,
			Values: values,
		})
	}

	code := convertAll(sheetInfos)

	return []PendingTask{{
		Path:     sheet.Path,
		FileName: sheet.FileName,
		Code:     code,
	}}, nil
}

// 启动给 Studio 插件用的轮询服务 -- Ian
func startPluginServer(port int) {
	mux := http.NewServeMux()

	// 插件轮询接口 -- Ian
	mux.HandleFunc("/poll", func(w http.ResponseWriter, r *http.Request) {
		taskMu.Lock()
		tasks := pendingTasks
		taskMu.Unlock()

		if len(tasks) == 0 {
			writeJSON(w, map[string]any{"hasUpdate": false})
			return
		}

		writeJSON(w, map[string]any{
			"hasUpdate": true,
			"tasks":     tasks,
		})
	})

	// 插件确认写入完成，记录结果 -- Ian
	mux.HandleFunc("/ack", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Success int      `json:"success"`
			Fail    int      `json:"fail"`
			Errors  []string `json:"errors"`
		}
		json.NewDecoder(r.Body).Decode(&body)

		resultMu.Lock()
		lastResult = &WriteResult{
			Done:    true,
			Success: body.Success,
			Fail:    body.Fail,
			Errors:  body.Errors,
		}
		resultMu.Unlock()

		clearPendingTasks()
		writeJSON(w, map[string]any{"ok": true})
	})

	// 网页查询写入结果 -- Ian
	mux.HandleFunc("/result", func(w http.ResponseWriter, r *http.Request) {
		resultMu.Lock()
		result := lastResult
		if result != nil {
			lastResult = nil // 读取后清空 -- Ian
		}
		resultMu.Unlock()

		if result == nil {
			writeJSON(w, map[string]any{"done": false})
			return
		}
		writeJSON(w, result)
	})

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("插件轮询服务启动，监听 %s\n", addr)
	http.ListenAndServe(addr, mux)
}

// 启动给浏览器 UI 用的管理服务 -- Ian
func startUIServer(port int) {
	mux := http.NewServeMux()

	// 获取所有配置 -- Ian
	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, configManager.GetConfig())
	})

	// 保存凭据 -- Ian
	mux.HandleFunc("/api/credentials", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			writeError(w, "Method not allowed")
			return
		}
		var body struct {
			AppID     string `json:"appId"`
			AppSecret string `json:"appSecret"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, "请求格式错误")
			return
		}
		if err := configManager.SaveCredentials(body.AppID, body.AppSecret); err != nil {
			writeError(w, err.Error())
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})

	// 测试连接 -- Ian
	mux.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
		if err := feiShuClient.TestConnection(); err != nil {
			writeError(w, err.Error())
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})

	// 获取配置表列表 -- Ian
	mux.HandleFunc("/api/sheets", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, configManager.GetSheets())
	})

	// 添加配置表 -- Ian
	mux.HandleFunc("/api/sheets/add", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			writeError(w, "Method not allowed")
			return
		}
		var sheet SheetConfig
		if err := json.NewDecoder(r.Body).Decode(&sheet); err != nil {
			writeError(w, "请求格式错误")
			return
		}
		if err := configManager.AddSheet(sheet); err != nil {
			writeError(w, err.Error())
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})

	// 更新配置表 -- Ian
	mux.HandleFunc("/api/sheets/update", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			writeError(w, "Method not allowed")
			return
		}
		var body struct {
			OriginalName string      `json:"originalName"`
			Sheet        SheetConfig `json:"sheet"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, "请求格式错误")
			return
		}
		if err := configManager.UpdateSheet(body.OriginalName, body.Sheet); err != nil {
			writeError(w, err.Error())
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})

	// 删除配置表 -- Ian
	mux.HandleFunc("/api/sheets/delete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			writeError(w, "Method not allowed")
			return
		}
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, "请求格式错误")
			return
		}
		if err := configManager.DeleteSheet(body.Name); err != nil {
			writeError(w, err.Error())
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})

	// 导入单张表 -- Ian
	mux.HandleFunc("/api/import", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			writeError(w, "Method not allowed")
			return
		}
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, "请求格式错误")
			return
		}

		// 找到对应配置 -- Ian
		var target *SheetConfig
		for _, s := range configManager.GetSheets() {
			if s.Name == body.Name {
				s := s
				target = &s
				break
			}
		}
		if target == nil {
			writeError(w, fmt.Sprintf("找不到配置「%s」", body.Name))
			return
		}

		tasks, err := importSheet(*target)
		if err != nil {
			writeError(w, err.Error())
			return
		}

		clearPendingTasks()
		addPendingTasks(tasks)
		writeJSON(w, map[string]any{"ok": true})
	})

	// 全部导入 -- Ian
	mux.HandleFunc("/api/import/all", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			writeError(w, "Method not allowed")
			return
		}

		sheets := configManager.GetSheets()
		if len(sheets) == 0 {
			writeError(w, "没有配置表可导入")
			return
		}

		var allTasks []PendingTask
		var errors []string

		for _, sheet := range sheets {
			tasks, err := importSheet(sheet)
			if err != nil {
				errors = append(errors, fmt.Sprintf("「%s」失败: %s", sheet.Name, err.Error()))
			} else {
				allTasks = append(allTasks, tasks...)
			}
		}

		if len(allTasks) > 0 {
			clearPendingTasks()
			addPendingTasks(allTasks)
		}

		writeJSON(w, map[string]any{
			"ok":     true,
			"errors": errors,
		})
	})

	// 保存端口设置 -- Ian
	mux.HandleFunc("/api/port", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			writeError(w, "Method not allowed")
			return
		}
		var body struct {
			Port int `json:"port"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, "请求格式错误")
			return
		}
		if err := configManager.SavePort(body.Port); err != nil {
			writeError(w, err.Error())
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, htmlUI)
	})

	// 导出方案到 ExcelConfig.json -- Ian
	mux.HandleFunc("/api/export", func(w http.ResponseWriter, r *http.Request) {
		cfg := configManager.GetConfig()
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			writeError(w, "导出失败: "+err.Error())
			return
		}
		exportPath := filepath.Join(filepath.Dir(configManager.filePath), "ExcelConfig.json")
		if err := os.WriteFile(exportPath, data, 0644); err != nil {
			writeError(w, "写入文件失败: "+err.Error())
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})

	// 从 ExcelConfig.json 加载方案 -- Ian
	mux.HandleFunc("/api/load", func(w http.ResponseWriter, r *http.Request) {
		exportPath := filepath.Join(filepath.Dir(configManager.filePath), "ExcelConfig.json")
		data, err := os.ReadFile(exportPath)
		if err != nil {
			writeError(w, "读取文件失败，请确认 ExcelConfig.json 存在: "+err.Error())
			return
		}
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			writeError(w, "文件格式错误: "+err.Error())
			return
		}
		// 覆盖当前配置 -- Ian
		configManager.mu.Lock()
		configManager.config = cfg
		configManager.mu.Unlock()
		if err := configManager.save(); err != nil {
			writeError(w, "保存失败: "+err.Error())
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})

	addr := fmt.Sprintf(":%d", port+1)
	fmt.Printf("UI 服务启动，监听 %s\n", addr)
	http.ListenAndServe(addr, mux)
}
