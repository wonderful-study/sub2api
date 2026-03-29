# Sub2API 服务器无缝迁移流程

这份流程适用于当前这种 Docker 本地目录部署方式：

- `deploy/.env`
- `deploy/data`
- `deploy/postgres_data`
- `deploy/redis_data`

只要这四部分一起迁移，新服务器通常可以直接开箱即用，账号、分组、API Key、授权状态都会一起带过去。

## 迁移原则

1. **不要重建数据库**
   直接迁移现有数据目录，避免重新授权和重新建号。

2. **打包前先停服务**
   PostgreSQL 正在运行时直接拷目录，容易出现不一致。

3. **保留原 `.env`**
   里面有数据库密码、JWT 密钥、TOTP 加密密钥等关键配置。

4. **新服务器只负责恢复**
   先用旧服务器打包，再传到新服务器恢复。

5. **域名和 HTTPS 不包含在迁移包里**
   迁移包只包含应用与数据，不包含域名注册商里的 DNS 配置，也不包含服务器系统级的 `/etc/caddy/Caddyfile` 与证书状态。

## 当前推荐做法

在源服务器或本地仓库目录执行：

```bash
cd /path/to/sub2api
./deploy/package-server-bundle.sh --stop-stack --restart-stack
```

脚本会自动：

1. 停止当前容器
2. 打包迁移所需目录
3. 重新拉起当前容器

生成类似这样的文件：

```text
sub2api-server-bundle-20260324-210550.tar.gz
```

## 域名与 HTTPS 迁移要点

如果你已经给服务配置了域名和 HTTPS，例如：

- `wongrelay.wiki`
- `www.wongrelay.wiki`

那么以后换服务器时，除了恢复 `deploy/` 目录，还需要同步处理下面三件事：

1. **注册商 DNS**
   把根域 `@` 和 `www` 的记录切到新服务器 IP。

2. **新服务器反向代理**
   安装并配置 `Caddy`，让域名通过 `443` 转发到 `127.0.0.1:8080`。

3. **证书重新签发**
   当 DNS 指向新服务器并生效后，`Caddy` 会为新服务器重新申请证书。

> 建议：如果你确定未来经常迁移，在真正切换前可先把域名 TTL 降到 `300-600`，这样切换会更快生效。

## 迁移到新服务器

### 1. 传输迁移包

在源机器执行：

```bash
scp sub2api-server-bundle-*.tar.gz root@NEW_SERVER_IP:/root/
```

### 2. 新服务器安装 Docker

```bash
apt update && apt upgrade -y
apt install -y curl ca-certificates gnupg
curl -fsSL https://get.docker.com | sh
apt install -y docker-compose-plugin
systemctl enable docker --now
```

### 3. 解压

```bash
cd /root
tar xzf sub2api-server-bundle-*.tar.gz
cd deploy
```

### 4. 检查 `.env`

最少确认：

```env
BIND_HOST=0.0.0.0
SERVER_PORT=8080
POSTGRES_USER=sub2api
POSTGRES_PASSWORD=...
POSTGRES_DB=sub2api
JWT_SECRET=...
TOTP_ENCRYPTION_KEY=...
```

不要随意改：

- `POSTGRES_PASSWORD`
- `JWT_SECRET`
- `TOTP_ENCRYPTION_KEY`

如果新服务器准备通过域名 + `Caddy` 对外提供服务，建议把：

```env
BIND_HOST=127.0.0.1
```

这样 `sub2api` 只监听本机，公网入口统一交给 `Caddy` 处理。

### 5. 启动

```bash
docker compose up -d
docker compose ps
docker compose logs -f sub2api
```

### 6. 验证

```bash
curl http://127.0.0.1:8080/health
```

正常返回：

```json
{"status":"ok"}
```

## 域名切换与 HTTPS 恢复

下面这部分适用于“旧服务器马上到期，但域名仍要继续用”的场景。

### 1. 新服务器先恢复服务

先完成上面的恢复流程，确保：

```bash
docker compose ps
curl http://127.0.0.1:8080/health
```

都正常。

### 2. 安装 Caddy

在新服务器执行：

```bash
apt update
apt install -y debian-keyring debian-archive-keyring apt-transport-https curl
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list
apt update
apt install -y caddy
```

### 3. 写 Caddy 配置

可直接使用一个最小可用版本：

```caddy
YOUR_DOMAIN, www.YOUR_DOMAIN {
    reverse_proxy 127.0.0.1:8080
}
```

例如：

```caddy
wongrelay.wiki, www.wongrelay.wiki {
    reverse_proxy 127.0.0.1:8080
}
```

保存到：

```text
/etc/caddy/Caddyfile
```

然后检查配置：

```bash
caddy validate --config /etc/caddy/Caddyfile
```

### 4. 开放 80 和 443

```bash
ufw allow 80/tcp
ufw allow 443/tcp
ufw status
```

### 5. 在注册商切换 DNS

把下面记录改到新服务器 IP：

- `@` -> `NEW_SERVER_IP`
- `www` -> `NEW_SERVER_IP`

如果你用 NameSilo，切的是 `Domain Manager -> DNS Manager` 里的 `A` 记录。

### 6. 等待 DNS 生效

优先看权威 DNS：

```bash
dig @ns1.dnsowl.com YOUR_DOMAIN +short
dig @ns1.dnsowl.com www.YOUR_DOMAIN +short
```

再看公共递归 DNS 是否跟上：

```bash
dig YOUR_DOMAIN +short
dig www.YOUR_DOMAIN +short
```

如果权威 DNS 已正确，而公共 DNS 仍是旧值，说明只是缓存尚未刷新。

### 7. 重新启动 Caddy 申请证书

当域名已经真正解析到新服务器后，执行：

```bash
systemctl restart caddy
systemctl status caddy --no-pager
journalctl -u caddy -n 100 --no-pager
```

看到类似下面的日志，一般就说明证书已经成功签发：

- `authorization finalized`
- `valid`
- `certificate obtained successfully`

### 8. 验证 HTTPS

```bash
curl -I https://YOUR_DOMAIN
curl -I https://www.YOUR_DOMAIN
```

浏览器访问：

- `https://YOUR_DOMAIN`
- `https://www.YOUR_DOMAIN`

### 9. 成功后再下线旧服务器

只有在下面三项都成立时，才建议关掉旧服务器：

1. 新服务器 `health` 正常
2. 域名已经解析到新服务器
3. HTTPS 已经可用

### 10. 收口公网端口

如果新服务器已经通过 `Caddy` 提供服务，建议关闭公网 `8080`，只保留：

- `22`
- `80`
- `443`

例如：

```bash
ufw delete allow 8080/tcp
ufw status
```

## 最小停机迁移 SOP

如果旧服务器还在跑，想把停机控制到最短：

1. 在旧服务器确认服务正常运行
2. 执行打包脚本生成迁移包
3. 把迁移包传到新服务器
4. 在新服务器恢复并启动
5. 在新服务器准备好 `Caddy` 和 `80/443`
6. 切 DNS 到新服务器
7. 等 HTTPS 正常后再停旧服务器

如果将来用了域名：

1. 新服务器先恢复并验证
2. 准备反向代理和 HTTPS 配置
3. 再切 DNS 到新服务器
4. 等流量稳定、证书正常后下线旧服务器

## 哪些情况可能还要补一次授权

大多数情况下，数据库和数据目录迁过去就够了。

但如果某些上游账号强绑定：

- 设备指纹
- 出口 IP
- 本地浏览器会话

迁到新服务器后，仍可能需要单独补一次授权。这属于上游平台策略，不是 Sub2API 本身迁移失败。

## 建议长期保留

每次迁移或大改前，都保留一份完整归档：

```bash
tar czf sub2api-backup-$(date +%F-%H%M%S).tar.gz deploy
```

如果已经上了域名，也建议额外保留这些信息：

- 当前域名列表
- DNS 记录截图或导出
- `/etc/caddy/Caddyfile`
- 当前服务器公网 IP
- 防火墙开放端口清单

## 推荐目录

建议服务器统一放在：

```text
/root/deploy
```

这样以后每次换机器都可以保持相同路径，操作最省心。
