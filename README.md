# RoEzConfig-LocalLua

将飞书电子表格一键转换为本地 Lua 配置文件的 Windows 桌面工具，专为使用 Rojo 工作流的 Roblox 开发者设计。

---

## 功能概述

- 从飞书电子表格拉取数据，自动转换为 Lua `ModuleScript` 格式并写入本地
- 支持单张导入和全部导入
- 本地 Web UI 管理所有配置，无需命令行操作
- 支持团队配置方案共享（导出 / 加载 `ExcelConfig.json`）
- 系统托盘常驻，无黑框窗口

---

## 快速上手

### 1. 准备飞书应用凭据

在 [飞书开放平台](https://open.feishu.cn) 创建一个**企业自建应用**，开启以下权限：

- `sheets:spreadsheet`（读取电子表格）

创建后记录 **App ID** 和 **App Secret**。

### 2. 运行程序

双击 `RoEzConfig-LocalLua.exe`，程序会：

1. 自动打开浏览器访问管理界面（`http://localhost:11451`）
2. 在 Windows 右下角托盘显示图标，无控制台黑框

> 若浏览器未自动打开，手动访问 `http://localhost:11451`

### 3. 填写凭据并测试连接

在「飞书设置」卡片中填入 App ID 和 App Secret，点击「测试连接」确认配置正确。

### 4. 添加配置表

点击「+ 添加」，填写：

| 字段 | 说明 |
|------|------|
| 名称 | 自定义标识，如 `怪物表` |
| 飞书 Token | 表格 URL 中的 `spreadsheetToken`，形如 `JzeysH5p1ang2ktu5sGcTOGFnLe` |
| 本地目录 | 输出文件夹的绝对路径，如 `C:\Users\pc\project\lua`（不存在时自动创建） |
| 文件名 | 生成的 `.lua` 文件名，不含扩展名，如 `MonsterConfig` |

### 5. 导入

- 点击单条配置右侧的「导入」按钮，仅导入该表
- 点击「全部导入」，批量处理所有配置表

导入完成后，指定目录下会生成或覆盖对应的 `.lua` 文件。

---

## 飞书表格格式约定

工具按以下约定解析表格数据：

| 行 | 含义 |
|----|------|
| 第 1 行 | 字段名（英文） |
| 第 2 行 | 中文备注（导入时跳过） |
| 第 3 行 | 字段类型：`number` 或 `string` |
| 第 4 行起 | 数据行，**第 1 列为主键** |

> 将备注列的字段名设为 `Note`，该列会被自动跳过，不写入 Lua 文件。

### 示例表格

| Id | Note | Name | Hp |
|----|------|------|----|
| id | 备注 | 名称 | 血量 |
| number | string | string | number |
| 1001 | 普通怪 | Slime | 100 |
| 1002 | 精英怪 | Dragon | 5000 |

### 生成的 Lua 代码

```lua
local GameData = {}

GameData.MonsterConfig = {
    [1001] = { Id = 1001, Name = "Slime", Hp = 100, },
    [1002] = { Id = 1002, Name = "Dragon", Hp = 5000, },
}

return GameData
```

---

## 团队协作

配置好所有表格后，点击「保存方案」，会在程序同目录生成 `ExcelConfig.json`。

将该文件分享给团队成员，成员将文件放在 exe 同目录后点击「加载方案」，即可一键同步所有配置（不含凭据，各自填写自己的 App ID / App Secret）。

---

## 托盘操作

| 操作 | 说明 |
|------|------|
| 双击托盘图标 / 点击「打开界面」 | 在浏览器中打开管理界面 |
| 点击「退出」 | 关闭程序，释放端口 |

---

## 构建

```bash
go build -o RoEzConfig-LocalLua.exe .
```

依赖：Go 1.20+，仅使用标准库和 `github.com/getlantern/systray`。

---

## 目录结构

```
RoEzConfig-LocalLua.exe   # 主程序
config.json               # 运行时自动生成，保存凭据和配置表列表
ExcelConfig.json          # 手动导出的团队共享方案（可选）
```
