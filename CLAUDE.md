# WorkLog — 项目文档

> 华为工作日报周报月报 + AI 导师系统
> GitHub: https://github.com/chenyongzhi1119/WorkLog

---

## 启动方式

```bash
# 必须设置 API Key（或在界面里配置）
export ANTHROPIC_API_KEY=sk-ant-...

# 启动（自动创建 worklog.db）
go run .

# 访问
open http://localhost:8090

# 杀掉旧进程再重启
lsof -ti :8090 | xargs kill -9; go run .
```

**端口**：8090（8080 被其他项目占用）

---

## 技术栈

| 层 | 选择 | 说明 |
|---|---|---|
| 后端 | Go 标准库 `net/http` | 零框架，单二进制 |
| 数据库 | `modernc.org/sqlite`（纯 Go） | 无 CGO，文件 `worklog.db` |
| AI SDK | `anthropic-sdk-go v1.51.0` | SSE 流式 + 同步调用 |
| AI 兼容层 | 自实现 OpenAI compat | DeepSeek/Qwen/GLM/OpenAI |
| 前端 | 原生 HTML/CSS/JS | 无框架，embedded 到二进制 |
| 流式 | SSE (Server-Sent Events) | `text/event-stream` |

---

## 项目结构

```
WorkLog/
├── main.go                 # 路由注册、启动、noCache 中间件
├── go.mod / go.sum
│
├── db/
│   ├── schema.sql          # 所有建表语句（CREATE TABLE IF NOT EXISTS）
│   └── db.go               # SQLite 初始化 + 所有 CRUD 函数 + migration
│
├── llm/
│   ├── provider.go         # Provider 接口（StreamChat / Chat）
│   ├── anthropic.go        # AnthropicProvider 实现
│   ├── openai_compat.go    # OpenAICompatProvider（DeepSeek/Qwen 等）
│   ├── manager.go          # ProviderManager（热切换，实现 Provider 接口）
│   └── prompts.go          # 所有 System Prompt 和 User Prompt 构建函数
│
├── handlers/
│   ├── reports.go          # 日报/周报/月报 CRUD + AI 聚合 + 日报对话/提取
│   ├── goals.go            # 目标 CRUD + 进度更新
│   ├── plans.go            # 今日计划（SSE 生成 + 保存 + 确认）
│   ├── mentor.go           # 导师对话（SSE）+ 记忆更新 + 周总结
│   ├── tasks.go            # 今日任务清单 CRUD（勾选/删除）
│   ├── worklogs.go         # 随手记 CRUD
│   ├── settings.go         # AI 供应商配置（读/写 + 热重载）
│   ├── profile.go          # 个人档案（读/写 settings 表）
│   └── stats.go            # Dashboard 统计数据（打卡/任务率/日历）
│
└── frontend/               # embed 到二进制（//go:embed frontend）
    ├── index.html          # 单页应用，5 个 Tab
    ├── style.css           # 亮色企业风格（参考 InterviewPro）
    └── app.js              # 所有前端逻辑（~800 行）
```

---

## 数据库表

| 表 | 用途 |
|---|---|
| `goals` | 目标（long_term/monthly/weekly，含进度 progress） |
| `daily_reports` | 日报（completed/plan/issues/ai_feedback） |
| `weekly_reports` | 周报（可 AI 聚合） |
| `monthly_reports` | 月报（可 AI 聚合） |
| `mentor_memory` | 导师记忆（key-value，6 个维度） |
| `mentor_conversations` | 对话历史（30 条滚动） |
| `daily_plans` | 今日计划（Markdown 文本） |
| `daily_tasks` | 今日任务清单（content/done/sort_order） |
| `work_logs` | 随手记（按日期查询） |
| `settings` | 所有配置（AI 供应商 + 个人档案 + 提醒时间） |

**Migration 策略**：`db.Init()` 逐条执行 schema.sql，并在 `migrations` 数组中追加新表/新列（`ALTER TABLE ADD COLUMN` 的错误被忽略）。

---

## API 路由

```
# 设置
GET/POST  /api/settings              # AI 供应商配置
GET/POST  /api/profile               # 个人档案

# 日报
GET/POST  /api/reports/daily         # 日报列表/提交
POST      /api/reports/daily/generate  # 从笔记 AI 生成日报 JSON
POST      /api/reports/daily/chat    # 导师问答式写日报（SSE）
POST      /api/reports/daily/extract # 从对话提取结构化日报
GET       /api/reports/yesterday     # 昨日日报（用于参考）
GET/POST  /api/reports/weekly        # 周报列表/AI 聚合
GET/POST  /api/reports/monthly       # 月报列表/AI 聚合

# 目标
GET/POST  /api/goals                 # 目标列表/新增
PUT       /api/goals/:id             # 更新（status/progress）

# 计划
GET       /api/plans                 # 获取今日计划
GET       /api/plans/generate        # AI 生成计划（SSE）
POST      /api/plans/save            # 保存计划文本（来自对话）
PUT       /api/plans/confirm         # 确认计划

# 导师
GET       /api/mentor/stream         # 主对话（SSE，含 goals/yesterday/profile）
GET       /api/mentor/weekly-summary # 本周总结（SSE）
GET       /api/mentor/memory         # 导师记忆查看
DELETE    /api/mentor/conversations  # 清空对话历史

# 任务/随手记
GET/POST  /api/tasks                 # 今日任务
PUT/DELETE /api/tasks/:id            # 勾选/删除
GET/POST  /api/worklogs              # 随手记
DELETE    /api/worklogs/:id

# 统计
GET       /api/stats                 # Dashboard 数据（streak/calendar/goals）
```

---

## 核心设计决策

### 1. Provider 热切换
`llm.ProviderManager` 实现了 `Provider` 接口，`handlers` 只依赖接口。切换供应商时调 `pm.Reload()`，不需要重启服务。

### 2. 导师记忆机制
提交日报后**后台异步**调用 Claude（非流式），提取 6 个维度写入 `mentor_memory` 表：
`goal_summary / work_patterns / strengths / growth_areas / recent_insights / project_context`

每次对话时，全部记忆 + 当前目标 + 昨日计划 + 今日计划 + **个人档案** 注入 System Prompt。

### 3. SSE 编码
后端：`json.Marshal(chunk)` 发完整 JSON 字符串（含引号）
前端：`JSON.parse(e.data)` 还原真实文本（解决 `\n` 乱码问题）

### 4. Migration 追加
旧数据库自动追加新表，不需要删库重建：
```go
migrations := []string{
    `CREATE TABLE IF NOT EXISTS settings (...)`,
    `CREATE TABLE IF NOT EXISTS work_logs (...)`,
    `CREATE TABLE IF NOT EXISTS daily_tasks (...)`,
    `ALTER TABLE goals ADD COLUMN progress INTEGER DEFAULT 0`,
}
// ALTER TABLE 已存在时报错被忽略
```

### 5. anthropic-sdk-go v1.51.0 注意事项
- `NewClient()` 返回值类型（非指针）
- 模型常量：`anthropic.ModelClaudeSonnet4_6`
- 流式事件：`event.AsAny().(type)` → `anthropic.ContentBlockDeltaEvent`
- Delta 文本：`event.Delta.AsAny().(anthropic.TextDelta).Text`
- 同步响应内容块：`block.AsAny().(anthropic.TextBlock).Text`

---

## 前端架构

**5 个 Tab（无框架，原生 JS）**：

| Tab | 内容 |
|---|---|
| 对话 | AI 导师主聊天 + 右侧目标进度侧栏 |
| 概览 | Dashboard（打卡统计 + 月历 + 目标总览）|
| 日报 | 华为三段式（今日任务清单 + 明日计划 + 问题风险 + 随手记）|
| 报告 | 历史日报/周报/月报查看 |
| 目标 | 目标 CRUD + 进度条 |

**关键 JS 函数**：
- `sendHomeMsg(msg)` — 主聊天，AbortController 支持停止
- `md(text)` — Markdown 渲染（保护表格块后处理换行，避免 `<br>` 污染）
- `buildCalendar(month, dates)` — 纯 CSS 月历
- `loadOverview()` — Dashboard 数据加载
- `checkReminder()` — 每分钟检查是否触发浏览器通知

**IME 输入法修复**（所有输入框）：
```js
let composing = false;
input.addEventListener('compositionstart', () => composing = true);
input.addEventListener('compositionend',   () => composing = false);
input.addEventListener('keydown', e => {
  if (e.key === 'Enter' && !composing) { /* send */ }
});
```

---

## 已知问题 / 待实现

- [ ] 目标层级树（`parent_id` 后端已支持，前端未接入）
- [ ] 目标截止日期字段
- [ ] 随手记支持编辑（当前只能删除）
- [ ] 任务拖拽排序
- [ ] 知识/成长笔记模块（收藏 AI 回复）
- [ ] Sprint/OKR 周期管理视图
- [ ] 数据导出（CSV/Markdown）
- [ ] macOS .app 打包（参考 InterviewPro 项目）

---

## 个人配置说明

- **GitHub**: https://github.com/chenyongzhi1119/WorkLog
- **本地路径**: `/Users/yongzhichen/Projects/WorkLog`
- **数据库**: `worklog.db`（本地，不上传 git）
- **API Key 存储**: SQLite `settings` 表（不在代码里）
