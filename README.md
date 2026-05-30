# Rokid Home Assistant Control Kit

把 Rokid Glasses / 灵珠平台变成 Home Assistant 的自然语言控制入口。

`rokid-ha-control-kit` 提供一个轻量 HTTP 服务：灵珠平台通过 SSE 调用它，它再根据实体别名、规则或 Home Assistant Conversation 调用 HA，实现“戴着眼镜控制家里设备”的体验。

## 功能特性

- **灵珠平台 SSE 入口**：提供 `POST /rokid/sse`，返回 `message`、`done`、`error` events。
- **自然语言控制**：本地实体别名优先命中，未命中时转发 HA Conversation。
- **HA service 直调**：提供 `POST /service-call`，适合 OpenClaw Skill 或其他 Agent 调用。
- **实体列表代理**：提供 `GET /entities`，便于调试和上层智能体发现设备。
- **安全控制**：支持 Auth AK、实体 allowlist、service allowlist、高风险操作二次确认、审计日志。
- **多种部署方式**：支持本地运行、Docker、docker-compose 和 Home Assistant Add-on。

## 接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/health` | 健康检查 |
| `GET` | `/entities` | 代理 Home Assistant `GET /api/states` |
| `POST` | `/service-call` | 调用 Home Assistant service |
| `POST` | `/intent` | 自然语言控制入口 |
| `POST` | `/rokid/sse` | 灵珠平台 SSE 入口 |

## 快速开始

```bash
cp .env.example .env
```

编辑 `.env`：

```env
PORT=8080
HA_URL=http://homeassistant.local:8123
HA_TOKEN=replace-with-home-assistant-token
ROKID_AUTH_AK=replace-with-rokid-auth-ak
ENTITY_ALIASES_FILE=config/entity_aliases.example.json
ALLOWED_ENTITIES=light.living_room,switch.air_purifier
ALLOWED_SERVICES=light.turn_on,light.turn_off,switch.turn_on,switch.turn_off
CONFIRM_TOKEN=replace-with-confirm-token
AUDIT_LOG_FILE=logs/audit.log
```

运行：

```bash
go run .
```

验证：

```bash
curl http://127.0.0.1:8080/health
```

## 配置项

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `PORT` | `8080` | HTTP 服务端口 |
| `HA_URL` | 空 | Home Assistant 地址，例如 `http://homeassistant.local:8123` |
| `HA_TOKEN` | 空 | Home Assistant Long-Lived Access Token |
| `ROKID_AUTH_AK` | 空 | 灵珠平台 Auth AK；为空时仅允许本机测试 |
| `ENTITY_ALIASES_FILE` | `config/entity_aliases.example.json` | 实体别名配置路径 |
| `ALLOWED_ENTITIES` | 空 | 逗号分隔的实体 allowlist；为空时不限制实体 |
| `ALLOWED_SERVICES` | 空 | 逗号分隔的 service allowlist，例如 `light.turn_on` |
| `CONFIRM_TOKEN` | 空 | 高风险 domain 二次确认 token |
| `AUDIT_LOG_FILE` | 空 | 请求审计日志路径；为空时不写审计日志 |

## 自部署指南

### 为什么自部署？

- **隐私安全**：Home Assistant token 和设备配置完全由你自己控制
- **网络隔离**：服务运行在你的本地网络，无需暴露 HA 到公网
- **定制化**：可以根据需求修改代码和配置
- **无依赖**：不依赖第三方云服务

### 部署方式

#### 1. 直接运行（适合开发测试）

```bash
# 克隆仓库
git clone https://github.com/Hylouis233/rokid-ha-control-kit.git
cd rokid-ha-control-kit

# 复制配置文件
cp .env.example .env

# 编辑配置
# HA_URL: 你的 Home Assistant 地址
# HA_TOKEN: 你的 Home Assistant Long-Lived Access Token
# ROKID_AUTH_AK: 生成一个随机密钥

# 运行
go run .
```

#### 2. Docker 部署（推荐生产环境）

```bash
# 克隆仓库
git clone https://github.com/Hylouis233/rokid-ha-control-kit.git
cd rokid-ha-control-kit

# 复制并编辑 docker-compose 配置
cp docker-compose.example.yml docker-compose.yml
# 编辑 docker-compose.yml 中的配置

# 构建并启动
docker-compose up -d
```

#### 3. Home Assistant Add-on（最简单）

如果你使用 Home Assistant OS 或 Supervised，可以安装 Add-on：

1. 在 HA 中打开 "Add-on Store"
2. 添加自定义仓库：`https://github.com/Hylouis233/rokid-ha-control-kit`
3. 安装 "Rokid HA Control Kit" Add-on
4. 配置并启动

### 公网暴露方案

由于灵珠平台需要 HTTPS 回调地址，你需要将服务暴露到公网。以下是几种方案：

#### 方案一：Tailscale Funnel（推荐，最简单）

Tailscale Funnel 可以将本地服务暴露到公网，无需配置域名和证书。

```bash
# 1. 安装 Tailscale
# Windows: https://tailscale.com/download/windows
# macOS: brew install tailscale
# Linux: curl -fsSL https://tailscale.com/install.sh | sh

# 2. 登录 Tailscale
tailscale login

# 3. 开启 Funnel（将本地 8080 端口暴露到公网）
tailscale funnel --bg 8080

# 4. 获取公网 URL
tailscale funnel status
# 会显示类似: https://your-machine.tail12345.ts.net
```

然后在灵珠平台配置：
- SSE URL: `https://your-machine.tail12345.ts.net/rokid/sse`
- Auth AK: 你配置的 `ROKID_AUTH_AK`

#### 方案二：Cloudflare Tunnel（免费，稳定）

Cloudflare Tunnel 提供免费的公网暴露，支持自定义域名。

```bash
# 1. 安装 cloudflared
# Windows: https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/
# macOS: brew install cloudflare/cloudflare/cloudflared
# Linux: https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/

# 2. 登录
cloudflared tunnel login

# 3. 创建隧道
cloudflared tunnel create rokid-ha-control-kit

# 4. 配置路由
cloudflared tunnel route dns rokid-ha-control-kit rokid.yourdomain.com

# 5. 运行隧道
cloudflared tunnel run rokid-ha-control-kit
```

#### 方案三：反向代理 + 动态 DNS

如果你有公网 IP 或使用动态 DNS 服务：

```nginx
# Nginx 配置示例
server {
    listen 443 ssl;
    server_name rokid.yourdomain.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location /rokid/sse {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header Connection "";
        proxy_buffering off;
        proxy_read_timeout 86400;
    }
}
```

### 配置示例

#### 完整配置示例

```env
# 服务端口
PORT=8080

# Home Assistant 配置
HA_URL=http://192.168.1.100:8123
HA_TOKEN=eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9...

# 灵珠平台 Auth AK（建议使用随机生成的密钥）
ROKID_AUTH_AK=your-random-secret-key-here

# 实体别名配置
ENTITY_ALIASES_FILE=config/entity_aliases.json

# 允许控制的实体（逗号分隔，为空则不限制）
ALLOWED_ENTITIES=light.living_room,light.bedroom,switch.air_purifier,climate.living_room

# 允许调用的 service（逗号分隔）
ALLOWED_SERVICES=light.turn_on,light.turn_off,switch.turn_on,switch.turn_off,climate.set_temperature

# 高风险操作确认 token
CONFIRM_TOKEN=your-confirm-token

# 审计日志路径
AUDIT_LOG_FILE=logs/audit.log
```

#### 生成随机 AK

```bash
# Linux/macOS
openssl rand -hex 32

# Windows PowerShell
-join ((1..32) | ForEach-Object { '{0:x2}' -f (Get-Random -Max 256) })

# 或使用在线工具: https://randomkeygen.com/
```

#### 实体别名配置示例

```json
[
  {
    "entity_id": "light.living_room",
    "domain": "light",
    "aliases": ["客厅灯", "大厅灯", "客厅的灯"]
  },
  {
    "entity_id": "light.bedroom",
    "domain": "light",
    "aliases": ["卧室灯", "房间灯", "卧室的灯"]
  },
  {
    "entity_id": "switch.air_purifier",
    "domain": "switch",
    "aliases": ["空气净化器", "净化器", "空气清新器"]
  },
  {
    "entity_id": "climate.living_room",
    "domain": "climate",
    "aliases": ["客厅空调", "空调", "客厅的空调"]
  }
]
```

## 请求示例

自然语言控制：

```bash
curl -X POST http://127.0.0.1:8080/intent \
  -H 'Content-Type: application/json' \
  -d '{"text":"打开客厅灯"}'
```

直接调用 service：

```bash
curl -X POST http://127.0.0.1:8080/service-call \
  -H 'Content-Type: application/json' \
  -d '{"domain":"light","service":"turn_on","service_data":{"entity_id":"light.living_room"}}'
```

灵珠 SSE 测试（简化格式）：

```bash
curl -N -X POST http://127.0.0.1:8080/rokid/sse \
  -H 'Authorization: Bearer replace-with-rokid-auth-ak' \
  -H 'Content-Type: application/json' \
  -d '{"text":"打开客厅灯","sessionId":"debug"}'
```

灵珠 SSE 测试（Lingzhu 格式）：

```bash
curl -N -X POST http://127.0.0.1:8080/rokid/sse \
  -H 'Authorization: Bearer replace-with-rokid-auth-ak' \
  -H 'Content-Type: application/json' \
  -d '{"message_id":"msg-001","agent_id":"rokid-ha-control-kit","message":[{"role":"user","type":"text","text":"打开客厅灯"}]}'
```

## 实体别名

复制并修改 `config/entity_aliases.example.json`：

```json
[
  {
    "entity_id": "light.living_room",
    "domain": "light",
    "aliases": ["客厅灯", "大厅灯"]
  }
]
```

别名命中后会直接调用 HA service，降低延迟并减少 LLM 误解。

## 安全建议

- 不要把 `HA_TOKEN`、`ROKID_AUTH_AK`、`CONFIRM_TOKEN` 写入仓库。
- 生产环境必须设置 `ROKID_AUTH_AK`。
- 不要在 URL query 中传递 AK，使用 `Authorization: Bearer <ak>` 或 `X-Auth-AK`。
- 对公网只暴露 `/rokid/sse`，并使用 HTTPS 反向代理。
- 建议配置 `ALLOWED_ENTITIES` 和 `ALLOWED_SERVICES`，把可控范围限制在灯、开关、空调等低风险设备。
- `lock`、`camera`、`alarm_control_panel`、`person` 等 domain 会触发二次确认，调用时必须带正确 `confirm_token`。
- 开启 `AUDIT_LOG_FILE` 后，每次 service 调用会写入 JSON Lines 审计日志。

## Home Assistant Token 创建

1. 在 Home Assistant 中打开用户头像菜单。
2. 找到 Long-Lived Access Tokens。
3. 创建一个专用于 Rokid 控制套件的 token。
4. 将 token 写入 `HA_TOKEN` 或 Add-on 的 `ha_token` 配置，不要提交到仓库。

## 灵珠平台配置

1. 在灵珠平台创建自定义智能体。
2. 将 SSE URL 配置为 `https://<你的公网域名>/rokid/sse`。
3. Auth AK 填写与 `ROKID_AUTH_AK` 相同的值。
4. 输入字段建议使用文本输入；服务端兼容 `text`、`content`、`query`、`input`。
5. 真实设备联调前，先用 `curl` 验证 `/health` 和 `/rokid/sse`。

## 灵珠平台 Lingzhu 协议支持

服务端已完整支持灵珠平台 Lingzhu 协议格式，包括：

- **请求格式**：自动识别灵珠平台的 `message` 数组格式，从 `role=user`、`type=text` 的消息中提取用户输入。
- **响应格式**：返回灵珠平台期望的 Lingzhu SSE 格式，包含 `role`、`type`、`answer_stream`、`message_id`、`agent_id`、`is_finish` 字段。
- **兼容性**：同时支持简化格式（`text`、`content`、`query`、`input`）和灵珠平台 Lingzhu 格式。

### Lingzhu 请求示例

```json
{
  "message_id": "msg-001",
  "agent_id": "rokid-ha-control-kit",
  "message": [
    {
      "role": "user",
      "type": "text",
      "text": "打开客厅灯"
    }
  ]
}
```

### Lingzhu 响应示例

```text
event: message
data: {"role":"agent","type":"answer","answer_stream":"正在处理 Home Assistant 指令...","message_id":"msg-001","agent_id":"rokid-ha-control-kit","is_finish":false}

event: message
data: {"role":"agent","type":"answer","answer_stream":"已打开客厅灯","message_id":"msg-001","agent_id":"rokid-ha-control-kit","is_finish":true}

event: done
data: "[DONE]"
```

## 反向代理示例

Caddy：

```caddyfile
rokid.example.com {
  reverse_proxy /rokid/sse 127.0.0.1:8080
}
```

Nginx：

```nginx
location /rokid/sse {
  proxy_pass http://127.0.0.1:8080;
  proxy_http_version 1.1;
  proxy_set_header Host $host;
  proxy_set_header Connection "";
  proxy_buffering off;
}
```

## 故障排查

| 现象 | 检查项 |
| --- | --- |
| `/rokid/sse` 返回 unauthorized | 检查 `ROKID_AUTH_AK` 和请求头是否一致 |
| Home Assistant 返回 401 | 检查 `HA_TOKEN` 是否为有效 Long-Lived Access Token |
| 命中别名但没有执行 | 检查 `ALLOWED_ENTITIES` 和 `ALLOWED_SERVICES` 是否包含该实体和 service |
| 门锁/摄像头相关操作被拒绝 | 高风险 domain 需要传入正确 `confirm_token` |
| SSE 没有持续输出 | 检查反向代理是否关闭 buffering，并确认使用 `curl -N` 测试 |
| 审计日志没有生成 | 检查 `AUDIT_LOG_FILE` 是否配置且目录可写 |

## 开发

```bash
go fmt ./...
go test ./...
go build ./...
```

## 许可证

MIT。
