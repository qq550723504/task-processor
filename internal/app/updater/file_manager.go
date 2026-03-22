// Package updater 提供自动更新器的文件操作管理功能
package updater

import (
	"task-processor/internal/core/logger"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

)

// FileManager 文件操作管理器
type FileManager struct{}

// NewFileManager 创建文件操作管理器
func NewFileManager() *FileManager {
	return &FileManager{}
}

// ReplaceExecutable 替换可执行文件
func (fm *FileManager) ReplaceExecutable(tmpFile string, newVersion string) error {
	// 获取当前程序路径
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取当前程序路径失败: %w", err)
	}

	// 替换程序（Windows 需要特殊处理）
	if runtime.GOOS == "windows" {
		// Windows 下使用延迟替换策略：
		// 1. 将新文件复制到临时位置
		// 2. 使用 Go 代码进行延迟替换和重启

		// 生成新版本的文件名
		dir := filepath.Dir(currentExe)
		baseName := filepath.Base(currentExe)

		// 提取基础名称（去掉版本号和扩展名）
		// 例如：task-processor-2.8.8.exe -> task-processor
		nameWithoutExt := strings.TrimSuffix(baseName, filepath.Ext(baseName))
		parts := strings.Split(nameWithoutExt, "-")
		var baseNameOnly string
		if len(parts) >= 2 {
			// 假设格式是 task-processor-x.x.x，取前两部分
			baseNameOnly = strings.Join(parts[:2], "-")
		} else {
			baseNameOnly = nameWithoutExt
		}

		// 生成新版本文件名
		newExeName := fmt.Sprintf("%s-%s.exe", baseNameOnly, newVersion)
		newExePath := filepath.Join(dir, newExeName)

		// 临时文件路径
		tempNewPath := newExePath + ".new"
		oldExePath := currentExe + ".old"

		// 删除可能存在的旧文件
		_ = os.Remove(tempNewPath)
		_ = os.Remove(oldExePath)

		// 将新文件复制到临时位置
		if err := fm.copyFile(tmpFile, tempNewPath); err != nil {
			return fmt.Errorf("复制新程序到临时位置失败: %w", err)
		}

		// 删除临时下载文件
		_ = os.Remove(tmpFile)

		logger.GetGlobalLogger("app/updater").Infof("已准备延迟更新文件，新版本文件名: %s", newExeName)
		return nil
	}

	// Linux/Mac 先备份再替换
	backupFile := currentExe + ".old"
	_ = os.Remove(backupFile) // 删除旧备份
	if err := fm.copyFile(currentExe, backupFile); err != nil {
		return fmt.Errorf("备份失败: %w", err)
	}
	logger.GetGlobalLogger("app/updater").Info("已备份当前版本")

	if err := os.Rename(tmpFile, currentExe); err != nil {
		return fmt.Errorf("替换程序失败: %w", err)
	}

	return nil
}

// RestartProgram 重启程序
func (fm *FileManager) RestartProgram() {
	currentExe, err := os.Executable()
	if err != nil {
		logger.GetGlobalLogger("app/updater").Errorf("获取程序路径失败: %v", err)
		return
	}

	// 获取当前工作目录
	workDir, err := os.Getwd()
	if err != nil {
		logger.GetGlobalLogger("app/updater").Warnf("获取工作目录失败: %v", err)
		workDir = filepath.Dir(currentExe)
	}

	logger.GetGlobalLogger("app/updater").Info("准备启动新版本程序...")

	if runtime.GOOS == "windows" {
		// Windows: 检查是否有 .new 文件需要更新
		dir := filepath.Dir(currentExe)

		// 查找所有 .new 文件
		files, err := filepath.Glob(filepath.Join(dir, "*.exe.new"))
		if err == nil && len(files) > 0 {
			// 找到更新文件，使用第一个
			newFilePath := files[0]
			logger.GetGlobalLogger("app/updater").Infof("检测到更新文件: %s", filepath.Base(newFilePath))
			fm.performDelayedUpdate(currentExe, newFilePath, workDir)
			return
		} else {
			// 普通重启（无更新）
			args := []string{"/c", "start", "Task Processor", "/D", workDir, currentExe}
			args = append(args, os.Args[1:]...)

			cmd := exec.Command("cmd", args...)
			if err := cmd.Start(); err != nil {
				logger.GetGlobalLogger("app/updater").Errorf("启动新程序失败: %v", err)
				return
			}

			logger.GetGlobalLogger("app/updater").Info("新程序启动命令已执行，当前程序即将退出...")
			logger.GetGlobalLogger("app/updater").Info("提示: 旧版本已备份为 .old 文件，确认新版本正常后可手动删除")

			// 刷新日志缓冲区
			time.Sleep(500 * time.Millisecond)

			// 在 Windows 上使用 taskkill 强制终止当前进程
			pid := os.Getpid()
			logger.GetGlobalLogger("app/updater").Infof("强制终止当前进程 (PID: %d)...", pid)

			// 使用 taskkill 强制终止 - 添加panic recovery
			go func() {
				defer func() {
					if r := recover(); r != nil {
						logger.GetGlobalLogger("app/updater").Errorf("强制终止进程goroutine panic recovered: %v", r)
					}
				}()
				time.Sleep(1 * time.Second)
				exec.Command("taskkill", "/F", "/PID", fmt.Sprintf("%d", pid)).Run()
			}()

			// 正常退出
			time.Sleep(1 * time.Second)
			os.Exit(0)
		}
	} else {
		// Linux/Mac: 直接启动
		cmd := exec.Command(currentExe, os.Args[1:]...)
		cmd.Dir = workDir

		if err := cmd.Start(); err != nil {
			logger.GetGlobalLogger("app/updater").Errorf("启动新程序失败: %v", err)
			return
		}

		logger.GetGlobalLogger("app/updater").Info("新程序启动命令已执行，当前程序即将退出...")
		logger.GetGlobalLogger("app/updater").Info("提示: 旧版本已备份为 .old 文件，确认新版本正常后可手动删除")

		// 刷新日志缓冲区
		time.Sleep(500 * time.Millisecond)

		// 正常退出
		os.Exit(0)
	}
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

// performDelayedUpdate 执行延迟更新（Windows专用）
func (fm *FileManager) performDelayedUpdate(currentExe, newFilePath, workDir string) {
	// 从 .new 文件路径获取目标文件名
	// 例如：task-processor-2.8.9.exe.new -> task-processor-2.8.9.exe
	targetExePath := strings.TrimSuffix(newFilePath, ".new")
	oldExePath := currentExe + ".old"

	// 使用 PowerShell 脚本进行延迟更新
	psScript := fmt.Sprintf(`
Start-Sleep -Seconds 5
Write-Host "Starting file replacement..."

$currentExe = "%s"
$newFilePath = "%s"
$targetExePath = "%s"
$oldExePath = "%s"
$workDir = "%s"

if (Test-Path $oldExePath) {
    Write-Host "Removing old backup file..."
    Remove-Item $oldExePath -Force
}

if (Test-Path $currentExe) {
    Write-Host "Backing up current program..."
    Move-Item $currentExe $oldExePath
}

if (Test-Path $newFilePath) {
    Write-Host "Installing new version..."
    Move-Item $newFilePath $targetExePath
}

Write-Host "Program update completed, starting new version..."
Start-Process -FilePath $targetExePath -WorkingDirectory $workDir
Write-Host "New version started successfully"

# Clean up script itself
Start-Sleep -Seconds 2
Remove-Item $PSCommandPath -Force
`, currentExe, newFilePath, targetExePath, oldExePath, workDir)

	// 创建临时 PowerShell 脚本
	scriptPath := "temp_update.ps1"
	if err := os.WriteFile(scriptPath, []byte(psScript), 0644); err != nil {
		logger.GetGlobalLogger("app/updater").Errorf("创建更新脚本失败: %v", err)
		return
	}

	// 启动 PowerShell 脚本
	cmd := exec.Command("powershell", "-ExecutionPolicy", "Bypass", "-File", scriptPath)
	if err := cmd.Start(); err != nil {
		logger.GetGlobalLogger("app/updater").Errorf("启动更新脚本失败: %v", err)
		os.Remove(scriptPath)
		return
	}

	// 立即退出当前程序
	logger.GetGlobalLogger("app/updater").Infof("PowerShell更新脚本已启动，将更新为: %s", filepath.Base(targetExePath))
	time.Sleep(100 * time.Millisecond) // 短暂等待确保日志输出
	os.Exit(0)
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
		logger.GetGlobalLogger("app/updater").Warnf("无法保存更新错误日志: %v", err)
	} else {
		logger.GetGlobalLogger("app/updater").Info("更新错误日志已保存到: update-error.log")
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
		logger.GetGlobalLogger("app/updater").Infof("检测到 %v 前更新过", timeSinceUpdate.Round(time.Second))
		return true
	}

	return false
}

// MarkAsUpdated 标记已更新到指定版本
func (fm *FileManager) MarkAsUpdated(version string) {
	markerFile := ".update-marker"
	content := fmt.Sprintf("%s\n%s", version, time.Now().Format(time.RFC3339))
	if err := os.WriteFile(markerFile, []byte(content), 0644); err != nil {
		logger.GetGlobalLogger("app/updater").Warnf("无法创建更新标记文件: %v", err)
	} else {
		logger.GetGlobalLogger("app/updater").Infof("已标记更新到版本: %s", version)
	}
}
