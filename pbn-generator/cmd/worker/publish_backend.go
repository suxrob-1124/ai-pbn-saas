package main

import (
	"fmt"
	"strings"

	"obzornik-pbn-generator/internal/config"
	"obzornik-pbn-generator/internal/domainfs"
)

func buildPublishContentBackend(cfg config.Config) (domainfs.SiteContentBackend, *domainfs.SSHPool, *domainfs.SSHBackend, error) {
	localBackend := domainfs.NewLocalFSBackend(cfg.DeployBaseDir)

	targets, err := domainfs.ParseDeployTargetsJSON(cfg.DeployTargetsJSON)
	if err != nil {
		return nil, nil, nil, err
	}
	for alias, target := range targets {
		if strings.TrimSpace(target.KeyPath) == "" {
			target.KeyPath = strings.TrimSpace(cfg.DeploySSHKeyPath)
			targets[alias] = target
		}
	}

	// Если таргетов нет вообще
	if len(targets) == 0 {
		// И мы при этом требуем глобальный ssh_remote — это фатальная ошибка
		if strings.EqualFold(strings.TrimSpace(cfg.DeployMode), "ssh_remote") {
			return nil, nil, nil, fmt.Errorf("DEPLOY_TARGETS_JSON must contain at least one target for ssh_remote")
		}
		// Иначе мы в local_mock и нам просто не нужен SSH пул
		return localBackend, nil, nil, nil
	}

	// Если таргеты есть, ВСЕГДА собираем SSH-пул (это критично для Canary Rollout)
	pool, err := domainfs.NewSSHPool(domainfs.SSHPoolConfig{
		MaxOpen:        cfg.DeploySSHPoolMaxOpen,
		MaxIdle:        cfg.DeploySSHPoolMaxIdle,
		IdleTTL:        cfg.DeploySSHPoolIdleTTL,
		DialTimeout:    cfg.DeployTimeout,
		KnownHostsPath: cfg.DeployKnownHostsPath,
	})
	if err != nil {
		return nil, nil, nil, err
	}

	sshBackend, err := domainfs.NewSSHBackend(pool, targets)
	if err != nil {
		_ = pool.Close()
		return nil, nil, nil, err
	}

	return domainfs.NewRouterBackend(localBackend, sshBackend), pool, sshBackend, nil
}