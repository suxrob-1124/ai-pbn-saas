#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────
# SiteGenAI — Первичная настройка сервера
# Запускать ОДИН раз на сервере от root
#
# Использование:
#   ssh -p 21212 root@185.209.20.95
#   bash /opt/sitegenai/scripts/server-init.sh
# ─────────────────────────────────────────────────────────────
set -euo pipefail

DEPLOY_PATH="/opt/sitegenai"
DEPLOY_USER="deploy"
DOMAIN="sitegenai.alfasearch.ru"
EMAIL="s.shukurov@advisability.com"
SSH_PORT=21212

echo "══════════════════════════════════════════════════"
echo " SiteGenAI — Server Init"
echo "══════════════════════════════════════════════════"

# ── 1. Системные пакеты ─────────────────────────────────────
echo ""
echo "── [1/8] Установка Docker и зависимостей ──"
if ! command -v docker &>/dev/null; then
  apt-get update -qq
  apt-get install -y -qq ca-certificates curl gnupg lsb-release rsync
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
  chmod a+r /etc/apt/keyrings/docker.gpg
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
    https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" \
    > /etc/apt/sources.list.d/docker.list
  apt-get update -qq
  apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-compose-plugin
  systemctl enable --now docker
  echo "Docker установлен: $(docker --version)"
else
  echo "Docker уже установлен: $(docker --version)"
fi

# ── 2. Deploy-пользователь + SSH-ключ для GitLab CI ─────────
echo ""
echo "── [2/8] Создание пользователя deploy ──"
if ! id "$DEPLOY_USER" &>/dev/null; then
  useradd -m -s /bin/bash "$DEPLOY_USER"
  usermod -aG docker "$DEPLOY_USER"
  echo "Пользователь $DEPLOY_USER создан"
else
  echo "Пользователь $DEPLOY_USER уже существует"
  usermod -aG docker "$DEPLOY_USER" 2>/dev/null || true
fi

# ── 3. Генерация SSH-ключа для GitLab CI ────────────────────
echo ""
echo "── [3/8] SSH-ключ для GitLab CI ──"
DEPLOY_KEY_DIR="/home/$DEPLOY_USER/.ssh"
DEPLOY_KEY_FILE="$DEPLOY_KEY_DIR/gitlab_deploy_ed25519"
mkdir -p "$DEPLOY_KEY_DIR"

if [ ! -f "$DEPLOY_KEY_FILE" ]; then
  ssh-keygen -t ed25519 -C "gitlab-ci-deploy@sitegenai" -f "$DEPLOY_KEY_FILE" -N ""
  # Разрешаем вход по этому ключу
  cat "${DEPLOY_KEY_FILE}.pub" >> "$DEPLOY_KEY_DIR/authorized_keys"
  chmod 600 "$DEPLOY_KEY_DIR/authorized_keys"
  echo ""
  echo "╔══════════════════════════════════════════════════════════╗"
  echo "║  SSH-ключ создан. СКОПИРУЙ приватный ключ в GitLab:     ║"
  echo "╚══════════════════════════════════════════════════════════╝"
  echo ""
  echo "=== ПРИВАТНЫЙ КЛЮЧ (для GitLab CI Variable: SSH_PRIVATE_KEY) ==="
  cat "$DEPLOY_KEY_FILE"
  echo ""
  echo "=== КОНЕЦ КЛЮЧА ==="
  echo ""
else
  echo "Ключ уже существует: $DEPLOY_KEY_FILE"
fi

chmod 700 "$DEPLOY_KEY_DIR"
chown -R $DEPLOY_USER:$DEPLOY_USER "$DEPLOY_KEY_DIR"

# ── 4. Генерация SSH known_hosts ────────────────────────────
echo ""
echo "── [4/8] SSH known_hosts для GitLab ──"
KNOWN_HOSTS_LINE=$(ssh-keyscan -p $SSH_PORT 185.209.20.95 2>/dev/null)
echo ""
echo "=== KNOWN HOSTS (для GitLab CI Variable: SSH_KNOWN_HOSTS) ==="
echo "$KNOWN_HOSTS_LINE"
echo "=== КОНЕЦ ==="
echo ""

# ── 5. Директории проекта ───────────────────────────────────
echo "── [5/8] Создание директорий ──"
mkdir -p "$DEPLOY_PATH"/{certbot/conf,certbot/www,data/postgres,backups/postgres/container,server,secrets/ssh}
chown -R $DEPLOY_USER:$DEPLOY_USER "$DEPLOY_PATH"

# ── 6. Firewall ─────────────────────────────────────────────
echo ""
echo "── [6/8] Настройка firewall (ufw) ──"
if command -v ufw &>/dev/null; then
  ufw allow ${SSH_PORT}/tcp   # SSH
  ufw allow 80/tcp            # HTTP (certbot + redirect)
  ufw allow 443/tcp           # HTTPS
  ufw --force enable
  ufw status
else
  echo "ufw не найден, пропускаем"
fi

# ── 7. Получение TLS-сертификата ────────────────────────────
echo ""
echo "── [7/8] Получение Let's Encrypt сертификата ──"
if [ ! -f "$DEPLOY_PATH/certbot/conf/live/$DOMAIN/fullchain.pem" ]; then
  # Временный nginx для certbot challenge
  docker run -d --name certbot-nginx \
    -p 80:80 \
    -v "$DEPLOY_PATH/certbot/www:/var/www/certbot" \
    nginx:alpine \
    sh -c 'mkdir -p /var/www/certbot && nginx -g "daemon off;"'

  docker exec certbot-nginx sh -c '
    echo "server { listen 80; location /.well-known/acme-challenge/ { root /var/www/certbot; } location / { return 444; } }" \
    > /etc/nginx/conf.d/default.conf && nginx -s reload
  '
  sleep 2

  docker run --rm \
    -v "$DEPLOY_PATH/certbot/conf:/etc/letsencrypt" \
    -v "$DEPLOY_PATH/certbot/www:/var/www/certbot" \
    certbot/certbot certonly \
      --webroot -w /var/www/certbot \
      -d "$DOMAIN" \
      --agree-tos \
      --no-eff-email \
      -m "$EMAIL"

  if [ -d "$DEPLOY_PATH/certbot/conf/live/$DOMAIN" ]; then
    echo "Сертификат получен!"
  else
    echo "ОШИБКА: Сертификат не получен. Проверь DNS A-запись для $DOMAIN → 185.209.20.95"
  fi

  docker rm -f certbot-nginx 2>/dev/null || true
else
  echo "Сертификат уже существует"
fi

# ── 8. Итог ─────────────────────────────────────────────────
echo ""
echo "── [8/8] Готово ──"
echo ""
echo "══════════════════════════════════════════════════════════════"
echo ""
echo " Следующие шаги:"
echo ""
echo " 1. Скопируй .env с секретами на сервер (с локальной машины):"
echo "    scp -P ${SSH_PORT} .env ${DEPLOY_USER}@185.209.20.95:${DEPLOY_PATH}/.env"
echo ""
echo " 2. Скопируй secrets/ssh/ на сервер (ключи для деплоя сайтов):"
echo "    scp -P ${SSH_PORT} -r secrets/ssh/ ${DEPLOY_USER}@185.209.20.95:${DEPLOY_PATH}/secrets/ssh/"
echo ""
echo " 3. В GitLab → Settings → CI/CD → Variables добавь:"
echo "    SSH_PRIVATE_KEY (Type: File) — приватный ключ выше"
echo "    SSH_KNOWN_HOSTS (Type: Variable) — known_hosts выше"
echo ""
echo " 4. Первый запуск:"
echo "    ssh -p ${SSH_PORT} ${DEPLOY_USER}@185.209.20.95"
echo "    cd ${DEPLOY_PATH}"
echo "    docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build"
echo ""
echo "══════════════════════════════════════════════════════════════"
