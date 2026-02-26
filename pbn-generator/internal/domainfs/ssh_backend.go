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
		if _, stderr, runErr := runSSHCommand(ctx, client, "mkdir -p -- "+shellQuote(path.Dir(fullPath)), nil); runErr != nil {
			return mapSSHError(runErr, stderr, fullPath)
		}
		if _, stderr, runErr := runSSHCommand(ctx, client, "cat > "+shellQuote(fullPath), content); runErr != nil {
			return mapSSHError(runErr, stderr, fullPath)
		}
		return b.applyOwnership(ctx, client, dctx.SiteOwner, fullPath, false)
	})
}

func (b *SSHBackend) EnsureDir(ctx context.Context, dctx DomainFSContext, dirPath string) error {
	fullPath, _, target, err := b.resolveTargetAndPath(dctx, dirPath)
	if err != nil {
		return err
	}
	return b.withSSH(ctx, target, func(client *ssh.Client) error {
		if _, stderr, runErr := runSSHCommand(ctx, client, "mkdir -p -- "+shellQuote(fullPath), nil); runErr != nil {
			return mapSSHError(runErr, stderr, fullPath)
		}
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
		if _, stderr, runErr := runSSHCommand(ctx, client, "mkdir -p -- "+shellQuote(path.Dir(newFull)), nil); runErr != nil {
			return mapSSHError(runErr, stderr, newFull)
		}
		if _, stderr, runErr := runSSHCommand(ctx, client, "mv -- "+shellQuote(oldFull)+" "+shellQuote(newFull), nil); runErr != nil {
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
		cmd := "if [ -d " + shellQuote(fullPath) + " ]; then rmdir -- " + shellQuote(fullPath) + "; else rm -f -- " + shellQuote(fullPath) + "; fi"
		_, stderr, runErr := runSSHCommand(ctx, client, cmd, nil)
		if runErr != nil {
			return mapSSHError(runErr, stderr, fullPath)
		}
		return nil
	})
}

func (b *SSHBackend) DeleteAll(ctx context.Context, dctx DomainFSContext, relPath string) error {
	fullPath, _, target, err := b.resolveTargetAndPath(dctx, relPath)
	if err != nil {
		return err
	}
	return b.withSSH(ctx, target, func(client *ssh.Client) error {
		_, stderr, runErr := runSSHCommand(ctx, client, "rm -rf -- "+shellQuote(fullPath), nil)
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

	cmd := "sh -lc " + shellQuote(command)
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
