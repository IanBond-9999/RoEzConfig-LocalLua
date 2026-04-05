package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// 飞书 API 地址 -- Ian
const (
	urlTenantAccessToken = "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal"
	urlSheets            = "https://open.feishu.cn/open-apis/sheets/v3/spreadsheets/%s/sheets/query"
	urlSheetRangeBatch   = "https://open.feishu.cn/open-apis/sheets/v2/spreadsheets/%s/values_batch_get"
)

// 飞书客户端 -- Ian
type FeiShuClient struct {
	mu          sync.Mutex
	accessToken string
	expireAt    time.Time
}

// 全局单例 -- Ian
var feiShuClient = &FeiShuClient{}

// 获取 tenant_access_token，过期前 30 分钟自动刷新 -- Ian
func (c *FeiShuClient) ensureToken() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// token 还有超过 30 分钟有效期，直接复用 -- Ian
	if c.accessToken != "" && time.Now().Add(30*time.Minute).Before(c.expireAt) {
		return nil
	}

	cfg := configManager.GetConfig()
	if cfg.AppID == "" || cfg.AppSecret == "" {
		return fmt.Errorf("飞书凭据未配置，请先填写 App ID 和 App Secret")
	}

	// 请求新 token -- Ian
	body, _ := json.Marshal(map[string]string{
		"app_id":     cfg.AppID,
		"app_secret": cfg.AppSecret,
	})

	resp, err := http.Post(urlTenantAccessToken, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("网络请求失败: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("响应解析失败: %w", err)
	}

	if code, ok := result["code"].(float64); ok && code != 0 {
		return fmt.Errorf("飞书 API 错误 [%d]: %s", int(code), result["msg"])
	}

	c.accessToken = result["tenant_access_token"].(string)
	expire := int(result["expire"].(float64))
	c.expireAt = time.Now().Add(time.Duration(expire) * time.Second)

	return nil
}

// 发送带鉴权的 GET 请求 -- Ian
func (c *FeiShuClient) get(url string) (map[string]any, error) {
	if err := c.ensureToken(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}

	return result, nil
}

// 获取表格的 sheet 列表 -- Ian
func (c *FeiShuClient) GetSheetInfo(spreadsheetToken string) ([]map[string]any, error) {
	url := fmt.Sprintf(urlSheets, spreadsheetToken)
	result, err := c.get(url)
	if err != nil {
		return nil, err
	}

	// 提取 sheets 列表 -- Ian
	data, ok := result["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("返回数据格式异常")
	}
	sheets, ok := data["sheets"].([]any)
	if !ok || len(sheets) == 0 {
		return nil, fmt.Errorf("没有可用的 sheet")
	}

	var result2 []map[string]any
	for _, s := range sheets {
		if m, ok := s.(map[string]any); ok {
			result2 = append(result2, m)
		}
	}
	return result2, nil
}

// 批量获取多个 sheet 的数据 -- Ian
func (c *FeiShuClient) GetSheetRangeData(spreadsheetToken string, sheetIDs []string) ([]map[string]any, error) {
	// 用逗号拼接所有 sheetID，和原插件保持一致 -- Ian
	url := fmt.Sprintf(urlSheetRangeBatch, spreadsheetToken)
	url += "?valueRenderOption=ToString&ranges=" + strings.Join(sheetIDs, ",")

	result, err := c.get(url)
	if err != nil {
		return nil, err
	}

	data, ok := result["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("返回数据格式异常")
	}
	valueRanges, ok := data["valueRanges"].([]any)
	if !ok {
		return nil, fmt.Errorf("未找到 valueRanges")
	}

	var result2 []map[string]any
	for _, v := range valueRanges {
		if m, ok := v.(map[string]any); ok {
			result2 = append(result2, m)
		}
	}
	return result2, nil
}

// 测试连接：强制重新获取 token -- Ian
func (c *FeiShuClient) TestConnection() error {
	c.mu.Lock()
	c.accessToken = ""
	c.mu.Unlock()
	return c.ensureToken()
}
