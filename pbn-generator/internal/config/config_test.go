package config

import (
	"crypto/rand"
	"crypto/rsa"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func validBaseConfig() Config {
	return Config{
		DBDriver:           "pgx",
		DSN:                "postgres://auth:auth@localhost:5432/auth?sslmode=disable",
		JWTSecret:          "test-jwt-token-12345",
		SessionTTL:         24 * time.Hour,
		LoginRateLimit:     30,
		LoginRateWindow:    time.Minute,
		RegisterRateLimit:  10,
		RegisterRateWindow: time.Minute,
		PasswordMinLength:  10,
		PasswordMaxLength:  128,
		APIKeySecret:       "test-api-key-12345",
		DeployMode:         "local_mock",
	}
}

func writeKnownHostsFile(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	pub, err := ssh.NewPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("new public key: %v", err)
	}
	line := knownhosts.Line([]string{"example.com"}, pub) + "\n"
	path := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(path, []byte(line), 0o644); err != nil {
		t.Fatalf("write known_hosts: %v", err)
	}
	return path
}

func TestConfigValidateLocalMock(t *testing.T) {
	t.Parallel()
	cfg := validBaseConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected local_mock config to be valid, got: %v", err)
	}
}

func TestConfigValidateInvalidDeployMode(t *testing.T) {
	t.Parallel()
	cfg := validBaseConfig()
	cfg.DeployMode = "invalid_mode"
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "DEPLOY_MODE must be one of") {
		t.Fatalf("expected deploy mode error, got: %v", err)
	}
}

func TestConfigValidateInvalidTargetsJSON(t *testing.T) {
	t.Parallel()
	cfg := validBaseConfig()
	cfg.DeployTargetsJSON = "{bad-json"
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "DEPLOY_TARGETS_JSON must be valid JSON") {
		t.Fatalf("expected targets json error, got: %v", err)
	}
}

func TestConfigValidateSSHRemoteRequiresFields(t *testing.T) {
	t.Parallel()
	cfg := validBaseConfig()
	cfg.DeployMode = "ssh_remote"
	cfg.DeployTimeout = 0
	cfg.DeployMaxParallel = 0
	cfg.DeployStagingStrategy = ""
	cfg.DeployStagingDirName = ""
	cfg.DeploySSHPoolMaxOpen = 0
	cfg.DeploySSHPoolMaxIdle = 0
	cfg.DeploySSHPoolIdleTTL = 0
	cfg.DeployKnownHostsPath = ""

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected validation error for missing ssh_remote fields")
	}
	msg := err.Error()
	for _, needle := range []string{
		"DEPLOY_TIMEOUT must be > 0 for ssh_remote",
		"DEPLOY_MAX_PARALLEL must be > 0 for ssh_remote",
		"DEPLOY_STAGING_STRATEGY is required for ssh_remote",
		"DEPLOY_STAGING_DIR_NAME is required for ssh_remote",
		"DEPLOY_SSH_POOL_MAX_OPEN must be > 0 for ssh_remote",
		"DEPLOY_SSH_POOL_MAX_IDLE must be > 0 for ssh_remote",
		"DEPLOY_SSH_POOL_IDLE_TTL must be > 0 for ssh_remote",
		"DEPLOY_KNOWN_HOSTS_PATH is required for ssh_remote",
	} {
		if !strings.Contains(msg, needle) {
			t.Fatalf("expected validation to contain %q, got: %s", needle, msg)
		}
	}
}

func TestConfigValidateSSHRemoteKnownHostsFormat(t *testing.T) {
	t.Parallel()
	cfg := validBaseConfig()
	cfg.DeployMode = "ssh_remote"
	cfg.DeployTimeout = 30 * time.Second
	cfg.DeployMaxParallel = 5
	cfg.DeployStagingStrategy = "co_located"
	cfg.DeployStagingDirName = ".tmp_deploy"
	cfg.DeploySSHPoolMaxOpen = 5
	cfg.DeploySSHPoolMaxIdle = 2
	cfg.DeploySSHPoolIdleTTL = time.Minute

	invalid := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(invalid, []byte("not-a-known-hosts-line\n"), 0o644); err != nil {
		t.Fatalf("write invalid known_hosts: %v", err)
	}
	cfg.DeployKnownHostsPath = invalid

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "invalid known_hosts format") {
		t.Fatalf("expected known_hosts format error, got: %v", err)
	}
}

func TestConfigValidateSSHRemoteKeyPermissions(t *testing.T) {
	t.Parallel()
	cfg := validBaseConfig()
	cfg.DeployMode = "ssh_remote"
	cfg.DeployTimeout = 30 * time.Second
	cfg.DeployMaxParallel = 5
	cfg.DeployStagingStrategy = "co_located"
	cfg.DeployStagingDirName = ".tmp_deploy"
	cfg.DeploySSHPoolMaxOpen = 5
	cfg.DeploySSHPoolMaxIdle = 2
	cfg.DeploySSHPoolIdleTTL = time.Minute
	cfg.DeployKnownHostsPath = writeKnownHostsFile(t)

	keyPath := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(keyPath, []byte("dummy-private-key"), 0o644); err != nil {
		t.Fatalf("write key: %v", err)
	}
	cfg.DeploySSHKeyPath = keyPath

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "must have 0600 permissions") {
		t.Fatalf("expected key mode error, got: %v", err)
	}
}

func TestConfigValidateSSHRemoteValid(t *testing.T) {
	t.Parallel()
	cfg := validBaseConfig()
	cfg.DeployMode = "ssh_remote"
	cfg.DeployTimeout = 30 * time.Second
	cfg.DeployMaxParallel = 5
	cfg.DeployStagingStrategy = "co_located"
	cfg.DeployStagingDirName = ".tmp_deploy"
	cfg.DeploySSHPoolMaxOpen = 5
	cfg.DeploySSHPoolMaxIdle = 2
	cfg.DeploySSHPoolIdleTTL = time.Minute
	cfg.DeployKnownHostsPath = writeKnownHostsFile(t)

	keyPath := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(keyPath, []byte("dummy-private-key"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	cfg.DeployTargetsJSON = `{"media1":{"ssh_user":"deploy","ssh_key_path":"` + keyPath + `"}}`

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid ssh_remote config, got: %v", err)
	}
}
