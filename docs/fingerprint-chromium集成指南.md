# fingerprint-chromium 集成指南

## 概述

本指南介绍如何将 [fingerprint-chromium](https://github.com/adryfish/fingerprint-chromium) 项目集成到现有的 Go 爬虫系统中，以获得更好的反检测能力。

## 主要优势

- **更强的反检测能力**：基于 Ungoogled Chromium，去除了 Google 服务
- **丰富的指纹特征**：支持 GPU、Canvas、WebGL、WebRTC 等多种指纹
- **自动化优化**：专门针对自动化场景进行了优化
- **命令行配置**：通过标准命令行参数配置指纹特征

## 安装步骤

### 1. 下载 fingerprint-chromium

```bash
# 访问 GitHub Release 页面
# https://github.com/adryfish/fingerprint-chromium/releases

# 下载 Windows 版本（推荐 Chrome 142）
# ungoogled-chromium_142.0.7444.175-1.1_windows_x64.zip
```

### 2. 解压到项目目录

```bash
# 解压到项目的 chrome 目录
mkdir chrome
# 解压 zip 文件到 chrome 目录
# 确保 chrome.exe 位于 ./chrome/chrome.exe
```

### 3. 更新配置

```yaml
browser:
  browserPath: "./chrome/chrome.exe"
  fingerprintSeed: 0  # 0 表示随机种子
  fingerprintPlatform: "windows"
  fingerprintBrand: "Chrome"
  language: "en-US"
```

## 代码使用示例

```go
package main

import (
    "your-project/internal/crawler/shared/browser"
)

func main() {
    // 1. 创建指纹生成器
    generator := browser.NewFingerprintGenerator()
    
    // 2. 生成随机指纹
    fingerprint := generator.GenerateRandomFingerprint("8.8.8.8")
    
    // 3. 创建 fingerprint-chromium 配置
    config := &browser.BrowserConfig{
        Headless:               false,
        BrowserPath:            "./chrome/chrome.exe",
        FingerprintSeed:        12345,
        FingerprintPlatform:    "windows",
        FingerprintBrand:       "Chrome",
        Language:               "en-US",
        AcceptLanguage:         "en-US,en;q=0.9",
        ViewportWidth:          1920,
        ViewportHeight:         1080,
    }
    
    // 4. 创建启动选项
    launchOptions := browser.CreateLaunchOptions(config, fingerprint)
    
    // 5. 使用 Playwright 启动浏览器
    // ... 你的 Playwright 代码
}
```

## 配置参数说明

### 基础配置

- `browserPath`: fingerprint-chromium 可执行文件路径
- `fingerprintSeed`: 指纹种子（0表示随机）

### 指纹参数

- `fingerprintPlatform`: 操作系统类型（windows, linux, macos）
- `fingerprintBrand`: 浏览器品牌（Chrome, Edge, Opera, Vivaldi）
- `fingerprintBrandVersion`: 浏览器版本号
- `language`: 浏览器语言
- `acceptLanguage`: 接受的语言列表
- `timezone`: 时区设置

## 简化设计

系统现在默认使用 fingerprint-chromium 模式：

- 使用标准命令行参数配置指纹
- 适用于 fingerprint-chromium 浏览器
- 更好的反检测能力和指纹丰富度

## 测试指纹效果

访问以下网站测试指纹伪装效果：

- [CreepJS](https://abrahamjuliot.github.io/creepjs/) - 综合指纹检测
- [PixelScan](https://pixelscan.net/fingerprint-check) - 音频指纹检测
- [BrowserLeaks](https://browserleaks.com/) - 多种泄露检测

## 常见问题

### Q: 如何选择合适的指纹种子？
A: 可以使用时间戳、用户ID哈希或固定值。相同种子会产生相同指纹。

### Q: 是否需要修改现有代码？
A: 不需要。只需更新配置文件，设置正确的 browserPath 即可。

### Q: 如何验证集成是否成功？
A: 查看日志中的 "使用fingerprint-chromium参数" 信息，或访问测试网站验证。

## 性能对比

| 特性 | 原有方案 | fingerprint-chromium |
|------|----------|---------------------|
| 反检测能力 | 中等 | 强 |
| 指纹丰富度 | 基础 | 丰富 |
| 配置复杂度 | 简单 | 简单 |
| 维护成本 | 中等 | 低 |

## 总结

fingerprint-chromium 集成为您的爬虫系统提供了更强的反检测能力，同时保持了良好的向后兼容性。建议在生产环境中逐步迁移，先在测试环境验证效果。