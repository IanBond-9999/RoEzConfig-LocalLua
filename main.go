package main

import (
	_ "embed"
	"fmt"
	"os/exec"
	"runtime"
	"syscall"

	"github.com/getlantern/systray"
)

// 隐藏控制台黑框（仅 Windows 生效） -- Ian
func hideConsole() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	user32 := syscall.NewLazyDLL("user32.dll")
	getConsoleWindow := kernel32.NewProc("GetConsoleWindow")
	showWindow := user32.NewProc("ShowWindow")
	hwnd, _, _ := getConsoleWindow.Call()
	if hwnd != 0 {
		const SW_HIDE = 0
		showWindow.Call(hwnd, SW_HIDE)
	}
}

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
	hideConsole()
	fmt.Println("RoEzConfig-LocalLua 启动中...")

	// 初始化配置 -- Ian
	if err := initConfig(); err != nil {
		fmt.Println("配置初始化失败:", err)
		return
	}
	fmt.Println("配置加载成功")

	// 启动 UI 服务 -- Ian
	go startUIServer()

	// 自动打开浏览器 -- Ian
	openBrowser("http://localhost:11451")

	// 启动系统托盘 -- Ian
	systray.Run(onTrayReady, onTrayExit)
}

// 托盘就绪时的回调 -- Ian
func onTrayReady() {
	systray.SetIcon(trayIcon)
	systray.SetTitle("RoEzConfig-LocalLua")
	systray.SetTooltip("RoEzConfig-LocalLua - LarkExcel2Local")

	// 打开界面菜单项 -- Ian
	mOpen := systray.AddMenuItem("打开界面", "在浏览器中打开管理界面")
	systray.AddSeparator()
	// 退出菜单项 -- Ian
	mQuit := systray.AddMenuItem("退出", "退出 RoEzConfig-LocalLua")

	go func() {
		for {
			select {
			case <-mOpen.ClickedCh:
				openBrowser("http://localhost:11451")
			case <-mQuit.ClickedCh:
				systray.Quit()
			}
		}
	}()
}

// 托盘退出时的回调 -- Ian
func onTrayExit() {
	fmt.Println("RoEzConfig-LocalLua 已退出")
}
