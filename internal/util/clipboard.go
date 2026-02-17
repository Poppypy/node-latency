package util

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func SetClipboardText(text string) error {
	switch runtime.GOOS {
	case "windows":
		psScript := "[Console]::InputEncoding=[System.Text.Encoding]::UTF8; $t=[Console]::In.ReadToEnd(); Set-Clipboard -Value $t"
		if err := pipeToCommandIfExists([]string{"powershell", "powershell.exe", "pwsh"},
			[]string{"-NoProfile", "-NonInteractive", "-Command", psScript}, text); err == nil {
			return nil
		}
		if err := pipeToCommandIfExists([]string{"cmd", "cmd.exe"}, []string{"/c", "chcp 65001 >NUL & clip"}, text); err == nil {
			return nil
		}
		if err := pipeToCommandIfExists([]string{"cmd", "cmd.exe"}, []string{"/c", "clip"}, text); err == nil {
			return nil
		}
		return errors.New("复制失败：无法调用 PowerShell/clip 写入剪贴板")
	case "darwin":
		if err := pipeToCommandIfExists([]string{"pbcopy"}, nil, text); err == nil {
			return nil
		}
		return nil
	default:
		if err := pipeToCommandIfExists([]string{"wl-copy"}, nil, text); err == nil {
			return nil
		}
		if err := pipeToCommandIfExists([]string{"xclip"}, []string{"-selection", "clipboard"}, text); err == nil {
			return nil
		}
		if err := pipeToCommandIfExists([]string{"xsel"}, []string{"--clipboard", "--input"}, text); err == nil {
			return nil
		}
		return nil
	}
}

func pipeToCommandIfExists(candidates []string, args []string, text string) error {
	for _, exe := range candidates {
		if exe == "" {
			continue
		}
		if _, err := exec.LookPath(exe); err != nil {
			continue
		}
		cmd := exec.Command(exe, args...)
		cmd.Stdin = strings.NewReader(text)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("%s: %s", exe, msg)
		}
		return fmt.Errorf("%s: %v", exe, err)
	}
	return errors.New("command not found")
}
