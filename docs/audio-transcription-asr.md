# ASR / 语音转文本接口文档

本文档说明本项目新增的语音转文本（ASR）接入方式，覆盖 OpenAI、xAI STT、智谱 GLM-ASR 和火山豆包语音识别。

## 支持范围

| 供应商 | 渠道类型 | 推荐模型 | 客户端入口 | 上游接口 |
| --- | --- | --- | --- | --- |
| OpenAI | OpenAI | `gpt-transcribe`、`gpt-4o-transcribe`、`gpt-4o-mini-transcribe`、`gpt-4o-transcribe-diarize`、`whisper-1` | `POST /v1/audio/transcriptions` | `/v1/audio/transcriptions` |
| xAI | xAI | `grok-stt` | `POST /v1/stt` 或 `POST /v1/audio/transcriptions` | `/v1/stt` |
| 智谱 GLM-ASR | Zhipu V4 | `glm-asr-2512`、`glm-asr` | `POST /v1/audio/transcriptions` | `/api/paas/v4/audio/transcriptions` |
| 火山豆包 ASR | VolcEngine | `volc-asr`、`volc-asr-2` | `POST /v1/audio/transcriptions` | `wss://openspeech.bytedance.com/api/v3/sauc/bigmodel_nostream` |

当前实现支持 REST/multipart 文件转录和上游 SSE 转发；xAI 官方 WebSocket 实时 STT（`wss://api.x.ai/v1/stt`）未在本次改造中新增代理入口。

`gpt-live-transcribe` 已加入 OpenAI 模型与倍率元数据，但它主要面向 OpenAI Realtime 转录场景；本次改造没有改变项目现有 Realtime/WebSocket 代理行为。

## 通用认证

所有请求都使用本项目签发的令牌：

```http
Authorization: Bearer <NEW_API_KEY>
```

请求体使用 `multipart/form-data`。如果使用浏览器或 SDK 构造 `FormData`，不要手动固定 `Content-Type`，让客户端自动携带 boundary。

## OpenAI 兼容转录

### 请求

```bash
curl --request POST 'https://<your-new-api-domain>/v1/audio/transcriptions' \
  --header 'Authorization: Bearer <NEW_API_KEY>' \
  --form file=@audio.mp3 \
  --form model=gpt-transcribe
```

常用字段：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `file` | file | 是 | 需要转录的音频文件 |
| `model` | string | 是 | 如 `gpt-transcribe`、`gpt-4o-transcribe`、`whisper-1` |
| `response_format` | string | 否 | `json`、`verbose_json`、`text`、`srt`、`vtt`、`diarized_json` 等，以目标模型实际支持为准 |
| `language` | string | 否 | 语言提示 |
| `prompt` | string | 否 | 转录上下文提示 |
| `stream` | boolean | 否 | `true` 时透传上游 SSE 转录事件；`whisper-1` 不支持流式 |

### 流式示例

```bash
curl --request POST 'https://<your-new-api-domain>/v1/audio/transcriptions' \
  --header 'Authorization: Bearer <NEW_API_KEY>' \
  --form file=@audio.mp3 \
  --form model=gpt-transcribe \
  --form stream=true
```

流式响应会直接转发上游 `text/event-stream`。非流式响应保持上游 JSON 原样返回。

## xAI STT

xAI 原生 STT 使用 `file` 或 `url` 作为音频来源，`file` 和 `url` 二选一。项目内部会确保 multipart 中的 `file` 字段最后写入，以满足 xAI 对 streamable upload 的要求。

### 请求文件

```bash
curl --request POST 'https://<your-new-api-domain>/v1/stt' \
  --header 'Authorization: Bearer <NEW_API_KEY>' \
  --form language=en \
  --form format=true \
  --form 'keyterm=Understand The Universe' \
  --form file=@meeting.mp3
```

### 请求远程 URL

```bash
curl --request POST 'https://<your-new-api-domain>/v1/stt' \
  --header 'Authorization: Bearer <NEW_API_KEY>' \
  --form url='https://example.com/audio.mp3' \
  --form language=en
```

`/v1/stt` 未传 `model` 时会自动使用路由模型 `grok-stt`；该字段只用于本项目渠道选择，不会转发给 xAI 上游。

常用字段：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `file` | file | 二选一 | 本地音频文件，上游限制以 xAI 官方为准 |
| `url` | string | 二选一 | 上游服务器拉取的远程音频 URL |
| `audio_format` | string | 否 | 原始/headerless 音频格式提示，如 `pcm`、`mulaw`、`alaw` |
| `sample_rate` | integer | 否 | 原始音频采样率 |
| `language` | string | 否 | 语言提示，配合 `format=true` 用于格式化 |
| `format` | boolean | 否 | 是否启用文本格式化 |
| `multichannel` | boolean | 否 | 是否分声道转录 |
| `channels` | integer | 否 | 声道数 |
| `diarize` | boolean | 否 | 是否启用说话人分离 |
| `keyterm` | string | 否 | 可重复传入，用于增强专有名词识别 |
| `filler_words` | boolean | 否 | 是否保留填充词 |
| `vad_threshold` | number | 否 | VAD 阈值 |

## 智谱 GLM-ASR

GLM-ASR 接入在 Zhipu V4 渠道上，使用 OpenAI 兼容入口 `/v1/audio/transcriptions`。

### 文件上传

```bash
curl --request POST 'https://<your-new-api-domain>/v1/audio/transcriptions' \
  --header 'Authorization: Bearer <NEW_API_KEY>' \
  --form model=glm-asr-2512 \
  --form stream=false \
  --form file=@audio.mp3
```

### Base64 音频

```bash
curl --request POST 'https://<your-new-api-domain>/v1/audio/transcriptions' \
  --header 'Authorization: Bearer <NEW_API_KEY>' \
  --form model=glm-asr-2512 \
  --form file_base64='<BASE64_AUDIO>' \
  --form stream=false
```

### 流式

```bash
curl --request POST 'https://<your-new-api-domain>/v1/audio/transcriptions' \
  --header 'Authorization: Bearer <NEW_API_KEY>' \
  --form model=glm-asr-2512 \
  --form stream=true \
  --form file=@audio.mp3
```

常用字段：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `model` | string | 是 | `glm-asr-2512` 或管理员配置的上游模型名 |
| `file` | file | 二选一 | 音频文件；智谱官方文档说明 `.wav/.mp3`、文件大小和时长限制以上游为准 |
| `file_base64` | string | 二选一 | 音频 Base64；同时传 `file` 和 `file_base64` 时，上游以 `file` 为准 |
| `prompt` | string | 否 | 之前的转录结果或上下文 |
| `hotwords` | array/string | 否 | 热词；multipart 可按上游要求传递 |
| `stream` | boolean | 否 | `true` 时返回 SSE |
| `request_id` | string | 否 | 请求唯一标识 |
| `user_id` | string | 否 | 终端用户标识 |

## 火山豆包流式语音识别

火山官方接口是 WebSocket 二进制协议。本项目把它封装到 OpenAI 兼容的 multipart 转录入口中：客户端上传完整音频文件，服务端内部建立火山 WebSocket、发送 full client request 和 audio-only 音频分包，等待最后一包结果后返回 OpenAI 风格响应。

默认使用文档推荐的流式输入模式 `bigmodel_nostream`，适合“完整文件上传后拿最终文本”的 REST 代理场景。当前不向客户端输出 SSE；即使传入 `stream=true`，火山原生实现也会返回最终非流式结果。

### 渠道配置

火山控制台展示的 Ark API 基础 URL 可直接填到渠道 `API 基础 URL / 完整接口 URL`；使用项目默认地址时填写中国区 Ark 地址即可。

中国区通常填写：

```text
https://ark.cn-beijing.volces.com
```

BytePlus 东南亚区通常填写：

```text
https://ark.ap-southeast.bytepluses.com
```

对于 `volc-asr` / `volc-asr-2`，无论 Base URL 留空，还是填写上述两个官方 Ark 地址之一，项目都会按火山官方 ASR 文档走内部 WebSocket 地址：

```text
wss://openspeech.bytedance.com/api/v3/sauc/bigmodel_nostream
```

Agent/Coding Plan 的 ASR 专用渠道可直接填写文档给出的完整 WebSocket 地址：

```text
wss://openspeech.bytedance.com/api/v3/plan/sauc/bigmodel_nostream
```

实时返回中间结果时也可填写双流地址：

```text
wss://openspeech.bytedance.com/api/v3/plan/sauc/bigmodel_async
```

当 45 号 VolcEngine 渠道填写完整的 `ws://` / `wss://` 地址时，项目会把它作为原生 ASR 接口直接使用，不再追加 `/v1/audio/transcriptions`。Agent/Coding Plan 链路会按其文档发送带序号的 Gzip 二进制帧，并默认使用 `volc.seedasr.sauc.duration`；渠道 Key 应填写 Agent/Coding Plan 的专属 API Key。

完整的 OpenAI 兼容 HTTP 转录地址（路径以 `/audio/transcriptions` 结尾）也可直接填写。其他情况下，自定义 HTTP(S) 地址仍按 Base URL 处理并自动追加 `/v1/audio/transcriptions`。

VolcEngine 渠道 Key 支持两种格式：

| Key 格式 | 说明 |
| --- | --- |
| `<X-Api-Key>` | 新版火山控制台 API Key，推荐 |
| `<APP ID>\|<Access Token>` | 旧版控制台鉴权，对应火山请求头 `X-Api-App-Key` / `X-Api-Access-Key` |
| `<APP ID>\|<Access Token>\|<Secret Key>` | 兼容火山控制台同时展示三项认证信息的填写方式；ASR WebSocket 只会使用前两段，`Secret Key` 不会外发 |

如果控制台页面展示的是 “APP ID / Access Token / Secret Key”，后台渠道 Key 可填：

```text
<APP ID>|<Access Token>|<Secret Key>
```

也可只填实际需要的前两段：

```text
<APP ID>|<Access Token>
```

默认 Resource ID：

| 模型 | 默认 Resource ID | 说明 |
| --- | --- | --- |
| `volc-asr` | `volc.bigasr.sauc.duration` | 豆包流式语音识别 1.0 小时版 |
| `volc-asr-2` | `volc.seedasr.sauc.duration` | 豆包流式语音识别 2.0 小时版 |

Agent/Coding Plan ASR 目前只提供豆包流式语音识别模型 2.0，因此配置 `/api/v3/plan/sauc/` 完整地址时，无显式覆盖的 Resource ID 固定为 `volc.seedasr.sauc.duration`。

火山豆包流式语音识别模型 2.0 的资源 ID：

| 资源包 | Resource ID |
| --- | --- |
| 小时版 | `volc.seedasr.sauc.duration` |
| 并发版 | `volc.seedasr.sauc.concurrent` |

如果使用并发版资源包，可在请求中传 `resource_id` 覆盖，例如 `volc.seedasr.sauc.concurrent`。

### 请求

```bash
curl --request POST 'https://<your-new-api-domain>/v1/audio/transcriptions' \
  --header 'Authorization: Bearer <NEW_API_KEY>' \
  --form model=volc-asr-2 \
  --form language=zh-CN \
  --form file=@audio.mp3
```

### 常用字段

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `model` | string | 是 | `volc-asr` 或 `volc-asr-2` |
| `file` | file | 是 | 音频文件；当前支持火山文档列出的 `pcm`、`wav`、`ogg`、`mp3` |
| `language` | string | 否 | 语言代码，如 `zh-CN`、`en-US`、`ja-JP`；仅 `bigmodel_nostream` 支持 |
| `audio_format` / `format` | string | 否 | 显式指定音频格式；未传时按文件扩展名推断 |
| `codec` | string | 否 | `raw` 或 `opus`；`ogg` 默认 `opus`，其他默认 `raw` |
| `rate` / `sample_rate` | integer | 否 | 默认 `16000` |
| `bits` | integer | 否 | 默认 `16` |
| `channel` / `channels` | integer | 否 | 默认 `1` |
| `resource_id` | string | 否 | 覆盖默认火山 Resource ID |
| `request_id` | string | 否 | 覆盖火山 `X-Api-Request-Id` |
| `response_format` | string | 否 | `json` / `verbose_json` / `text`；`verbose_json` 会默认开启 `show_utterances` |
| `chunk_size` | integer | 否 | WebSocket 音频分包大小，默认 `8192` 字节，最大 `262144` |

可直接透传的火山请求开关包括：`enable_itn`、`enable_punc`、`show_utterances`、`enable_auto_lang`、`show_speech_rate`、`show_volume`、`enable_lid`、`enable_emotion_detection`、`enable_gender_detection`、`result_type`、`output_zh_variant` 等。

高级参数可通过 `metadata` 传入 JSON，覆盖火山 full client request 的 `user`、`audio`、`request` 子对象：

```bash
curl --request POST 'https://<your-new-api-domain>/v1/audio/transcriptions' \
  --header 'Authorization: Bearer <NEW_API_KEY>' \
  --form model=volc-asr-2 \
  --form file=@meeting.wav \
  --form 'metadata={
    "resource_id": "volc.seedasr.sauc.concurrent",
    "request": {
      "show_utterances": true,
      "result_type": "full"
    }
  }'
```

### 响应

默认返回 JSON：

```json
{
  "text": "这是字节跳动，今日头条母公司。",
  "duration": 3.696,
  "utterances": [
    {
      "text": "这是字节跳动，",
      "start_time": 0,
      "end_time": 1705,
      "definite": true
    }
  ]
}
```

`duration` 单位为秒，由火山 `audio_info.duration` 毫秒值转换而来。`response_format=text` 时只返回纯文本。

## 视频素材识别加速建议

当前 ASR 接口识别的是音频，不直接识别视频画面，也不直接接收 `.mp4` 作为火山 ASR 原生输入。用于漫剧/短剧素材快速理解、台词提取或集数质检时，建议先在本地从视频中抽取音轨，再把音频提交到统一转录接口。

```bash
: "${NEW_API_BASE:?set NEW_API_BASE, e.g. https://<your-new-api-domain>}"
: "${NEW_API_KEY:?set NEW_API_KEY}"
: "${VIDEO_FILE:?set VIDEO_FILE, e.g. work/{别名}/downloads/第1集.mp4}"

mkdir -p "work/{别名}/qa/asr"
audio_file="work/{别名}/qa/asr/第1集.wav"
text_file="work/{别名}/qa/asr/第1集.txt"

ffmpeg -y -i "${VIDEO_FILE}" -vn -ac 1 -ar 16000 -f wav "${audio_file}"

curl --request POST "${NEW_API_BASE}/v1/audio/transcriptions" \
  --header "Authorization: Bearer ${NEW_API_KEY}" \
  --form model=volc-asr-2 \
  --form language=zh-CN \
  --form response_format=text \
  --form file=@"${audio_file}" \
  > "${text_file}"
```

批量处理前 15 集时，可把转写文本保存到 `work/{别名}/qa/asr/第N集.txt`，并在 `sources.json` 的对应 episode 中补充 `transcript_file`、`transcript_model`、`transcript_language` 和 `transcribed_at`。转写结果用于加速理解和复核，不替代 `ffprobe` 视频验收、平台目录顺序核对和人工可见内容判断。

## 计费与用量

- 非流式响应中如上游返回 `usage`，优先使用上游 usage。
- 如响应中有 `duration` 但没有 usage，会按 `ceil(duration_seconds) / 60 * 1000` 折算为音频输入 tokens。
- 流式响应会转发上游 SSE，并尽量从最终事件中的 `usage` 读取用量；没有 usage 时使用预估 tokens 兜底。
- 火山豆包 ASR 的上游 duration 单位为毫秒，项目会转换为秒后按同一规则折算音频输入 tokens。
- 默认模型倍率已补充到模型倍率配置，管理员仍可在后台覆盖具体价格。

## 渠道测试说明

后台渠道测试目前是 JSON 伪请求，不能携带真实 multipart 音频文件。因此选择 `audio-transcription` 端点时，后端会返回“不支持无真实音频文件的渠道测试”的明确错误。实际 ASR 可通过上述 curl 或客户端 SDK 调用验证。

## 参考资料

- OpenAI Transcriptions API：`https://developers.openai.com/api/docs/guides/speech-to-text`
- OpenAI Transcription guide：`https://developers.openai.com/api/docs/guides/transcription`
- xAI Speech to Text：`https://docs.x.ai/docs/guides/speech-to-text`
- 智谱语音转文本 API：`https://docs.bigmodel.cn/api-reference/模型-api/语音转文本.md`
- GLM-ASR-2512：`https://docs.bigmodel.cn/cn/guide/models/sound-and-video/glm-asr-2512.md`
- 火山豆包流式语音识别 WebSocket：`https://docs.volcengine.com/docs/6561/1354869?lang=zh`
