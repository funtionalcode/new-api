# Docker 容器出网 NAT 修复

当 Docker 以 `--iptables=false` 启动时，即使 `docker-compose.yml` 中启用了 `com.docker.network.bridge.enable_ip_masquerade: "true"`，Docker 也不会自动写入容器网络的 `MASQUERADE` 规则。此时容器访问外网或访问内网代理可能会超时。

常见现象：

- 宿主机 `curl -x socks5://... https://bigmodel.cn/...` 成功；
- `new-api` 容器内或同网络命名空间内请求同一个代理超时；
- GLM/DeepSeek 额度刷新报 `context deadline exceeded`，并提示经代理请求失败。

## 一次性修复

在宿主机执行：

```bash
sudo ./scripts/ensure-docker-masquerade.sh new-api_new-api-network
```

脚本会读取 Docker network 的实际 IPv4 subnet，并幂等补充：

```bash
iptables -t nat -A POSTROUTING -s <subnet> ! -d <subnet> -j MASQUERADE
```

## 开机持久化

如果 Docker 服务长期使用 `--iptables=false`，建议加一个 systemd oneshot 服务：

```ini
[Unit]
Description=Ensure new-api docker network masquerade
After=docker.service
Requires=docker.service

[Service]
Type=oneshot
WorkingDirectory=/data/haogege/gpt/new-api
ExecStart=/data/haogege/gpt/new-api/scripts/ensure-docker-masquerade.sh new-api_new-api-network
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
```

保存为 `/etc/systemd/system/new-api-docker-masquerade.service` 后执行：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now new-api-docker-masquerade.service
```

## 验证

使用和 `new-api` 相同的网络命名空间验证代理可达：

```bash
docker run --rm --network container:new-api curlimages/curl:8.8.0 \
  -sS -m 20 -o /tmp/out \
  -w 'http_code=%{http_code} time_total=%{time_total} remote_ip=%{remote_ip}\n' \
  -x 'socks5h://user:password@proxy-host:7890' \
  'https://bigmodel.cn/api/monitor/usage/model-usage?type=3'
```

返回 `http_code=200` 或其他明确 HTTP 状态码，说明容器网络已经能通过代理出网。若仍超时，继续检查代理地址、认证和宿主机防火墙。
