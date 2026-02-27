package main

import (
	"fmt"
	"strings"

	"obzornik-pbn-generator/internal/config"
	"obzornik-pbn-generator/internal/domainfs"
)

func buildPublishContentBackend(cfg config.Config) (domainfs.SiteContentBackend, *domainfs.SSHPool, error) {
	if !strings.EqualFold(strings.TrimSpace(cfg.DeployMode), "ssh_remote") {
		return nil, nil, nil
	}
	localBackend := domainfs.NewLocalFSBackend(cfg.DeployBaseDir)

	targets, err := domainfs.ParseDeployTargetsJSON(cfg.DeployTargetsJSON)
	if err != nil {
		return nil, nil, err
	}
	for alias, target := range targets {
		if strings.TrimSpace(target.KeyPath) == "" {
			target.KeyPath = strings.TrimSpace(cfg.DeploySSHKeyPath)
			targets[alias] = target
		}
	}
	if len(targets) == 0 {
		return nil, nil, fmt.Errorf("DEPLOY_TARGETS_JSON must contain at least one target for ssh_remote")
	}

	pool, err := domainfs.NewSSHPool(domainfs.SSHPoolConfig{
		MaxOpen:        cfg.DeploySSHPoolMaxOpen,
		MaxIdle:        cfg.DeploySSHPoolMaxIdle,
		IdleTTL:        cfg.DeploySSHPoolIdleTTL,
		DialTimeout:    cfg.DeployTimeout,
		KnownHostsPath: cfg.DeployKnownHostsPath,
	})
	if err != nil {
		return nil, nil, err
	}

	sshBackend, err := domainfs.NewSSHBackend(pool, targets)
	if err != nil {
		_ = pool.Close()
		return nil, nil, err
	}
	return domainfs.NewRouterBackend(localBackend, sshBackend), pool, nil
}
