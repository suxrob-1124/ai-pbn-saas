package domainfs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type SSHBackend struct {
	pool    *SSHPool
	targets map[string]SSHTarget
}

func NewSSHBackend(pool *SSHPool, targets map[string]SSHTarget) (*SSHBackend, error) {
	if pool == nil {
		return nil, fmt.Errorf("ssh pool is required")
	}
	copied := make(map[string]SSHTarget, len(targets))
	for k, v := range targets {
		copied[strings.TrimSpace(k)] = v
	}
	return &SSHBackend{
		pool:    pool,
		targets: copied,
	}, nil
}

func (b *SSHBackend) ReadFile(ctx context.Context, dctx DomainFSContext, filePath string) ([]byte, error) {
	fullPath, _, target, err := b.resolveTargetAndPath(dctx, filePath)
	if err != nil {
		return nil, err
	}
	var content []byte
	err = b.withSSH(ctx, target, func(client *ssh.Client) error {
		stdout, stderr, runErr := runSSHCommand(ctx, client, "cat -- "+shellQuote(fullPath), nil)
		if runErr != nil {
			return mapSSHError(runErr, stderr, fullPath)
		}
		content = stdout
		return nil
	})
	if err != nil {
		return nil, err
	}
	return content, nil
}

func (b *SSHBackend) WriteFile(ctx context.Context, dctx DomainFSContext, filePath string, content []byte) error {
	fullPath, _, target, err := b.resolveTargetAndPath(dctx, filePath)
	if err != nil {
		return err
	}
	return b.withSSH(ctx, target, func(client *ssh.Client) error {
		dir := path.Dir(fullPath)

		// 1. Убеждаемся, что директория для файла существует
		if _, stderr, runErr := runSSHCommand(ctx, client, "sudo mkdir -p -- "+shellQuote(dir), nil); runErr != nil {
			return mapSSHError(runErr, stderr, dir)
		}

		// 2. Безопасно пишем файл через sudo tee (это обходит любые блокировки папок)
		writeCmd := "sudo tee -- " + shellQuote(fullPath) + " > /dev/null"
		if _, stderr, runErr := runSSHCommand(ctx, client, writeCmd, content); runErr != nil {
			return mapSSHError(runErr, stderr, fullPath)
		}

		// 3. Возвращаем правильного владельца файлу (и на всякий случай папке, если она была создана только что)
		if dctx.SiteOwner != "" {
			_ = b.applyOwnership(ctx, client, dctx.SiteOwner, fullPath, false)
			_ = b.applyOwnership(ctx, client, dctx.SiteOwner, dir, false)
		}

		return nil
	})
}

func (b *SSHBackend) EnsureDir(ctx context.Context, dctx DomainFSContext, dirPath string) error {
	fullPath, _, target, err := b.resolveTargetAndPath(dctx, dirPath)
	if err != nil {
		return err
	}
	return b.withSSH(ctx, target, func(client *ssh.Client) error {
		if _, stderr, runErr := runSSHCommand(ctx, client, "mkdir -p -- "+shellQuote(fullPath), nil); runErr != nil {
			if isPermissionDeniedOutput(stderr, runErr) {
				if _, sudoStderr, sudoErr := runSSHCommand(ctx, client, "sudo mkdir -p -- "+shellQuote(fullPath), nil); sudoErr != nil {
					return mapSSHError(sudoErr, sudoStderr, fullPath)
				}
				if target.User != "" {
					_ = b.chownPath(ctx, client, target.User, fullPath)
				}
			} else {
				return mapSSHError(runErr, stderr, fullPath)
			}
		}
		// Всегда отдаем права исходному владельцу сайта (чтобы Nginx мог читать файлы)
		return b.applyOwnership(ctx, client, dctx.SiteOwner, fullPath, false)
	})
}

func (b *SSHBackend) Stat(ctx context.Context, dctx DomainFSContext, relPath string) (FileInfo, error) {
	fullPath, rootPath, target, err := b.resolveTargetAndPath(dctx, relPath)
	if err != nil {
		return FileInfo{}, err
	}
	var out FileInfo
	err = b.withSSH(ctx, target, func(client *ssh.Client) error {
		cmd := "if [ ! -e " + shellQuote(fullPath) + " ]; then exit 44; fi; " +
			"if [ -d " + shellQuote(fullPath) + " ]; then kind=d; else kind=f; fi; " +
			"size=$(stat -c %s -- " + shellQuote(fullPath) + " 2>/dev/null || echo 0); " +
			"mtime=$(stat -c %Y -- " + shellQuote(fullPath) + " 2>/dev/null || echo 0); " +
			"name=$(basename -- " + shellQuote(fullPath) + "); " +
			"printf '%s|%s|%s|%s\\n' \"$kind\" \"$size\" \"$mtime\" \"$name\""
		stdout, stderr, runErr := runSSHCommand(ctx, client, cmd, nil)
		if runErr != nil {
			if exitCode(runErr) == 44 {
				return fs.ErrNotExist
			}
			return mapSSHError(runErr, stderr, fullPath)
		}
		line := strings.TrimSpace(string(stdout))
		parts := strings.SplitN(line, "|", 4)
		if len(parts) != 4 {
			return fmt.Errorf("invalid stat output for %s", fullPath)
		}
		size, _ := strconv.ParseInt(parts[1], 10, 64)
		mtimeSec, _ := strconv.ParseInt(parts[2], 10, 64)
		rel, relErr := b.relativeToRoot(fullPath, rootPath)
		if relErr != nil {
			return relErr
		}
		out = FileInfo{
			Path:    rel,
			Name:    strings.TrimSpace(parts[3]),
			Size:    size,
			IsDir:   strings.TrimSpace(parts[0]) == "d",
			ModTime: time.Unix(mtimeSec, 0).UTC(),
		}
		return nil
	})
	if err != nil {
		return FileInfo{}, err
	}
	return out, nil
}

func (b *SSHBackend) ReadDir(ctx context.Context, dctx DomainFSContext, dirPath string) ([]EntryInfo, error) {
	fullPath, _, target, err := b.resolveTargetAndPath(dctx, dirPath)
	if err != nil {
		return nil, err
	}
	var out []EntryInfo
	err = b.withSSH(ctx, target, func(client *ssh.Client) error {
		cmd := "if [ ! -d " + shellQuote(fullPath) + " ]; then exit 45; fi; " +
			"find " + shellQuote(fullPath) + " -mindepth 1 -maxdepth 1 -printf '%y|%f\\n'"
		stdout, stderr, runErr := runSSHCommand(ctx, client, cmd, nil)
		if runErr != nil {
			if exitCode(runErr) == 45 {
				return fs.ErrNotExist
			}
			return mapSSHError(runErr, stderr, fullPath)
		}
		lines := strings.Split(strings.TrimSpace(string(stdout)), "\n")
		out = make([]EntryInfo, 0, len(lines))
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "|", 2)
			if len(parts) != 2 {
				continue
			}
			out = append(out, EntryInfo{
				Name:  parts[1],
				IsDir: parts[0] == "d",
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (b *SSHBackend) Move(ctx context.Context, dctx DomainFSContext, oldPath, newPath string) error {
	oldFull, _, target, err := b.resolveTargetAndPath(dctx, oldPath)
	if err != nil {
		return err
	}
	newFull, _, _, err := b.resolveTargetAndPath(dctx, newPath)
	if err != nil {
		return err
	}
	return b.withSSH(ctx, target, func(client *ssh.Client) error {
		destDir := shellQuote(path.Dir(newFull))
		mkdirCmd := "mkdir -p -- " + destDir + " || sudo mkdir -p -- " + destDir
		if _, stderr, runErr := runSSHCommand(ctx, client, mkdirCmd, nil); runErr != nil {
			return mapSSHError(runErr, stderr, newFull)
		}
		mvCmd := "mv -- " + shellQuote(oldFull) + " " + shellQuote(newFull) +
			" || sudo mv -- " + shellQuote(oldFull) + " " + shellQuote(newFull)
		if _, stderr, runErr := runSSHCommand(ctx, client, mvCmd, nil); runErr != nil {
			return mapSSHError(runErr, stderr, oldFull)
		}
		return nil
	})
}

func (b *SSHBackend) Delete(ctx context.Context, dctx DomainFSContext, relPath string) error {
	fullPath, _, target, err := b.resolveTargetAndPath(dctx, relPath)
	if err != nil {
		return err
	}
	return b.withSSH(ctx, target, func(client *ssh.Client) error {
		// For files/dirs owned by the site owner the SSH user may not have direct
		// write permission on the parent directory. Try the unprivileged command
		// first and fall back to sudo so the behaviour is consistent with
		// DeleteAll which uses sudo chown before rm.
		cmd := "if [ -d " + shellQuote(fullPath) + " ]; then rmdir -- " + shellQuote(fullPath) +
			" || sudo rmdir -- " + shellQuote(fullPath) +
			"; else rm -f -- " + shellQuote(fullPath) + " || sudo rm -f -- " + shellQuote(fullPath) + "; fi"
		_, stderr, runErr := runSSHCommand(ctx, client, cmd, nil)
		if runErr != nil {
			return mapSSHError(runErr, stderr, fullPath)
		}
		return nil
	})
}

func (b *SSHBackend) DeleteAll(ctx context.Context, dctx DomainFSContext, relPath string) error {
	fullPath, rootPath, target, err := b.resolveTargetAndPath(dctx, relPath)
	if err != nil {
		return err
	}
	return b.withSSH(ctx, target, func(client *ssh.Client) error {
		// 1. Временно забираем права на директорию себе (рекурсивно) для безопасного удаления
		if target.User != "" && dctx.SiteOwner != "" {
			_, stderr, chownErr := runSSHCommand(ctx, client, "sudo chown -R "+strings.TrimSpace(target.User)+" "+shellQuote(fullPath), nil)
			if chownErr != nil {
				return fmt.Errorf("failed to take ownership for deletion: %w", mapSSHError(chownErr, stderr, fullPath))
			}
		}

		var cmd string
		if fullPath == rootPath {
			// Если очищаем весь сайт, удаляем ТОЛЬКО содержимое (включая скрытые файлы),
			// чтобы не сломать саму папку ISPmanager
			cmd = "bash -c 'shopt -s dotglob nullglob; rm -rf " + shellQuote(fullPath) + "/*'"
		} else {
			// Если это вложенная папка (assets, css и т.д.), удаляем её целиком.
			// Пробуем без sudo, если не получится — с sudo (аналогично Delete).
			cmd = "rm -rf -- " + shellQuote(fullPath) +
				" || sudo rm -rf -- " + shellQuote(fullPath)
		}

		// 2. Выполняем удаление
		_, stderr, runErr := runSSHCommand(ctx, client, cmd, nil)

		// 3. Возвращаем права владельцу сайта на корень (т.к. корень мы не удалили)
		if fullPath == rootPath && dctx.SiteOwner != "" {
			_ = b.applyOwnership(ctx, client, dctx.SiteOwner, fullPath, false)
		}

		if runErr != nil {
			return mapSSHError(runErr, stderr, fullPath)
		}
		return nil
	})
}

func (b *SSHBackend) ListTree(ctx context.Context, dctx DomainFSContext, dirPath string) ([]FileInfo, error) {
	fullPath, rootPath, target, err := b.resolveTargetAndPath(dctx, dirPath)
	if err != nil {
		return nil, err
	}
	out := make([]FileInfo, 0, 32)
	err = b.withSSH(ctx, target, func(client *ssh.Client) error {
		cmd := "if [ ! -d " + shellQuote(fullPath) + " ]; then exit 45; fi; " +
			"find " + shellQuote(fullPath) + " -mindepth 1 -printf '%y|%s|%T@|%P\\n'"
		stdout, stderr, runErr := runSSHCommand(ctx, client, cmd, nil)
		if runErr != nil {
			if exitCode(runErr) == 45 {
				return fs.ErrNotExist
			}
			return mapSSHError(runErr, stderr, fullPath)
		}
		lines := strings.Split(strings.TrimSpace(string(stdout)), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "|", 4)
			if len(parts) != 4 {
				continue
			}
			rel := strings.TrimSpace(parts[3])
			if rel == "" || rel == "." {
				continue
			}
			size, _ := strconv.ParseInt(parts[1], 10, 64)
			mtimeFloat, _ := strconv.ParseFloat(parts[2], 64)
			mtime := time.Unix(int64(mtimeFloat), int64((mtimeFloat-float64(int64(mtimeFloat)))*1e9)).UTC()
			fullEntry := path.Join(fullPath, rel)
			rootRel, relErr := b.relativeToRoot(fullEntry, rootPath)
			if relErr != nil {
				return relErr
			}
			out = append(out, FileInfo{
				Path:    rootRel,
				Name:    path.Base(rel),
				Size:    size,
				IsDir:   parts[0] == "d",
				ModTime: mtime,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			return out[i].Name < out[j].Name
		}
		return out[i].Path < out[j].Path
	})
	return out, nil
}

func (b *SSHBackend) withSSH(ctx context.Context, target SSHTarget, fn func(*ssh.Client) error) error {
	lease, err := b.pool.Acquire(ctx, target)
	if err != nil {
		return err
	}
	defer lease.Release()

	if err := fn(lease.Client); err != nil {
		if isConnectionError(err) {
			lease.Discard()
			// Retry once with a fresh connection if the context is still alive.
			// Stale idle connections silently break; this avoids a spurious 500
			// on the first read after the SSH server closes the keep-alive.
			if ctx.Err() != nil {
				return err
			}
			lease2, err2 := b.pool.Acquire(ctx, target)
			if err2 != nil {
				return err
			}
			defer lease2.Release()
			if err2 = fn(lease2.Client); err2 != nil {
				if isConnectionError(err2) {
					lease2.Discard()
				}
				return err2
			}
			return nil
		}
		return err
	}
	return nil
}

func (b *SSHBackend) resolveTargetAndPath(dctx DomainFSContext, relPath string) (string, string, SSHTarget, error) {
	target, err := b.resolveTarget(dctx)
	if err != nil {
		return "", "", SSHTarget{}, err
	}
	root, err := b.resolveRoot(dctx)
	if err != nil {
		return "", "", SSHTarget{}, err
	}
	cleanRel, err := normalizeRelativePath(relPath)
	if err != nil {
		return "", "", SSHTarget{}, err
	}
	if cleanRel == "" {
		return root, root, target, nil
	}
	full := path.Clean(path.Join(root, cleanRel))
	if err := b.ensureWithinRoot(full, root); err != nil {
		return "", "", SSHTarget{}, err
	}
	return full, root, target, nil
}

func (b *SSHBackend) resolveRoot(dctx DomainFSContext) (string, error) {
	root := strings.TrimSpace(dctx.PublishedPath)
	if root == "" {
		return "", fmt.Errorf("published path is required for ssh_remote")
	}
	root = path.Clean(root)
	if !strings.HasPrefix(root, "/") {
		root = "/" + root
	}
	if root == "/" {
		return "", fmt.Errorf("published path cannot be root")
	}
	return root, nil
}

func (b *SSHBackend) ensureWithinRoot(fullPath, rootPath string) error {
	if fullPath == rootPath {
		return nil
	}
	prefix := rootPath
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	if !strings.HasPrefix(fullPath, prefix) {
		return fmt.Errorf("path escapes root")
	}
	return nil
}

func (b *SSHBackend) relativeToRoot(fullPath, rootPath string) (string, error) {
	fullPath = path.Clean(fullPath)
	rootPath = path.Clean(rootPath)
	if err := b.ensureWithinRoot(fullPath, rootPath); err != nil {
		return "", err
	}
	if fullPath == rootPath {
		return ".", nil
	}
	rel := strings.TrimPrefix(fullPath, rootPath)
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" {
		return ".", nil
	}
	return rel, nil
}

func (b *SSHBackend) resolveTarget(dctx DomainFSContext) (SSHTarget, error) {
	serverID := strings.TrimSpace(dctx.ServerID)
	if serverID != "" {
		if target, ok := b.targets[serverID]; ok {
			return target, nil
		}
		return SSHTarget{}, fmt.Errorf("ssh target not found for server_id=%s", serverID)
	}
	if len(b.targets) == 1 {
		for _, target := range b.targets {
			return target, nil
		}
	}
	return SSHTarget{}, fmt.Errorf("server_id is required for ssh_remote")
}

func (b *SSHBackend) applyOwnership(ctx context.Context, client *ssh.Client, siteOwner, targetPath string, recursive bool) error {
	owner := strings.TrimSpace(siteOwner)
	if owner == "" {
		return nil
	}
	if err := validateOwner(owner); err != nil {
		return err
	}
	flag := ""
	if recursive {
		flag = "-R "
	}
	cmd := fmt.Sprintf("sudo chown %s%s %s", flag, owner, shellQuote(targetPath))
	_, stderr, err := runSSHCommand(ctx, client, cmd, nil)
	if err != nil {
		return mapSSHError(err, stderr, targetPath)
	}
	return nil
}

func runSSHCommand(ctx context.Context, client *ssh.Client, command string, stdin []byte) ([]byte, []byte, error) {
	if client == nil {
		return nil, nil, fmt.Errorf("ssh client is nil")
	}
	session, err := client.NewSession()
	if err != nil {
		return nil, nil, fmt.Errorf("open ssh session: %w", err)
	}
	defer session.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	cmd := "bash -lc " + shellQuote(command)
	done := make(chan error, 1)
	if len(stdin) > 0 {
		writer, err := session.StdinPipe()
		if err != nil {
			return nil, nil, fmt.Errorf("open stdin pipe: %w", err)
		}
		if err := session.Start(cmd); err != nil {
			return nil, nil, err
		}
		go func() {
			_, _ = io.Copy(writer, bytes.NewReader(stdin))
			_ = writer.Close()
		}()
		go func() { done <- session.Wait() }()
	} else {
		go func() { done <- session.Run(cmd) }()
	}

	select {
	case <-ctx.Done():
		_ = session.Close()
		return stdout.Bytes(), stderr.Bytes(), ctx.Err()
	case runErr := <-done:
		return stdout.Bytes(), stderr.Bytes(), runErr
	}
}

func mapSSHError(err error, stderr []byte, targetPath string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	lower := strings.ToLower(string(stderr))
	if strings.Contains(lower, "no such file or directory") {
		return fs.ErrNotExist
	}
	if strings.Contains(lower, "not found") {
		return fs.ErrNotExist
	}
	if strings.Contains(lower, "permission denied") || strings.Contains(lower, "operation not permitted") {
		return fs.ErrPermission
	}
	details := strings.TrimSpace(string(stderr))
	if details != "" {
		return fmt.Errorf("ssh command failed (%s): %s: %w", targetPath, details, err)
	}
	return fmt.Errorf("ssh command failed (%s): %w", targetPath, err)
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *ssh.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitStatus()
	}
	return -1
}

func validateOwner(owner string) error {
	for _, r := range owner {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			continue
		}
		switch r {
		case '_', '-', ':', '.':
			continue
		default:
			return fmt.Errorf("invalid site owner format")
		}
	}
	return nil
}

func isPermissionDeniedOutput(stderr []byte, err error) bool {
	msg := strings.ToLower(strings.TrimSpace(string(stderr)))
	if strings.Contains(msg, "permission denied") || strings.Contains(msg, "operation not permitted") {
		return true
	}
	if err == nil {
		return false
	}
	lowerErr := strings.ToLower(err.Error())
	return strings.Contains(lowerErr, "permission denied") || strings.Contains(lowerErr, "operation not permitted")
}

func (b *SSHBackend) pathExists(ctx context.Context, client *ssh.Client, fullPath string) (bool, error) {
	stdout, stderr, err := runSSHCommand(ctx, client, "if [ -e "+shellQuote(fullPath)+" ]; then printf '1'; else printf '0'; fi", nil)
	if err != nil {
		return false, mapSSHError(err, stderr, fullPath)
	}
	return strings.TrimSpace(string(stdout)) == "1", nil
}

func (b *SSHBackend) chownPath(ctx context.Context, client *ssh.Client, owner, targetPath string) error {
	trimmedOwner := strings.TrimSpace(owner)
	if trimmedOwner == "" {
		return fmt.Errorf("ssh target user is empty")
	}
	if err := validateOwner(trimmedOwner); err != nil {
		return err
	}
	_, stderr, err := runSSHCommand(ctx, client, "sudo chown "+trimmedOwner+" "+shellQuote(targetPath), nil)
	if err != nil {
		return mapSSHError(err, stderr, targetPath)
	}
	return nil
}

func shellQuote(raw string) string {
	return "'" + strings.ReplaceAll(raw, "'", `'\''`) + "'"
}

func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection closed") ||
		strings.Contains(msg, "use of closed network connection") ||
		strings.Contains(msg, "timeout") {
		return true
	}
	return errors.Is(err, io.EOF) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}

func (b *SSHBackend) DiscoverDomain(ctx context.Context, serverID, domainHost string) (string, string, error) {
	target, err := b.resolveTarget(DomainFSContext{ServerID: serverID})
	if err != nil {
		return "", "", err
	}

	script := buildProbeScript(domainHost)

	var pubPath, owner string
	err = b.withSSH(ctx, target, func(client *ssh.Client) error {
		// Обязательно используем bash -lc (мы это правили ранее)
		stdout, stderr, runErr := runSSHCommand(ctx, client, script, nil)
		if runErr != nil {
			return mapSSHError(runErr, stderr, "discovery")
		}

		// Парсим ответ вручную (без inventory)
		values := make(map[string]string)
		lines := strings.Split(string(stdout), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				values[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}

		status := strings.ToLower(values["INVENTORY_STATUS"])
		switch status {
		case "found":
			pubPath = values["INVENTORY_PATH"]
			owner = values["INVENTORY_OWNER"]
			return nil
		case "not_found":
			return fmt.Errorf("Сайт %s не найден в конфигах Nginx/Apache", domainHost)
		case "permission_denied":
			return fmt.Errorf("Сайт найден, но нет прав на папку: %s", values["INVENTORY_PATH"])
		case "ambiguous":
			return fmt.Errorf("Найдено несколько путей для %s, невозможно выбрать автоматически", domainHost)
		default:
			return fmt.Errorf("Неизвестный ответ сервера: %s", string(stdout))
		}
	})

	if err != nil {
		return "", "", err
	}
	if pubPath == "" {
		return "", "", fmt.Errorf("Не удалось определить путь для %s", domainHost)
	}
	return pubPath, owner, nil
}

func buildProbeScript(domainHost string) string {
	domain := shellQuote(domainHost)
	// Используем strings.ReplaceAll вместо Sprintf, чтобы не ломать bash-символы %
	script := `set -euo pipefail
domain=__DOMAIN__
dirs=(
  /etc/nginx/sites-enabled
  /etc/nginx/sites-available
  /etc/nginx/conf.d
  /etc/nginx/vhosts
  /etc/apache2/sites-enabled
  /etc/apache2/sites-available
  /etc/apache2/conf-enabled
  /etc/httpd/conf.d
  /usr/local/mgr5/etc/nginx/vhosts
  /usr/local/mgr5/etc/apache2/vhosts
)
cfgs=()
for d in "${dirs[@]}"; do
  [[ -d "$d" ]] || continue
  while IFS= read -r f; do cfgs+=("$f"); done < <(grep -RIl -- "$domain" "$d" 2>/dev/null || true)
done
declare -a roots=()
for cfg in "${cfgs[@]}"; do
  declare -A vars=()
  while IFS='|' read -r var_name var_value; do
    [[ -n "$var_name" && -n "$var_value" ]] || continue
    vars["$var_name"]="$var_value"
  done < <(awk '
    BEGIN {IGNORECASE=1}
    $1=="set" && $2 ~ /^\$/ {
      v=$2
      val=$3
      sub(/;$/, "", val)
      gsub(/"/, "", val)
      print v "|" val
    }
  ' "$cfg" 2>/dev/null || true)
  while IFS= read -r candidate; do
    candidate="${candidate%%;*}"
    candidate="${candidate%%\"*}"
    candidate="${candidate##\"}"
    if [[ "$candidate" == \$* ]]; then
      var="${candidate%%/*}"
      rest=""
      if [[ "$candidate" == */* ]]; then
        rest="/${candidate#*/}"
      fi
      if [[ -n "${vars[$var]+x}" ]]; then
        candidate="${vars[$var]}$rest"
      fi
    fi
    [[ -n "$candidate" ]] && roots+=("$candidate")
  done < <(awk 'BEGIN {IGNORECASE=1} $1=="root" {print $2} $1=="DocumentRoot" {print $2}' "$cfg" 2>/dev/null || true)
done
declare -A seen=()
declare -a uniq=()
for r in "${roots[@]}"; do
  [[ -n "$r" ]] || continue
  if [[ -z "${seen[$r]+x}" ]]; then
    seen[$r]=1
    uniq+=("$r")
  fi
done
if [[ ${#uniq[@]} -eq 0 ]]; then
  echo "INVENTORY_STATUS=not_found"
  exit 0
fi
if [[ ${#uniq[@]} -gt 1 ]]; then
  echo "INVENTORY_STATUS=ambiguous"
  exit 0
fi
path="${uniq[0]}"
echo "INVENTORY_PATH=$path"
if [[ ! -e "$path" ]]; then
  echo "INVENTORY_STATUS=not_found"
  exit 0
fi
if ! owner="$(stat -c '%U:%G' "$path" 2>/dev/null)"; then
  echo "INVENTORY_STATUS=permission_denied"
  exit 0
fi
echo "INVENTORY_STATUS=found"
echo "INVENTORY_OWNER=$owner"
`
	return strings.ReplaceAll(script, "__DOMAIN__", domain)
}
