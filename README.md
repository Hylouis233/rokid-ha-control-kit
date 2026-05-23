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

## Docker

```bash
docker build -t rokid-ha-control-kit .
docker run --rm -p 8080:8080 \
  -e HA_URL="http://homeassistant.local:8123" \
  -e HA_TOKEN="replace-with-home-assistant-token" \
  -e ROKID_AUTH_AK="replace-with-rokid-auth-ak" \
  rokid-ha-control-kit
```

也可以复制 `docker-compose.example.yml` 后按实际环境修改。

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

灵珠 SSE 测试：

```bash
curl -N -X POST http://127.0.0.1:8080/rokid/sse \
  -H 'Authorization: Bearer replace-with-rokid-auth-ak' \
  -H 'Content-Type: application/json' \
  -d '{"text":"打开客厅灯","sessionId":"debug"}'
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
