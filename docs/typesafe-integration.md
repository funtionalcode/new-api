# TypeSafe 渠道与前后评估

TypeSafe Jev 提供结构化判断，支持 `noul`、`choice` 和 `score`。它不生成聊天文本。
原生协议见 [API 文档](https://docs.typesafe.ai/api)，模型与上游定价见 [模型文档](https://docs.typesafe.ai/models)。

## 创建渠道

在渠道页面选择 `TypeSafe`，填写 API Key，默认地址为 `https://api.typesafe.ai`。
支持拉取模型和渠道测试。当前模型包括 `jev-latest`、`jev-preview` 和 `jev-1.13.0`。
按官方当前价格配置输入 $0.042 / 百万 token、输出免费；售价可在系统设置中调整。

原生接口使用本系统令牌，保留 TypeSafe 响应的 `model`、`answers`、`usage`：

```sh
curl "$NEW_API_BASE_URL/v1/systemone" \
  -H "Authorization: Bearer $NEW_API_KEY" \
  -H 'Content-Type: application/json' \
  --data '{
    "model": "jev-latest",
    "state": "付款失败，急需处理。",
    "questions": {
      "urgent": {"type": "noul", "instructions": "是否需要紧急处理？"},
      "category": {"type": "choice", "instructions": "选择问题分类", "criteria": {"billing": "付款与账单", "technical": "技术故障"}},
      "severity": {"type": "score", "instructions": "评估紧急程度", "criteria": ["低", "中", "高"]}
    }
  }'
```

## 其他模型渠道引用 TypeSafe

在需要评估的渠道的参数覆盖 JSON 中添加 `_typesafe`。将 `123` 替换为已创建的 TypeSafe 渠道 ID：

```json
{
  "_typesafe": {
    "channel_id": 123,
    "model": "jev-latest",
    "timeout_ms": 5000,
    "max_chars": 12000,
    "before": {
      "category": {
        "type": "choice",
        "instructions": "根据 state.request 判断请求类别。",
        "criteria": {"coding": "编程任务", "translation": "翻译任务", "other": "其他请求"}
      }
    },
    "after": {
      "relevance": {
        "type": "score",
        "instructions": "state.response 是否切合 state.request？",
        "criteria": ["偏离问题", "部分相关", "直接回答问题"]
      }
    }
  }
}
```

- `before`、`after` 可以单独配置，也可以同时配置。每项都是 TypeSafe 的 questions 对象。
- 默认超时为 5 秒，最多 30 秒；默认每个输入或输出文本字段保留前 12000 个字符，最多 60000 个。截断会标记 `truncated`。
- 支持 HTTP Chat Completions、Responses、Claude、Gemini，以及 Responses WebSocket。流式输出保持原样，只采集公开回答文本；纯工具调用无文本时跳过回答评估。
- 请求前评估会增加首字延迟；回答后评估在答案发送后、主请求记账前执行。失败、超时或无权限只记录状态，不拦截或改写主模型回答。
- 引用的 TypeSafe 渠道必须启用、包含所选模型、属于当前分组，并向请求用户开放；用户或令牌启用了模型限制时，也需允许所选 Jev 模型。评估独立计费，使用原请求的用户和令牌额度；费用与主模型分别记录。
- `_typesafe` 由中转本地处理，不发往主模型供应商；它也可以与现有参数和 `operations` 同时使用。
- 评估结果位于使用日志详情的 TypeSafe 评估结果中，仅管理员可见。结果含阶段、状态、回答、截断标记和评估请求 ID，评估日志反向记录主请求 ID。
- 默认不启用任何渠道的前后评估。配置该字段表示将对应输入和输出发送到指定的 TypeSafe 渠道。

此配置用于记录判断结果，不会自动切换模型、拒绝请求或重试低分回答。
