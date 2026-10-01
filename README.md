# Домашний кинотеатр

Поиск фильмов на rutracker/rutor → скачивание через transmission-daemon на роутере → просмотр на проекторе через Jellyfin. Архитектура и обоснования — в `/Users/svsergeev/.claude/plans/shimmying-brewing-pillow.md`.

## Перед первым запуском (на хосте, не в docker-compose)

### 1. Swap (обязательно при 2 GB RAM)

```sh
sudo fallocate -l 2G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
sudo sysctl vm.swappiness=15
echo 'vm.swappiness=15' | sudo tee -a /etc/sysctl.conf
```

### 2. Монтирование SMB-диска с роутера

```sh
sudo apt install cifs-utils   # или apk/yum в зависимости от дистрибутива

sudo mkdir -p /mnt/downloads
sudo tee /etc/samba/credentials-router <<'EOF'
username=<логин на роутере>
password=<пароль>
EOF
sudo chmod 600 /etc/samba/credentials-router
```

`/etc/fstab`:
```
//192.168.1.1/downloads /mnt/downloads cifs credentials=/etc/samba/credentials-router,uid=1000,gid=1000,iocharset=utf8,vers=3.0,_netdev,x-systemd.automount,x-systemd.mount-timeout=30 0 0
```

```sh
sudo systemctl daemon-reload
sudo mount /mnt/downloads
```

Убедиться, что путь скачивания, который настроен в Transmission на роутере (`download-dir` по умолчанию), указывает на ту же шару/подпапку, что смонтирована здесь как `/mnt/downloads` — иначе backend не сможет ни проверить файлы, ни Jellyfin их не увидит.

В `.env` указать этот путь как `HOST_DOWNLOADS_PATH=/mnt/downloads` — `docker-compose.yml` берёт путь для bind-mount из этой переменной.

#### Если хост не видит роутер напрямую

На некоторых серверах доступ в домашнюю сеть (`192.168.1.1`) есть только у определённой docker-сети (например, через VPN-сайдкар типа WireGuard), а у самого хоста — нет. В этом случае шаг 2 выше напрямую не сработает (`mount -t cifs` с хоста упадёт — нет маршрута).

Решение — сервис `smb-nfs-bridge` (профиль `remote-lan-bridge`, выключен по умолчанию): контейнер в той же docker-сети, что и VPN, сам монтирует CIFS (доступ есть у его сети) и тут же отдаёт это наружу как NFSv4 на `127.0.0.1:2049`. Хост всегда может достучаться до контейнеров на своих docker-сетях — поэтому дальше хост монтирует NFS с localhost вместо CIFS с роутера, и весь остальной flow (`HOST_DOWNLOADS_PATH`, bind-mount в backend/jellyfin) не меняется.

```sh
# в .env: SMB_HOST, SMB_SHARE, SMB_USER, SMB_PASSWORD
# networks.homelab-private-network в docker-compose.yml — это реальное имя
# docker-сети, где у VPN-контейнера есть доступ в домашнюю сеть; подставьте своё

docker compose --profile remote-lan-bridge up -d smb-nfs-bridge
```

`/etc/fstab` на хосте — вместо CIFS-строки из шага 2:
```
127.0.0.1:/ /mnt/downloads nfs4 _netdev,x-systemd.automount,x-systemd.mount-timeout=30 0 0
```

(смонтировать можно, только когда `smb-nfs-bridge` уже поднят и прошёл CIFS-логин — проверить `docker compose logs smb-nfs-bridge`).

Известный риск: образ `itsthenetwork/nfs-server-alpine`, на котором собран `smb-nfs-bridge`, собирается только под `linux/amd64` — на arm64-хосте это пойдёт через эмуляцию (собирается и работает, но медленнее) либо не соберётся вовсе, если `qemu-user-static`/binfmt для amd64 не настроен.

## Запуск

```sh
cp .env.example .env
# заполнить .env: пароли, JWT_SECRET, TMDB_API_KEY, TRANSMISSION_USER/PASSWORD,
# RUTRACKER_LOGIN/PASSWORD, HOST_DOWNLOADS_PATH=/mnt/downloads

docker compose up -d --build
```

### Продакшен за nginx-balancer

На общем хосте, где единая точка входа (TLS + роутинг по доменам) —
`../nginx-balancer`, этот стек не должен сам публиковать 80/8096 на хост
(конфликт с портом 80 балансировщика). Вместо обычного запуска —
применить `docker-compose.prod.yml` поверх основного:

```sh
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

Это снимает хостовые порты у `nginx`/`jellyfin` и подключает их к сети
`homelab-private-network` (её создаёт и владеет ей `nginx-balancer` —
должен быть поднят первым, см. его README) под именами
`home_cinema_nginx`/`home_cinema_jellyfin`, по которым их находит vhost
`nginx/conf.d/50-home-cinema.conf` в `nginx-balancer`:
`cinema.services.fqrmix.ru` и `jellyfin.services.fqrmix.ru` (оба домена
уже покрыты существующим wildcard-сертификатом `nginx-balancer`, новых
DNS/cert-записей заводить не нужно).

Для локального теста (без nginx-balancer) ничего не меняется — просто не
добавлять `-f docker-compose.prod.yml`, порты 80/8096 публикуются на хост
как раньше.

rutracker.org закрыт Cloudflare-челленджем, который не проходит ни один обычный HTTP-клиент (ни curl, ни Go, ни Node) — ни логин-форма, ни что-либо ещё на домене. Поэтому в стек добавлен сайдкар `flaresolverr` (headless-браузер, решает challenge автоматически, ~15 сек холодный логин); `RutrackerClient` логинится через него сам при старте и при истечении сессии — никакого ручного шага не требуется. Это проверено вживую: 4/4 холодных логина подряд успешны, поиск и скачивание `.torrent` работают.

Важно для 2 GB хоста: Chrome в `flaresolverr` на пике решения challenge потребляет **~290 МБ** (не ~110, как казалось по простою) — при `mem_limit: 300m` это OOM-killило рендерер и валилось в `tab crashed`. В `docker-compose.yml` стоит `mem_limit: 600m` и `shm_size: 512m` (дефолтные 64 МБ `/dev/shm` тоже роняли Chrome) — не уменьшайте без повторной проверки на живых логинах. Это ещё один всегда работающий контейнер (в простое ~30 МБ) плюс кратковременные пики под 600 МБ при каждом логине — проверьте, что на вашем сервере включён swap (см. шаг 1 выше), иначе такой пик при памяти впритык может начать убивать другие процессы.

### Локальный тест на macOS (Docker Desktop)

Docker Desktop не даёт бинд-маунтить произвольные абсолютные пути вроде `/mnt/downloads`, которых нет на хосте и которых нет в его allowlist для расшаривания (`Settings → Resources → File sharing`). Если `HOST_DOWNLOADS_PATH` в `.env` не задан, `docker-compose.yml` по умолчанию использует `./data/downloads` (внутри репозитория, уже создана) — этого достаточно, чтобы поднять стек локально и проверить API/фронтенд без реального SMB-диска. На реальном сервере деплоя `HOST_DOWNLOADS_PATH` обязателен и должен указывать на смонтированную SMB-шару.

Первый запуск backend применяет миграции автоматически (`goose` встроен в бинарник).

## Проверка

```sh
docker stats                      # суммарный RSS должен укладываться в mem_limit'ы из docker-compose.yml
curl -s http://localhost/api/health
curl -s "http://localhost:8096"   # Jellyfin UI
```

Настроить Jellyfin (создать библиотеку на `/media`, создать API key в Dashboard → API Keys → положить в `JELLYFIN_API_KEY`).

## Известные риски (см. план)

- **rutracker-скрейпер** (`backend/internal/search/rutracker.go`) парсит HTML регулярками и может сломаться при изменении вёрстки rutracker. Логин идёт через `flaresolverr` автоматически при истечении сессии — ручных действий не требует, но если сам FlareSolverr перестанет проходить challenge (Cloudflare меняет защиту время от времени), поиск начнёт падать с `flaresolverr: ...` в логах backend.
- **rutor-скрейпер** (`backend/internal/search/rutor.go`) парсит HTML-страницу `/search/<query>` (RSS-поиск rutor отключил); зеркала и вёрстка периодически меняются — свериться с живым ответом при проблемах.
