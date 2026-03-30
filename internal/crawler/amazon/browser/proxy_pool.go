package browser

import (
	"task-processor/internal/core/config"
	"task-processor/internal/core/logger"
	sharedbrowser "task-processor/internal/crawler/shared/browser"
)

func newProxyPool(cfg *config.Config) *sharedbrowser.ProxyPool {
	if cfg == nil {
		return nil
	}

	return sharedbrowser.NewProxyPool(sharedbrowser.ProxyPoolConfig{
		Enabled:                cfg.Amazon.ProxyPool.Enabled,
		Strategy:               cfg.Amazon.ProxyPool.Strategy,
		FailureCooldownSeconds: cfg.Amazon.ProxyPool.FailureCooldownSeconds,
		Proxies:                cfg.Amazon.ProxyPool.Proxies,
	})
}

func logProxyAssigned(instanceID int, proxyServer string, strategy string) {
	if proxyServer == "" {
		return
	}
	if strategy == "health_aware" {
		logger.GetGlobalLogger("crawler/amazon").Infof("实例 %d 按健康度分配代理: %s", instanceID, proxyServer)
		return
	}
	logger.GetGlobalLogger("crawler/amazon").Infof("实例 %d 分配代理: %s", instanceID, proxyServer)
}

func logProxyFallback(instanceID int, proxyServer string) {
	if proxyServer == "" {
		return
	}
	logger.GetGlobalLogger("crawler/amazon").Warnf("所有代理都在冷却中，实例 %d 继续复用代理: %s", instanceID, proxyServer)
}

func logProxyCooldown(proxyServer string, cooldownSeconds int) {
	if proxyServer == "" {
		return
	}
	logger.GetGlobalLogger("crawler/amazon").Warnf("代理进入冷却: %s (冷却=%ds)", proxyServer, cooldownSeconds)
}
