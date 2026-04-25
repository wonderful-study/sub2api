# 在线体验交付文档

## 交付范围

在线体验为普通用户面板新增一个可直接使用的 Playground 页面，用户无需手动创建 API Key，即可在浏览器内使用当前账号已授权的 OpenAI 分组完成：

1. 对话
2. 文生图
3. 图生图

该能力不是从 GPT2API 直接移植，而是在 Sub2API 内新增用户态兼容入口，复用项目现有 OpenAI 网关、账号调度、计费、订阅、限流和使用记录链路。

## 用户入口

前端页面入口：

- `/online-experience`

普通用户登录后可从侧边栏和仪表盘快捷入口进入。页面风格、主题、布局和组件均复用 Sub2API 用户面板体系。

页面主要区域：

1. 顶部摘要：共享余额、当前分组、文字模型数量、图片模型数量。
2. 分组选择：只展示当前用户可用的 `platform=openai` 分组。
3. 对话：支持模型选择、系统提示词、流式输出、停止、重试、清空。
4. 文生图：支持图片模型、比例、张数、质量和 prompt。
5. 图生图：支持图片上传、图片模型、比例、质量和编辑 prompt。

## 权限与额度

在线体验与普通 API Key 使用同一套用户资源模型：

1. 共享用户余额。
2. 共享用户可用分组。
3. 共享订阅限制。
4. 共享分组倍率。
5. 共享用户限流和账号调度。
6. 使用记录写入统一 usage log。

用户不需要额外授权，也不需要先创建普通 API Key。

当用户没有可用 OpenAI 分组时，页面展示空状态，不创建内部 Key，也不允许发起体验请求。

## 技术架构

在线体验采用“JWT 用户态入口 + 隐藏内部 API Key + 现有 OpenAI 网关”的结构。

请求链路：

1. 浏览器使用登录态 JWT 调用 `/api/v1/online-experience/*`。
2. 后端根据当前用户和请求中的 `group_id` 校验分组权限。
3. 后端按 `user_id + group_id` 获取或创建隐藏内部 API Key。
4. 后端把内部 API Key、用户、分组、订阅上下文写入 Gin context。
5. 请求继续进入现有 OpenAI 网关 Handler。
6. 网关完成账号调度、上游转发、计费、扣费、限流和使用记录。

核心代码位置：

- 后端在线体验入口：`backend/internal/onlineexperience/`
- 后端用户路由：`backend/internal/server/routes/user.go`
- 隐藏内部 Key 规则：`backend/internal/service/api_key_internal.go`
- 在线体验内部 Key 创建：`backend/internal/service/api_key_online_experience.go`
- 前端在线体验模块：`frontend/src/modules/online-experience/`
- 前端路由：`frontend/src/router/index.ts`
- 侧边栏入口：`frontend/src/components/layout/AppSidebar.vue`
- 仪表盘快捷入口：`frontend/src/components/user/dashboard/UserDashboardQuickActions.vue`

## 后端接口

所有接口都要求普通用户 JWT 登录态。

### 获取分组

```http
GET /api/v1/online-experience/groups
```

返回当前用户可用的 OpenAI 分组。响应字段使用普通用户 DTO 契约，字段名为 `id`、`name`、`platform`、`status` 等小写 JSON 字段。

该契约由 E2E 测试 `TestOnlineExperienceGroupJSONContract` 覆盖，防止后端直接返回 Go 结构体字段 `ID`、`Name`、`Platform` 导致前端下拉为空。

### 获取模型

```http
GET /api/v1/online-experience/models?group_id=<group_id>
```

模型列表优先来自当前分组可用模型；当分组没有账号级模型映射时，回退到 OpenAI 默认模型列表。

返回模型分为：

1. `chat`：对话模型。
2. `image`：图片模型，当前识别 `gpt-image-*`。

### 对话

```http
POST /api/v1/online-experience/chat
Content-Type: application/json
```

请求体携带：

```json
{
  "group_id": 2,
  "model": "chatgpt-4o-latest",
  "messages": [
    { "role": "user", "content": "hello" }
  ],
  "stream": true
}
```

最终进入 `OpenAIGatewayHandler.ChatCompletions`，复用现有聊天网关。

### 文生图

```http
POST /api/v1/online-experience/images/generations
Content-Type: application/json
```

请求体携带：

```json
{
  "group_id": 2,
  "model": "gpt-image-1",
  "prompt": "A small blue cube on a clean desk.",
  "size": "1024x1024",
  "quality": "high",
  "n": 1
}
```

最终进入 `OpenAIGatewayHandler.Images`。

说明：

1. API Key 类型 OpenAI 账号会转发到官方 Images API：`/v1/images/generations`。
2. OAuth/Codex 类 OpenAI 账号会由 Sub2API 内部转换成 Responses API + `image_generation` tool。
3. 在线体验本身不直接实现图片生成协议，只复用项目现有图片网关能力。

### 图生图

```http
POST /api/v1/online-experience/images/edits
Content-Type: multipart/form-data
```

表单字段：

1. `group_id`
2. `model`
3. `prompt`
4. `size`
5. `quality`
6. `image`

最终进入 `OpenAIGatewayHandler.Images`，复用现有图片编辑链路。

## 内部 API Key

在线体验使用隐藏的持久化内部 API Key，命名规则：

```text
__online_experience__:<group_id>
```

行为约束：

1. 按 `user_id + group_id` 维度隔离。
2. 普通用户 API Key 列表不展示。
3. 普通用户 API Key 搜索不返回。
4. 不向前端暴露内部 Key 明文。
5. 使用记录保留真实 `api_key_id`，但展示名称转换为“在线体验”。

该设计保证在线体验能够复用现有网关身份、计费和审计链路，同时不污染用户手动创建的 API Key 列表。

## 计费与使用记录

在线体验请求完成后由现有 `OpenAIGatewayService.RecordUsage` 写入使用记录并扣费。

计费来源：

1. 对话按现有模型 token 计费。
2. 图片请求优先按图片计费路径处理。
3. 如果配置了渠道或分组图片价格，优先使用配置价格。
4. 如果没有配置，则回退到项目当前默认图片价格逻辑。

图片生成消耗建议在生产环境显式配置渠道或分组图片价格，避免默认兜底价与上游官方价格不一致。

## 配置与部署

在线体验不需要单独服务。部署当前 Sub2API 后，前后端路由随主应用一起生效。

Docker 本地开发部署参考：

- `deploy/MIGRATION_CN.md`
- `deploy/docker-compose.dev.yml`

E2E 测试参数可配置在：

- `deploy/.env`
- `deploy/.env.example`

关键参数：

```env
E2E_BASE_URL=http://127.0.0.1:8080
ONLINE_EXPERIENCE_USER_EMAIL=user@example.com
ONLINE_EXPERIENCE_USER_PASSWORD=password
ONLINE_EXPERIENCE_GROUP_ID=
ONLINE_EXPERIENCE_REQUIRE_UPSTREAM=false
```

## 验收测试

自动化测试文件：

- `backend/internal/integration/e2e_online_experience_test.go`

运行方式：

```bash
./deploy/run-online-experience-e2e.sh
```

覆盖场景：

1. 未登录用户访问在线体验接口返回 `401`。
2. 已登录用户请求缺少 `group_id` 时返回 OpenAI 兼容错误。
3. 分组 JSON 契约使用小写字段，前端可正确渲染分组下拉。
4. 已登录用户可获取可用 OpenAI 分组。
5. 已登录用户可基于分组加载模型列表。
6. 对话请求进入共享网关上下文。
7. 文生图请求进入共享网关上下文。
8. 图生图请求进入共享网关上下文。
9. 在线体验内部 API Key 不暴露在普通用户 API Key 列表。

默认情况下，E2E 允许上游不可用场景。当请求已经进入共享鉴权和网关上下文后，如果本地环境没有可用余额、订阅、账号或上游容量，可能返回 `403`、`429`、`502` 或 `503`。如需强制真实上游成功，设置：

```env
ONLINE_EXPERIENCE_REQUIRE_UPSTREAM=true
```

## 已知限制

1. 当前在线体验只面向 OpenAI 分组，不展示 Anthropic、Gemini、Antigravity 等其他平台分组。
2. 图片模型类型按 `gpt-image-*` 识别，新增非该命名格式的图片模型时需要同步模型分类规则。
3. 图片生成的最终费用取决于上游返回、渠道定价、分组图片价格和默认兜底价，生产环境建议显式配置图片价格。
4. 在线体验前端不保存历史会话，刷新页面后仅保留上次选择的分组和 tab。

## 交付结论

在线体验已作为 Sub2API 的独立模块交付：

1. 前端功能隔离在 `frontend/src/modules/online-experience/`。
2. 后端入口隔离在 `backend/internal/onlineexperience/`。
3. 主工程只保留必要路由、导航和依赖接线。
4. 业务能力复用现有网关，后续 Sub2API 网关升级后在线体验可自然继承。
5. 自动化 E2E 覆盖核心用户故事和关键 JSON 契约。
