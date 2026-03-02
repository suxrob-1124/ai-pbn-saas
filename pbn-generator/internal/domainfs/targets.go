package domainfs

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type SSHTarget struct {
	Alias   string
	Host    string
	User    string
	KeyPath string
	Port    int
}

func (t SSHTarget) Address() string {
	port := t.Port
	if port <= 0 {
		port = 22
	}
	return fmt.Sprintf("%s:%d", strings.TrimSpace(t.Host), port)
}

func ParseDeployTargetsJSON(raw string) (map[string]SSHTarget, error) {
	out := map[string]SSHTarget{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return out, nil
	}
	var payload map[string]map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, fmt.Errorf("parse DEPLOY_TARGETS_JSON: %w", err)
	}
	for alias, item := range payload {
		host := pickTargetString(item, "ssh_host", "host")
		user := pickTargetString(item, "ssh_user", "user")
		key := pickTargetString(item, "ssh_key_path", "key_path")
		port := pickTargetInt(item, 22, "ssh_port", "port")
		if host == "" || user == "" {
			continue
		}
		out[alias] = SSHTarget{
			Alias:   alias,
			Host:    host,
			User:    user,
			KeyPath: key,
			Port:    port,
		}
	}
	return out, nil
}

func pickTargetString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func pickTargetInt(m map[string]any, fallback int, keys ...string) int {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch tv := v.(type) {
		case float64:
			if int(tv) > 0 {
				return int(tv)
			}
		case int:
			if tv > 0 {
				return tv
			}
		case string:
			if n, err := strconv.Atoi(strings.TrimSpace(tv)); err == nil && n > 0 {
				return n
			}
		}
	}
	return fallback
}
