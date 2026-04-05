package main

import (
	_ "embed"
)

// 嵌入 index.html，编译时自动打包进 exe -- Ian
//
//go:embed index.html
var htmlUI string
