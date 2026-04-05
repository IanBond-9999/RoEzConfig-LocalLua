package main

import (
	_ "embed"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/getlantern/systray"
)

// 嵌入托盘图标 -- Ian
//
//go:embed icon.ico
var trayIcon []byte

// 用默认浏览器打开指定 URL -- Ian
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}

func main() {
	fmt.Println("RoEzConfig - LarkExcel2Studio 启动中...")

	// 初始化配置 -- Ian
	if err := initConfig(); err != nil {
		fmt.Println("配置初始化失败:", err)
		return
	}
	fmt.Println("配置加载成功")

	cfg := configManager.GetConfig()

	// 启动插件轮询服务 -- Ian
	go startPluginServer(cfg.Port)

	// 启动 UI 服务 -- Ian
	go startUIServer(cfg.Port)

	// 自动打开浏览器 -- Ian
	uiURL := fmt.Sprintf("http://localhost:%d", cfg.Port+1)
	openBrowser(uiURL)

	// 启动系统托盘 -- Ian
	systray.Run(onTrayReady, onTrayExit)
}

// 托盘就绪时的回调 -- Ian
func onTrayReady() {
	systray.SetIcon(trayIcon)
	systray.SetTitle("RoEzConfig")
	systray.SetTooltip("RoEzConfig - LarkExcel2Studio")

	// 打开界面菜单项 -- Ian
	mOpen := systray.AddMenuItem("打开界面", "在浏览器中打开管理界面")
	systray.AddSeparator()
	// 退出菜单项 -- Ian
	mQuit := systray.AddMenuItem("退出", "退出 RoEzConfig")

	go func() {
		for {
			select {
			case <-mOpen.ClickedCh:
				cfg := configManager.GetConfig()
				uiURL := fmt.Sprintf("http://localhost:%d", cfg.Port+1)
				openBrowser(uiURL)
			case <-mQuit.ClickedCh:
				systray.Quit()
			}
		}
	}()
}

// 托盘退出时的回调 -- Ian
func onTrayExit() {
	fmt.Println("RoEzConfig 已退出")
}
