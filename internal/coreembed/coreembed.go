package coreembed

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

//go:embed mihomo-windows-amd64-v3.exe
var embeddedFiles embed.FS

const embeddedCoreName = "mihomo-windows-amd64-v3.exe"

// ResolveCorePath 返回可执行内核路径：
// 1) 如果用户配置了 CorePath 且文件存在，优先使用用户配置；
// 2) 否则自动释放内嵌内核到本地缓存目录并返回该路径。
func ResolveCorePath(configuredPath string) (string, error) {
	if configuredPath != "" {
		if st, err := os.Stat(configuredPath); err == nil && !st.IsDir() {
			return configuredPath, nil
		}
	}

	if runtime.GOOS != "windows" {
		return "", errors.New("当前内嵌内核仅支持 Windows")
	}

	data, err := embeddedFiles.ReadFile(embeddedCoreName)
	if err != nil {
		return "", fmt.Errorf("读取内嵌内核失败: %w", err)
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("获取缓存目录失败: %w", err)
	}
	targetDir := filepath.Join(cacheDir, "node-latency", "core")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", fmt.Errorf("创建内核目录失败: %w", err)
	}
	targetPath := filepath.Join(targetDir, embeddedCoreName)

	needWrite := true
	if fi, err := os.Stat(targetPath); err == nil && !fi.IsDir() && fi.Size() == int64(len(data)) {
		needWrite = false
	}
	if needWrite {
		if err := os.WriteFile(targetPath, data, 0o755); err != nil {
			return "", fmt.Errorf("写入内嵌内核失败: %w", err)
		}
	}

	return targetPath, nil
}

