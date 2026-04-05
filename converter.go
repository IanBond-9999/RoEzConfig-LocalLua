package main

import (
	"fmt"
	"strings"
)

// 转义 Lua 字符串中的特殊字符 -- Ian
func escapeLuaString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	return s
}

// 将单个 sheet 的数据转换为 Lua 代码片段 -- Ian
// 表格约定格式：
//
//	第 1 行：字段名
//	第 2 行：中文备注（跳过）
//	第 3 行：字段类型（number / string 等）
//	第 4 行起：数据行，第 1 列为主键
//	列名为 "Note" 的列跳过不导入
func convertSheet(sheetTitle string, values [][]any) (string, error) {
	if len(values) < 3 {
		return "", fmt.Errorf("「%s」数据不足 3 行，无法解析表头和类型", sheetTitle)
	}

	// 提取表头和类型行 -- Ian
	headers := values[0]
	valueTypes := values[2]

	var output strings.Builder
	output.WriteString(fmt.Sprintf("GameData.%s = {", sheetTitle))

	// 从第 4 行开始遍历数据 -- Ian
	for i := 3; i < len(values); i++ {
		row := values[i]
		if len(row) == 0 || row[0] == nil {
			continue
		}

		var line strings.Builder

		// 主键格式：number 用 [123]，string 用 ["key"] -- Ian
		keyType := ""
		if len(valueTypes) > 0 {
			keyType = fmt.Sprintf("%v", valueTypes[0])
		}
		if keyType == "number" {
			line.WriteString(fmt.Sprintf("\n\t[%v] = {", row[0]))
		} else {
			line.WriteString(fmt.Sprintf("\n\t[\"%s\"] = {", escapeLuaString(fmt.Sprintf("%v", row[0]))))
		}

		// 遍历每一列 -- Ian
		for j := 0; j < len(headers); j++ {
			fieldName := fmt.Sprintf("%v", headers[j])

			// 跳过 Note 列 -- Ian
			if fieldName == "Note" {
				continue
			}

			var fieldValue any
			if j < len(row) {
				fieldValue = row[j]
			}

			fieldType := ""
			if j < len(valueTypes) {
				fieldType = fmt.Sprintf("%v", valueTypes[j])
			}

			// 格式化值 -- Ian
			var formatted string
			if fieldValue == nil || fmt.Sprintf("%v", fieldValue) == "nil" {
				formatted = "nil"
			} else if fieldType == "string" {
				formatted = fmt.Sprintf(`"%s"`, escapeLuaString(fmt.Sprintf("%v", fieldValue)))
			} else {
				// 数字类型：避免科学计数法，尝试格式化为整数 -- Ian
				if f, ok := fieldValue.(float64); ok {
					if f == float64(int64(f)) {
						formatted = fmt.Sprintf("%d", int64(f))
					} else {
						formatted = strings.TrimRight(fmt.Sprintf("%f", f), "0")
						formatted = strings.TrimRight(formatted, ".")
					}
				} else {
					formatted = fmt.Sprintf("%v", fieldValue)
				}
			}

			line.WriteString(fmt.Sprintf(" %s = %s,", fieldName, formatted))
		}

		line.WriteString(" },")
		output.WriteString(line.String())
	}

	output.WriteString("\n}")
	return output.String(), nil
}

// 将多个 sheet 转换为完整的 Lua ModuleScript 源码 -- Ian
func convertAll(sheets []SheetInfo) string {
	var output strings.Builder
	output.WriteString("local GameData = {}")

	for _, sheet := range sheets {
		output.WriteString("\n")
		code, err := convertSheet(sheet.Title, sheet.Values)
		if err != nil {
			output.WriteString(fmt.Sprintf("\n-- [错误] %s", err.Error()))
		} else {
			output.WriteString("\n" + code)
		}
	}

	output.WriteString("\n\nreturn GameData")
	return output.String()
}

// sheet 信息结构 -- Ian
type SheetInfo struct {
	Title  string
	Values [][]any
}
