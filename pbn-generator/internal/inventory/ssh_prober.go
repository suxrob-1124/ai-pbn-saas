package inventory

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type SSHProber struct {
	Timeout        time.Duration
	KnownHostsPath string
}

func (p SSHProber) Probe(ctx context.Context, target Target, domainASCII string) (ProbeResult, error) {
	if strings.TrimSpace(target.Host) == "" || strings.TrimSpace(target.User) == "" {
		return ProbeResult{}, fmt.Errorf("target host/user is required")
	}
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := []string{
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=yes",
	}
	if target.Port > 0 {
		args = append(args, "-p", fmt.Sprintf("%d", target.Port))
	}
	if strings.TrimSpace(p.KnownHostsPath) != "" {
		args = append(args, "-o", "UserKnownHostsFile="+p.KnownHostsPath)
	}
	if strings.TrimSpace(target.KeyPath) != "" {
		args = append(args, "-i", target.KeyPath)
	}
	args = append(args, fmt.Sprintf("%s@%s", target.User, target.Host), "bash", "-lc", BuildProbeScript(domainASCII))

	cmd := exec.CommandContext(probeCtx, "ssh", args...)
	out, err := cmd.CombinedOutput()
	parsed := ParseProbeOutput(string(out))
	if parsed.Status != "" {
		return parsed, nil
	}
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		if strings.Contains(strings.ToLower(msg), "permission denied") {
			return ProbeResult{Status: ProbePermissionDenied, Message: msg}, nil
		}
		return ProbeResult{}, fmt.Errorf("ssh probe failed: %s", msg)
	}
	return ProbeResult{Status: ProbeError, Message: "empty probe response"}, nil
}

func BuildProbeScript(domainASCII string) string {
	domain := shQuote(domainASCII)
	return fmt.Sprintf(`set -euo pipefail
domain=%s
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
  printf 'INVENTORY_PATHS=%%s\n' "$(IFS='|'; echo "${uniq[*]}")"
  exit 0
fi
path="${uniq[0]}"
echo "INVENTORY_PATH=$path"
if [[ ! -e "$path" ]]; then
  echo "INVENTORY_STATUS=not_found"
  exit 0
fi
if ! owner="$(stat -c '%%U:%%G' "$path" 2>/dev/null)"; then
  echo "INVENTORY_STATUS=permission_denied"
  exit 0
fi
echo "INVENTORY_STATUS=found"
echo "INVENTORY_OWNER=$owner"
`, domain)
}

func ParseProbeOutput(out string) ProbeResult {
	values := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		values[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	var status ProbeStatus
	switch strings.ToLower(values["INVENTORY_STATUS"]) {
	case "found":
		status = ProbeFound
	case "not_found":
		status = ProbeNotFound
	case "ambiguous":
		status = ProbeAmbiguous
	case "permission_denied":
		status = ProbePermissionDenied
	case "error":
		status = ProbeError
	default:
		status = ""
	}
	candidates := []string{}
	if raw := strings.TrimSpace(values["INVENTORY_PATHS"]); raw != "" {
		for _, p := range strings.Split(raw, "|") {
			p = strings.TrimSpace(p)
			if p != "" {
				candidates = append(candidates, p)
			}
		}
	}
	return ProbeResult{
		Status:        status,
		PublishedPath: values["INVENTORY_PATH"],
		SiteOwner:     values["INVENTORY_OWNER"],
		Candidates:    candidates,
	}
}

func shQuote(v string) string {
	if v == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(v, "'", `'"'"'`) + "'"
}
