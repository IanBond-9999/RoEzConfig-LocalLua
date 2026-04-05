package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

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

// 执行单张表导入，将 Lua 文件写到本地指定路径 -- Ian
func importSheet(sheet SheetConfig) error {
	// 第一步：获取 sheet 列表 -- Ian
	sheets, err := feiShuClient.GetSheetInfo(sheet.Token)
	if err != nil {
		return fmt.Errorf("获取 sheet 列表失败: %w", err)
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
		return fmt.Errorf("获取数据失败: %w", err)
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

	// 第五步：写入本地 .lua 文件 -- Ian
	if err := os.MkdirAll(sheet.Path, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	filePath := filepath.Join(sheet.Path, sheet.FileName+".lua")
	return os.WriteFile(filePath, []byte(code), 0644)
}

// 启动给浏览器 UI 用的管理服务 -- Ian
func startUIServer() {
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

		if err := importSheet(*target); err != nil {
			writeError(w, err.Error())
			return
		}
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

		var errors []string
		for _, sheet := range sheets {
			if err := importSheet(sheet); err != nil {
				errors = append(errors, fmt.Sprintf("「%s」失败: %s", sheet.Name, err.Error()))
			}
		}

		writeJSON(w, map[string]any{
			"ok":     true,
			"errors": errors,
		})
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

	fmt.Println("UI 服务启动，监听 :11451")
	http.ListenAndServe(":11451", mux)
}
