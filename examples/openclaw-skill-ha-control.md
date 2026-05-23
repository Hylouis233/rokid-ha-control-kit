# OpenClaw Skill 示例：Rokid Home Assistant 控制

这个示例用于让 OpenClaw Agent 通过 `rokid-ha-control-kit` 控制 Home Assistant。

## HTTP 工具

- Base URL：`http://<ha-control-kit-host>:8080`
- 健康检查：`GET /health`
- 自然语言控制：`POST /intent`
- 直接服务调用：`POST /service-call`

## 自然语言控制

```http
POST /intent
Content-Type: application/json

{
  "text": "打开客厅灯"
}
```

当 `config/entity_aliases.example.json` 中存在匹配别名时，会优先走本地规则调用 HA service；否则转发到 Home Assistant Conversation API。

## 直接服务调用

```http
POST /service-call
Content-Type: application/json

{
  "domain": "light",
  "service": "turn_on",
  "service_data": {
    "entity_id": "light.living_room",
    "brightness_pct": 80
  }
}
```

## 灵珠 SSE 入口

在灵珠平台自定义智能体中配置：

- SSE URL：`https://<your-domain>/rokid/sse`
- Auth AK：与服务端环境变量 `ROKID_AUTH_AK` 保持一致
- 输入类型：Text

生产环境建议仅通过反向代理暴露 `/rokid/sse`，不要把 Home Assistant 或内部管理端口直接暴露到公网。
