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
func importSheet(project Project, sheet SheetConfig) error {
	// 第一步：获取 sheet 列表 -- Ian
	sheets, err := feiShuClient.GetSheetInfo(project.AppID, project.AppSecret, sheet.Token)
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
	valueRanges, err := feiShuClient.GetSheetRangeData(project.AppID, project.AppSecret, sheet.Token, sheetIDs)
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

	// === 项目管理 API === -- Ian

	// 添加项目 -- Ian
	mux.HandleFunc("/api/projects/add", func(w http.ResponseWriter, r *http.Request) {
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
		if body.Name == "" {
			writeError(w, "项目名称不能为空")
			return
		}
		if err := configManager.AddProject(body.Name); err != nil {
			writeError(w, err.Error())
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})

	// 重命名项目 -- Ian
	mux.HandleFunc("/api/projects/rename", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			writeError(w, "Method not allowed")
			return
		}
		var body struct {
			OldName string `json:"oldName"`
			NewName string `json:"newName"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, "请求格式错误")
			return
		}
		if err := configManager.RenameProject(body.OldName, body.NewName); err != nil {
			writeError(w, err.Error())
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})

	// 删除项目 -- Ian
	mux.HandleFunc("/api/projects/delete", func(w http.ResponseWriter, r *http.Request) {
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
		if err := configManager.DeleteProject(body.Name); err != nil {
			writeError(w, err.Error())
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})

	// === 项目内操作 API（均需 project 参数） === -- Ian

	// 保存凭据 -- Ian
	mux.HandleFunc("/api/credentials", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			writeError(w, "Method not allowed")
			return
		}
		var body struct {
			Project   string `json:"project"`
			AppID     string `json:"appId"`
			AppSecret string `json:"appSecret"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, "请求格式错误")
			return
		}
		if err := configManager.SaveCredentials(body.Project, body.AppID, body.AppSecret); err != nil {
			writeError(w, err.Error())
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})

	// 测试连接 -- Ian
	mux.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			writeError(w, "Method not allowed")
			return
		}
		var body struct {
			Project string `json:"project"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, "请求格式错误")
			return
		}
		proj := configManager.GetProject(body.Project)
		if proj == nil {
			writeError(w, fmt.Sprintf("找不到项目「%s」", body.Project))
			return
		}
		if err := feiShuClient.TestConnection(proj.AppID, proj.AppSecret); err != nil {
			writeError(w, err.Error())
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})

	// 获取配置表列表 -- Ian
	mux.HandleFunc("/api/sheets", func(w http.ResponseWriter, r *http.Request) {
		project := r.URL.Query().Get("project")
		sheets := configManager.GetSheets(project)
		if sheets == nil {
			sheets = []SheetConfig{}
		}
		writeJSON(w, sheets)
	})

	// 添加配置表 -- Ian
	mux.HandleFunc("/api/sheets/add", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			writeError(w, "Method not allowed")
			return
		}
		var body struct {
			Project string      `json:"project"`
			Sheet   SheetConfig `json:"sheet"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, "请求格式错误")
			return
		}
		if err := configManager.AddSheet(body.Project, body.Sheet); err != nil {
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
			Project      string      `json:"project"`
			OriginalName string      `json:"originalName"`
			Sheet        SheetConfig `json:"sheet"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, "请求格式错误")
			return
		}
		if err := configManager.UpdateSheet(body.Project, body.OriginalName, body.Sheet); err != nil {
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
			Project string `json:"project"`
			Name    string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, "请求格式错误")
			return
		}
		if err := configManager.DeleteSheet(body.Project, body.Name); err != nil {
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
			Project string `json:"project"`
			Name    string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, "请求格式错误")
			return
		}

		proj := configManager.GetProject(body.Project)
		if proj == nil {
			writeError(w, fmt.Sprintf("找不到项目「%s」", body.Project))
			return
		}

		// 找到对应配置 -- Ian
		var target *SheetConfig
		for _, s := range proj.Sheets {
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

		if err := importSheet(*proj, *target); err != nil {
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
		var body struct {
			Project string `json:"project"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, "请求格式错误")
			return
		}

		proj := configManager.GetProject(body.Project)
		if proj == nil {
			writeError(w, fmt.Sprintf("找不到项目「%s」", body.Project))
			return
		}

		if len(proj.Sheets) == 0 {
			writeError(w, "没有配置表可导入")
			return
		}

		var errors []string
		for _, sheet := range proj.Sheets {
			if err := importSheet(*proj, sheet); err != nil {
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

	// 清除所有配置 -- Ian
	mux.HandleFunc("/api/clear", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			writeError(w, "Method not allowed")
			return
		}
		configManager.mu.Lock()
		configManager.config = Config{Projects: []Project{}}
		configManager.mu.Unlock()
		if err := configManager.save(); err != nil {
			writeError(w, "清除失败: "+err.Error())
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})

	fmt.Println("UI 服务启动，监听 :11451")
	http.ListenAndServe(":11451", mux)
}
