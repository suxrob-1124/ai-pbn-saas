#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────
# Первичная настройка сервера для SiteGenAI
# Запускать ОДИН раз на чистом сервере (185.209.20.95)
#
# Использование:
#   ssh root@185.209.20.95
#   bash /opt/sitegenai/scripts/server-init.sh
# ─────────────────────────────────────────────────────────────
set -euo pipefail

DEPLOY_PATH="/opt/sitegenai"
DEPLOY_USER="deploy"
DOMAIN="sitegenai.alfasearch.ru"
EMAIL="s.shukurov@advisability.com"

echo "══════════════════════════════════════════════════"
echo " SiteGenAI — Server Init"
echo "══════════════════════════════════════════════════"

# ── 1. Системные пакеты ─────────────────────────────────────
echo "── [1/7] Установка Docker и зависимостей ──"
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

# ── 2. Deploy-пользователь ──────────────────────────────────
echo "── [2/7] Создание пользователя deploy ──"
if ! id "$DEPLOY_USER" &>/dev/null; then
  useradd -m -s /bin/bash "$DEPLOY_USER"
  usermod -aG docker "$DEPLOY_USER"
  mkdir -p /home/$DEPLOY_USER/.ssh
  # Скопируй свой публичный ключ сюда:
  # echo "ssh-ed25519 AAAA... your-key" > /home/$DEPLOY_USER/.ssh/authorized_keys
  chmod 700 /home/$DEPLOY_USER/.ssh
  chown -R $DEPLOY_USER:$DEPLOY_USER /home/$DEPLOY_USER/.ssh
  echo "Пользователь $DEPLOY_USER создан. Не забудь добавить SSH ключ!"
else
  echo "Пользователь $DEPLOY_USER уже существует"
fi

# ── 3. Директории проекта ───────────────────────────────────
echo "── [3/7] Создание директорий ──"
mkdir -p "$DEPLOY_PATH"/{certbot/conf,certbot/www,data/postgres,backups/postgres/container,server,secrets/ssh}
chown -R $DEPLOY_USER:$DEPLOY_USER "$DEPLOY_PATH"

# ── 4. Firewall ─────────────────────────────────────────────
echo "── [4/7] Настройка firewall (ufw) ──"
if command -v ufw &>/dev/null; then
  ufw allow 22/tcp    # SSH
  ufw allow 80/tcp    # HTTP (certbot + redirect)
  ufw allow 443/tcp   # HTTPS
  ufw --force enable
  ufw status
else
  echo "ufw не найден, пропускаем"
fi

# ── 5. Получение TLS-сертификата ────────────────────────────
echo "── [5/7] Получение Let's Encrypt сертификата ──"
if [ ! -f "$DEPLOY_PATH/certbot/conf/live/$DOMAIN/fullchain.pem" ]; then
  # Запускаем временный nginx для certbot challenge
  docker run -d --name certbot-nginx \
    -p 80:80 \
    -v "$DEPLOY_PATH/certbot/www:/var/www/certbot" \
    nginx:alpine \
    sh -c 'mkdir -p /var/www/certbot && nginx -g "daemon off;"'

  # Создаём конфиг для certbot challenge
  docker exec certbot-nginx sh -c '
    echo "server { listen 80; location /.well-known/acme-challenge/ { root /var/www/certbot; } location / { return 444; } }" \
    > /etc/nginx/conf.d/default.conf && nginx -s reload
  '

  sleep 2

  # Получаем сертификат
  docker run --rm \
    -v "$DEPLOY_PATH/certbot/conf:/etc/letsencrypt" \
    -v "$DEPLOY_PATH/certbot/www:/var/www/certbot" \
    certbot/certbot certonly \
      --webroot -w /var/www/certbot \
      -d "$DOMAIN" \
      --agree-tos \
      --no-eff-email \
      -m "$EMAIL"

  # Копируем сертификаты в нужное место
  if [ -d "$DEPLOY_PATH/certbot/conf/live/$DOMAIN" ]; then
    echo "Сертификат получен!"
  else
    echo "ОШИБКА: Сертификат не получен. Проверь DNS A-запись для $DOMAIN"
  fi

  docker rm -f certbot-nginx 2>/dev/null || true
else
  echo "Сертификат уже существует"
fi

# ── 6. .env файл ────────────────────────────────────────────
echo "── [6/7] Проверка .env ──"
if [ ! -f "$DEPLOY_PATH/.env" ]; then
  echo ""
  echo "⚠  ВАЖНО: Скопируй .env файл с секретами на сервер!"
  echo "   scp .env $DEPLOY_USER@$(hostname -I | awk '{print $1}'):$DEPLOY_PATH/.env"
  echo ""
else
  echo ".env файл найден"
fi

# ── 7. Итог ─────────────────────────────────────────────────
echo "── [7/7] Готово ──"
echo ""
echo "══════════════════════════════════════════════════"
echo " Первичная настройка завершена!"
echo ""
echo " Следующие шаги:"
echo "  1. Добавь SSH ключ для deploy:"
echo "     echo 'ssh-ed25519 ...' >> /home/$DEPLOY_USER/.ssh/authorized_keys"
echo ""
echo "  2. Скопируй .env с секретами:"
echo "     scp .env $DEPLOY_USER@185.209.20.95:$DEPLOY_PATH/.env"
echo ""
echo "  3. Первый запуск (из GitLab или вручную):"
echo "     cd $DEPLOY_PATH"
echo "     docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build"
echo ""
echo "  4. Настрой GitLab CI/CD Variables:"
echo "     SSH_PRIVATE_KEY  — приватный ключ deploy"
echo "     SSH_KNOWN_HOSTS  — ssh-keyscan 185.209.20.95"
echo "══════════════════════════════════════════════════"
