// Package updater 提供自动更新器的文件操作管理功能
package updater

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/sirupsen/logrus"
)

// FileManager 文件操作管理器
type FileManager struct{}

// NewFileManager 创建文件操作管理器
func NewFileManager() *FileManager {
	return &FileManager{}
}

// ReplaceExecutable 替换可执行文件
func (fm *FileManager) ReplaceExecutable(tmpFile string) error {
	// 获取当前程序路径
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取当前程序路径失败: %w", err)
	}

	// 替换程序（Windows 需要特殊处理）
	if runtime.GOOS == "windows" {
		// Windows 下使用重命名方式：先将当前exe改名为.old作为备份
		oldExe := currentExe + ".old"

		// 删除旧的.old文件（如果存在）
		os.Remove(oldExe)

		// 将当前exe重命名为.old（Windows允许重命名正在运行的exe）
		if err := os.Rename(currentExe, oldExe); err != nil {
			return fmt.Errorf("重命名当前程序失败: %w", err)
		}

		logrus.Info("已备份当前版本为 .old")

		// 将新文件移动到正确位置
		if err := fm.copyFile(tmpFile, currentExe); err != nil {
			// 如果失败，尝试恢复
			os.Rename(oldExe, currentExe)
			return fmt.Errorf("复制新程序失败: %w", err)
		}

		// 删除临时文件
		os.Remove(tmpFile)

		logrus.Info("程序文件已更新")
		return nil
	}

	// Linux/Mac 先备份再替换
	backupFile := currentExe + ".old"
	os.Remove(backupFile) // 删除旧备份
	if err := fm.copyFile(currentExe, backupFile); err != nil {
		return fmt.Errorf("备份失败: %w", err)
	}
	logrus.Info("已备份当前版本")

	if err := os.Rename(tmpFile, currentExe); err != nil {
		return fmt.Errorf("替换程序失败: %w", err)
	}

	return nil
}

// RestartProgram 重启程序
func (fm *FileManager) RestartProgram() {
	currentExe, err := os.Executable()
	if err != nil {
		logrus.Errorf("获取程序路径失败: %v", err)
		return
	}

	// 获取当前工作目录
	workDir, err := os.Getwd()
	if err != nil {
		logrus.Warnf("获取工作目录失败: %v", err)
		workDir = filepath.Dir(currentExe)
	}

	logrus.Info("准备启动新版本程序...")

	if runtime.GOOS == "windows" {
		// Windows: 使用 cmd /c start 在新窗口中启动程序
		// 构建参数
		args := []string{"/c", "start", "Task Processor", "/D", workDir, currentExe}
		args = append(args, os.Args[1:]...)

		cmd := exec.Command("cmd", args...)

		if err := cmd.Start(); err != nil {
			logrus.Errorf("启动新程序失败: %v", err)
			return
		}
	} else {
		// Linux/Mac: 直接启动
		cmd := exec.Command(currentExe, os.Args[1:]...)
		cmd.Dir = workDir

		if err := cmd.Start(); err != nil {
			logrus.Errorf("启动新程序失败: %v", err)
			return
		}
	}

	logrus.Info("新程序已启动，当前程序即将退出...")
	logrus.Info("提示: 旧版本已备份为 .old 文件，确认新版本正常后可手动删除")

	// 刷新日志缓冲区
	time.Sleep(500 * time.Millisecond)

	// 在 Windows 上使用 taskkill 强制终止当前进程
	if runtime.GOOS == "windows" {
		// 获取当前进程 PID
		pid := os.Getpid()
		logrus.Infof("强制终止当前进程 (PID: %d)...", pid)

		// 使用 taskkill 强制终止 - 添加panic recovery
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logrus.Errorf("强制终止进程goroutine panic recovered: %v", r)
				}
			}()
			time.Sleep(1 * time.Second)
			exec.Command("taskkill", "/F", "/PID", fmt.Sprintf("%d", pid)).Run()
		}()
	}

	// 正常退出
	time.Sleep(1 * time.Second)
	os.Exit(0)
}

// copyFile 复制文件
func (fm *FileManager) copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

// SaveUpdateError 保存更新错误到文件
func (fm *FileManager) SaveUpdateError(currentVersion, context string, err error) {
	errorLog := fmt.Sprintf(`========================================
Auto Update Error Log
========================================
Time: %s
Current Version: %s
Context: %s
Error: %v
========================================
`, time.Now().Format("2006-01-02 15:04:05"), currentVersion, context, err)

	// 保存到当前目录
	if err := os.WriteFile("update-error.log", []byte(errorLog), 0644); err != nil {
		logrus.Warnf("无法保存更新错误日志: %v", err)
	} else {
		logrus.Info("更新错误日志已保存到: update-error.log")
	}
}

// IsAlreadyUpdated 检查是否已经更新过指定版本
func (fm *FileManager) IsAlreadyUpdated(version string) bool {
	markerFile := ".update-marker"
	data, err := os.ReadFile(markerFile)
	if err != nil {
		return false
	}
	return string(data) == version
}

// IsRecentlyUpdated 检查是否最近刚更新过（1小时内）
func (fm *FileManager) IsRecentlyUpdated() bool {
	markerFile := ".update-marker"
	info, err := os.Stat(markerFile)
	if err != nil {
		return false
	}

	// 如果标记文件是1小时内创建的，认为是最近更新过
	timeSinceUpdate := time.Since(info.ModTime())
	if timeSinceUpdate < 1*time.Hour {
		logrus.Infof("检测到 %v 前更新过", timeSinceUpdate.Round(time.Second))
		return true
	}

	return false
}

// MarkAsUpdated 标记已更新到指定版本
func (fm *FileManager) MarkAsUpdated(version string) {
	markerFile := ".update-marker"
	content := fmt.Sprintf("%s\n%s", version, time.Now().Format(time.RFC3339))
	if err := os.WriteFile(markerFile, []byte(content), 0644); err != nil {
		logrus.Warnf("无法创建更新标记文件: %v", err)
	} else {
		logrus.Infof("已标记更新到版本: %s", version)
	}
}
