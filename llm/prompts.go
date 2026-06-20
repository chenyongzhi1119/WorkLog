package llm

import (
	"fmt"
	"strings"
	"time"
	"worklog/db"
)

// ProfileKeys are the settings keys for user profile
var ProfileKeys = []string{"profile_name","profile_role","profile_team","profile_stage","profile_projects"}

func BuildProfileContext(settings map[string]string) string {
	name     := settings["profile_name"]
	role     := settings["profile_role"]
	team     := settings["profile_team"]
	stage    := settings["profile_stage"]
	projects := settings["profile_projects"]

	if name == "" && role == "" && projects == "" {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("【用户档案】\n")
	if name     != "" { sb.WriteString("姓名：" + name + "\n") }
	if role     != "" { sb.WriteString("岗位：" + role + "\n") }
	if team     != "" { sb.WriteString("团队：" + team + "\n") }
	if stage    != "" { sb.WriteString("阶段：" + stage + "\n") }
	if projects != "" { sb.WriteString("当前项目：" + projects + "\n") }
	sb.WriteString("\n")
	return sb.String()
}

func ReportChatSystem(goals []*db.Goal, yesterdayPlan, todayPlan string) string {
	var sb strings.Builder
	sb.WriteString(`你是用户的职场导师，现在帮他整理今天的工作日报。

通过轻松的对话，引导用户说出今天的工作情况。规则：
- 每次只问 1 个问题，简短，不要长篇大论
- 根据用户的回答追问细节（比如"完成了吗？"、"大概花了多长时间？"）
- 要覆盖到：①今天做了什么 ②完成情况 ③遇到问题 ④明天计划
- 当你觉得信息足够写日报时（通常 4-6 轮对话），最后说：
  "好了，信息差不多了，点击下面「提取日报」按钮，我来帮你整理成正式日报。"
- 语气自然亲切，像朋友聊天，不要像在填表

`)
	if yesterdayPlan != "" {
		sb.WriteString("【昨日计划（用于追问完成情况）】\n" + yesterdayPlan + "\n\n")
	}
	if todayPlan != "" {
		sb.WriteString("【今日时间计划（供参考）】\n" + todayPlan[:min(len(todayPlan), 400)] + "\n\n")
	}
	if len(goals) > 0 {
		sb.WriteString("【用户当前目标】\n")
		lvName := map[string]string{"long_term": "长期", "monthly": "月目标", "weekly": "周目标"}
		for _, g := range goals {
			sb.WriteString("- [" + lvName[g.Level] + "] " + g.Title + "\n")
		}
	}
	sb.WriteString("\n开场提示：结合昨日计划直接问今天完成情况，没有昨日计划就问今天做了什么。")
	return sb.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func ReportChatOpeningUser(yesterdayPlan string) string {
	if yesterdayPlan != "" {
		return "（我来写今天的日报，帮我问问）"
	}
	return "（我来写今天的日报，帮我问问）"
}

func ReportExtractSystem() string {
	return `根据以下对话内容，整理成华为工作日报格式。

严格按以下 JSON 输出，不要有任何其他内容：
{
  "completed": "今日完成内容（编号条目，保留具体细节）",
  "plan":      "明日工作计划（编号条目）",
  "issues":    "问题和风险（无则输出空字符串）"
}

写作要求：
- 保留对话中提到的具体名称、模块名、会议名，不要泛化
- completed 要体现实际完成的事，而非计划
- 语言简洁专业，每条 20-50 字
- 只输出 JSON`
}

func ReportExtractUser(history []Message) string {
	var sb strings.Builder
	sb.WriteString("以下是关于今日工作的对话记录：\n\n")
	for _, m := range history {
		role := "用户"
		if m.Role == "assistant" {
			role = "导师"
		}
		sb.WriteString(role + "：" + m.Content + "\n\n")
	}
	sb.WriteString("请从以上对话中提取日报内容，按要求输出 JSON。")
	return sb.String()
}

func DailyReportGenerateSystem() string {
	return `你是一个华为工作日报助手。用户会提供今天的工作情况（可能包含：做了什么、完成情况、遇到的问题、明天计划，以及昨日计划/今日时间计划作为参考）。

请根据这些信息整理成专业的华为日报格式。

严格按照以下 JSON 输出，不要有任何其他内容：
{
  "completed": "今日完成内容",
  "plan":      "明日工作计划",
  "issues":    "问题和风险"
}

写作要求：
- completed：用编号条目列举，保留用户提到的具体细节（功能名、模块名、会议名等），不要泛化
- plan：用编号条目列举，优先用用户填写的明日计划；若未填写则根据今日工作和参考计划推断
- issues：用用户描述的原话整理；若用户说"无"或未提到，输出空字符串
- 语言简洁专业，每条 20-50 字为宜
- 只输出 JSON，不要 markdown 代码块、不要解释`
}

func DailyReportGenerateUser(notes string) string {
	return "工作备注：\n" + notes
}

func MentorSystemPrompt(memory map[string]string, goals []*db.Goal, yesterdayPlan, todayPlan string, profile map[string]string) string {
	get := func(k string) string {
		if v, ok := memory[k]; ok && v != "" {
			return v
		}
		return ""
	}

	var sb strings.Builder
	sb.WriteString("你是用户在华为工作的专属 AI 导师。你既是职场教练，也是每日工作的协作者。\n\n")
	// Inject user profile
	if profile != nil {
		if ctx := BuildProfileContext(profile); ctx != "" {
			sb.WriteString(ctx)
		}
	}
	sb.WriteString("你的职责：\n")
	sb.WriteString("- 帮用户规划今日工作（制定带时间块的计划）\n")
	sb.WriteString("- 帮用户整理华为日报（通过对话提炼三段式内容）\n")
	sb.WriteString("- 跟踪目标进度，在合适时候提醒和检查\n")
	sb.WriteString("- 提供有针对性的职场指导，基于用户真实情况\n\n")
	sb.WriteString("对话风格：简洁、有行动力、不废话。每次只问或说一件事。\n\n")

	if len(goals) > 0 {
		sb.WriteString("【当前目标】\n")
		lv := map[string]string{"long_term": "长期", "monthly": "月", "weekly": "周"}
		for _, g := range goals {
			sb.WriteString(fmt.Sprintf("- [%s] %s（进度 %d%%）\n", lv[g.Level], g.Title, g.Progress))
		}
		sb.WriteString("\n")
	}
	if yesterdayPlan != "" {
		sb.WriteString("【昨日计划（今天应跟进）】\n" + yesterdayPlan + "\n\n")
	}
	if todayPlan != "" {
		sb.WriteString("【今日已有计划】\n" + todayPlan[:min(len(todayPlan), 500)] + "\n\n")
	}

	if m := get("goal_summary"); m != "" {
		sb.WriteString("【对用户的了解】\n")
		if v := get("goal_summary"); v != "" { sb.WriteString("目标：" + v + "\n") }
		if v := get("work_patterns"); v != "" { sb.WriteString("工作习惯：" + v + "\n") }
		if v := get("strengths"); v != "" { sb.WriteString("优势：" + v + "\n") }
		if v := get("growth_areas"); v != "" { sb.WriteString("成长空间：" + v + "\n") }
		if v := get("recent_insights"); v != "" { sb.WriteString("近期洞察：" + v + "\n") }
		if v := get("project_context"); v != "" { sb.WriteString("项目背景：" + v + "\n") }
	}

	sb.WriteString("\n当用户请求「制定计划」时，输出带时间块的 Markdown 表格（09:00-18:00，午休12:00-13:30）。\n")
	sb.WriteString("当用户请求「整理日报」时，先通过问答了解今天情况，然后输出：\n```json\n{\"completed\":\"...\",\"plan\":\"...\",\"issues\":\"...\"}\n```\n")
	sb.WriteString("当用户描述完今天工作，在 JSON 前说「日报整理好了：」作为信号。\n")

	return sb.String()
}

func PlanningSystemPrompt() string {
	return `你是一位工作规划助手，帮助用户基于长期目标制定今日时间块计划。

输出格式（严格遵守）：

## 今日计划

| 时间 | 任务 | 关联目标 |
|------|------|----------|
| 09:00–10:00 | 任务描述 | 目标名 |
| 10:00–12:00 | 任务描述 | 目标名 |
| 13:30–15:30 | 任务描述 | 目标名 |
| 15:30–17:30 | 任务描述 | 目标名 |
| 17:30–18:00 | 整理日报 / 复盘 | — |

## 今日重点
用 1-2 句话说明今天最重要的事是什么，以及为什么。

规则：
- 工作时间默认 09:00–18:00，午休 12:00–13:30
- 每个时间块 1–2.5 小时，不要太碎
- 优先推进与目标最相关的高价值任务
- 考虑昨日遗留的问题和计划
- 用中文`
}

func WeeklyReportSystemPrompt() string {
	return `你是一位专业的报告撰写助手，负责将华为员工的日报整合为标准周报。

输出格式（严格遵守）：
## 一、本周工作完成情况
[按工作类型分类汇总，突出关键成果]

## 二、下周工作计划
[基于日报中的明日计划汇总]

## 三、问题与风险
[汇总本周遇到的问题和风险]

要求：
- 去重并合并相似内容
- 语言专业、简洁
- 用中文`
}

func MonthlyReportSystemPrompt() string {
	return `你是一位专业的报告撰写助手，负责将华为员工的周报整合为月度报告。

输出格式（严格遵守）：
## 一、本月重点工作完成情况
[按项目/模块分类，突出关键里程碑]

## 二、本月目标达成情况
[对照月初目标的完成度分析]

## 三、下月工作计划
[关键任务和目标]

## 四、问题、风险与建议
[重点问题的根因和改进措施]

要求：
- 语言专业，突出价值和影响
- 用中文`
}

func MentorWeeklySummarySystem() string {
	return `你是一位资深职场导师，正在对用户的本周工作进行深度复盘。

请按以下结构输出本周总结（Markdown 格式）：

## 本周工作概览
简述本周完成的主要工作，1-3 句话。

## 目标推进情况
逐一评估各层级目标的本周进展，指出哪些在推进、哪些被搁置。

## 亮点与优势
本周工作中体现出的优秀之处（具体到事件，不泛泛而谈）。

## 需要关注的问题
本周出现的问题、风险或低效模式，给出具体改进建议。

## 下周建议
3-5 条具体可执行的下周工作建议，优先级排序。

要求：
- 基于用户真实的日报内容，避免空洞评语
- 如果某方面信息不足，直接说"本周未记录相关信息"
- 用中文，语气专业但有温度`
}

func MentorWeeklySummaryUser(reports []*db.DailyReport, goals []*db.Goal, memory map[string]string) string {
	var sb strings.Builder
	sb.WriteString("请对我的本周工作进行复盘分析。\n\n")

	sb.WriteString("【本周日报】\n")
	if len(reports) == 0 {
		sb.WriteString("（本周暂无日报记录）\n")
	} else {
		for _, r := range reports {
			sb.WriteString(fmt.Sprintf("\n%s\n完成：%s\n计划：%s\n问题：%s\n", r.Date, r.Completed, r.Plan, r.Issues))
		}
	}

	sb.WriteString("\n【当前目标】\n")
	if len(goals) == 0 {
		sb.WriteString("（未设置目标）\n")
	} else {
		lvName := map[string]string{"long_term": "长期", "monthly": "月目标", "weekly": "周目标"}
		for _, g := range goals {
			sb.WriteString(fmt.Sprintf("- [%s] %s（进度 %d%%）\n", lvName[g.Level], g.Title, g.Progress))
		}
	}

	if v := memory["recent_insights"]; v != "" {
		sb.WriteString("\n【历史洞察】\n" + v + "\n")
	}
	return sb.String()
}

func MemoryUpdatePrompt(report *db.DailyReport, currentMemory map[string]string) string {
	get := func(k string) string {
		if v, ok := currentMemory[k]; ok {
			return v
		}
		return ""
	}

	return fmt.Sprintf(`请根据以下新提交的日报，更新用户的工作档案。

【新日报 %s】
今日完成：%s
明日计划：%s
问题和风险：%s

【当前档案】
目标规划：%s
工作习惯：%s
优势领域：%s
成长空间：%s
近期洞察：%s
项目背景：%s

请输出更新后的档案，格式为 JSON，只输出 JSON 不要有其他内容：
{
  "goal_summary": "对用户目标状态的简洁描述（如有新信息则更新，否则保留原内容）",
  "work_patterns": "观察到的工作习惯和节奏特点",
  "strengths": "用户展现出的优势和擅长点",
  "growth_areas": "需要改进和提升的方向",
  "recent_insights": "最近的关键发现和洞察（保留最近3-5条）",
  "project_context": "工作项目和团队背景"
}`,
		report.Date,
		report.Completed,
		report.Plan,
		report.Issues,
		get("goal_summary"),
		get("work_patterns"),
		get("strengths"),
		get("growth_areas"),
		get("recent_insights"),
		get("project_context"),
	)
}

func PlanningUserPrompt(goals []*db.Goal, recentReports []*db.DailyReport, date string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("今天是 %s，请帮我规划今日工作计划。\n\n", date))

	sb.WriteString("【我的目标】\n")
	if len(goals) == 0 {
		sb.WriteString("（暂未设置目标）\n")
	} else {
		for _, g := range goals {
			levelName := map[string]string{"long_term": "长期目标", "monthly": "月目标", "weekly": "周目标"}[g.Level]
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", levelName, g.Title))
			if g.Description != "" {
				sb.WriteString(fmt.Sprintf("  %s\n", g.Description))
			}
		}
	}

	sb.WriteString("\n【近期日报】\n")
	if len(recentReports) == 0 {
		sb.WriteString("（暂无日报记录）\n")
	} else {
		for _, r := range recentReports {
			sb.WriteString(fmt.Sprintf("\n%s\n", r.Date))
			if r.Completed != "" {
				sb.WriteString(fmt.Sprintf("完成：%s\n", r.Completed))
			}
			if r.Plan != "" {
				sb.WriteString(fmt.Sprintf("计划：%s\n", r.Plan))
			}
			if r.Issues != "" {
				sb.WriteString(fmt.Sprintf("问题：%s\n", r.Issues))
			}
		}
	}

	sb.WriteString("\n请给出今日工作计划，包含具体任务和预估时间。")
	return sb.String()
}

func WeeklyReportUserPrompt(reports []*db.DailyReport, weekStart string) string {
	weekEnd := func() string {
		t, _ := time.Parse("2006-01-02", weekStart)
		return t.AddDate(0, 0, 6).Format("2006-01-02")
	}()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("请将以下日报整合为 %s 至 %s 的华为标准周报：\n\n", weekStart, weekEnd))
	for _, r := range reports {
		sb.WriteString(fmt.Sprintf("【%s】\n今日完成：%s\n明日计划：%s\n问题风险：%s\n\n", r.Date, r.Completed, r.Plan, r.Issues))
	}
	return sb.String()
}

func MonthlyReportUserPrompt(reports []*db.WeeklyReport, month string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("请将以下周报整合为 %s 的华为标准月报：\n\n", month))
	for _, r := range reports {
		sb.WriteString(fmt.Sprintf("【%s 周】\n%s\n\n", r.WeekStart, r.Content))
	}
	return sb.String()
}
