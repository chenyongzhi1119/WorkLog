package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"log"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

var DB *sql.DB

func Init(path string) error {
	var err error
	DB, err = sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(on)")
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}
	// Execute each statement individually so existing DBs get new tables added.
	for _, stmt := range strings.Split(schema, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err = DB.Exec(stmt); err != nil {
			return fmt.Errorf("apply schema stmt [%.60s]: %w", stmt, err)
		}
	}
	// Explicit migrations for columns/tables added after initial release.
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS daily_tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date TEXT NOT NULL,
			content TEXT NOT NULL,
			done INTEGER DEFAULT 0,
			sort_order INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT DEFAULT '',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS work_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// ALTER TABLE ignores errors if column already exists (caught below)
		`ALTER TABLE goals ADD COLUMN progress INTEGER DEFAULT 0`,
	}
	for _, m := range migrations {
		if _, err = DB.Exec(m); err != nil {
			// ALTER TABLE ADD COLUMN fails if column already exists — safe to ignore
			if !strings.Contains(err.Error(), "duplicate column") &&
				!strings.Contains(err.Error(), "already exists") {
				return fmt.Errorf("migration failed [%.60s]: %w", m, err)
			}
		}
	}
	log.Printf("database ready at %s", path)
	return nil
}

// Goal types

type Goal struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Level       string  `json:"level"`
	ParentID    *int64  `json:"parent_id"`
	Status      string  `json:"status"`
	Progress    int     `json:"progress"`
	CreatedAt   string  `json:"created_at"`
	Children    []*Goal `json:"children,omitempty"`
}

func ListGoals() ([]*Goal, error) {
	rows, err := DB.Query(`SELECT id, title, description, level, parent_id, status, progress, created_at FROM goals WHERE status='active' ORDER BY level, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var goals []*Goal
	for rows.Next() {
		g := &Goal{}
		if err := rows.Scan(&g.ID, &g.Title, &g.Description, &g.Level, &g.ParentID, &g.Status, &g.Progress, &g.CreatedAt); err != nil {
			return nil, err
		}
		goals = append(goals, g)
	}
	return goals, nil
}

func CreateGoal(title, description, level string, parentID *int64) (*Goal, error) {
	res, err := DB.Exec(`INSERT INTO goals(title,description,level,parent_id) VALUES(?,?,?,?)`, title, description, level, parentID)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &Goal{ID: id, Title: title, Description: description, Level: level, ParentID: parentID, Status: "active"}, nil
}

func UpdateGoal(id int64, title, description, status string) error {
	_, err := DB.Exec(`UPDATE goals SET title=?, description=?, status=? WHERE id=?`, title, description, status, id)
	return err
}

func UpdateGoalProgress(id int64, progress int) error {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	_, err := DB.Exec(`UPDATE goals SET progress=? WHERE id=?`, progress, id)
	return err
}

// DailyReport types

type DailyReport struct {
	ID         int64  `json:"id"`
	Date       string `json:"date"`
	Completed  string `json:"completed"`
	Plan       string `json:"plan"`
	Issues     string `json:"issues"`
	AIFeedback string `json:"ai_feedback"`
	CreatedAt  string `json:"created_at"`
}

func UpsertDailyReport(date, completed, plan, issues string) (*DailyReport, error) {
	_, err := DB.Exec(`INSERT INTO daily_reports(date,completed,plan,issues) VALUES(?,?,?,?)
		ON CONFLICT(date) DO UPDATE SET completed=excluded.completed, plan=excluded.plan, issues=excluded.issues`,
		date, completed, plan, issues)
	if err != nil {
		return nil, err
	}
	return GetDailyReport(date)
}

func GetDailyReport(date string) (*DailyReport, error) {
	r := &DailyReport{}
	err := DB.QueryRow(`SELECT id,date,completed,plan,issues,ai_feedback,created_at FROM daily_reports WHERE date=?`, date).
		Scan(&r.ID, &r.Date, &r.Completed, &r.Plan, &r.Issues, &r.AIFeedback, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return r, err
}

func ListDailyReports(month string) ([]*DailyReport, error) {
	rows, err := DB.Query(`SELECT id,date,completed,plan,issues,ai_feedback,created_at FROM daily_reports WHERE date LIKE ? ORDER BY date DESC`, month+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reports []*DailyReport
	for rows.Next() {
		r := &DailyReport{}
		if err := rows.Scan(&r.ID, &r.Date, &r.Completed, &r.Plan, &r.Issues, &r.AIFeedback, &r.CreatedAt); err != nil {
			return nil, err
		}
		reports = append(reports, r)
	}
	return reports, nil
}

func UpdateDailyFeedback(date, feedback string) error {
	_, err := DB.Exec(`UPDATE daily_reports SET ai_feedback=? WHERE date=?`, feedback, date)
	return err
}

func YesterdayReport() (*DailyReport, error) {
	r := &DailyReport{}
	err := DB.QueryRow(`SELECT id,date,completed,plan,issues,ai_feedback,created_at FROM daily_reports WHERE date=date('now','-1 day')`).
		Scan(&r.ID, &r.Date, &r.Completed, &r.Plan, &r.Issues, &r.AIFeedback, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return r, err
}

func RecentDailyReports(n int) ([]*DailyReport, error) {
	rows, err := DB.Query(`SELECT id,date,completed,plan,issues,ai_feedback,created_at FROM daily_reports ORDER BY date DESC LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reports []*DailyReport
	for rows.Next() {
		r := &DailyReport{}
		if err := rows.Scan(&r.ID, &r.Date, &r.Completed, &r.Plan, &r.Issues, &r.AIFeedback, &r.CreatedAt); err != nil {
			return nil, err
		}
		reports = append(reports, r)
	}
	return reports, nil
}

// WeeklyReport / MonthlyReport

type WeeklyReport struct {
	ID            int64  `json:"id"`
	WeekStart     string `json:"week_start"`
	Content       string `json:"content"`
	AutoGenerated bool   `json:"auto_generated"`
	CreatedAt     string `json:"created_at"`
}

func UpsertWeeklyReport(weekStart, content string, auto bool) error {
	autoInt := 0
	if auto {
		autoInt = 1
	}
	_, err := DB.Exec(`INSERT INTO weekly_reports(week_start,content,auto_generated) VALUES(?,?,?)
		ON CONFLICT(week_start) DO UPDATE SET content=excluded.content, auto_generated=excluded.auto_generated`,
		weekStart, content, autoInt)
	return err
}

func ListWeeklyReports(limit int) ([]*WeeklyReport, error) {
	rows, err := DB.Query(`SELECT id,week_start,content,auto_generated,created_at FROM weekly_reports ORDER BY week_start DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reports []*WeeklyReport
	for rows.Next() {
		r := &WeeklyReport{}
		var autoInt int
		if err := rows.Scan(&r.ID, &r.WeekStart, &r.Content, &autoInt, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.AutoGenerated = autoInt == 1
		reports = append(reports, r)
	}
	return reports, nil
}

func DailyReportsByWeek(weekStart string) ([]*DailyReport, error) {
	// weekStart is Monday; get 7 days
	rows, err := DB.Query(`SELECT id,date,completed,plan,issues,ai_feedback,created_at FROM daily_reports
		WHERE date >= ? AND date < date(?, '+7 days') ORDER BY date`, weekStart, weekStart)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reports []*DailyReport
	for rows.Next() {
		r := &DailyReport{}
		if err := rows.Scan(&r.ID, &r.Date, &r.Completed, &r.Plan, &r.Issues, &r.AIFeedback, &r.CreatedAt); err != nil {
			return nil, err
		}
		reports = append(reports, r)
	}
	return reports, nil
}

type MonthlyReport struct {
	ID            int64  `json:"id"`
	Month         string `json:"month"`
	Content       string `json:"content"`
	AutoGenerated bool   `json:"auto_generated"`
	CreatedAt     string `json:"created_at"`
}

func UpsertMonthlyReport(month, content string, auto bool) error {
	autoInt := 0
	if auto {
		autoInt = 1
	}
	_, err := DB.Exec(`INSERT INTO monthly_reports(month,content,auto_generated) VALUES(?,?,?)
		ON CONFLICT(month) DO UPDATE SET content=excluded.content, auto_generated=excluded.auto_generated`,
		month, content, autoInt)
	return err
}

func ListMonthlyReports(limit int) ([]*MonthlyReport, error) {
	rows, err := DB.Query(`SELECT id,month,content,auto_generated,created_at FROM monthly_reports ORDER BY month DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reports []*MonthlyReport
	for rows.Next() {
		r := &MonthlyReport{}
		var autoInt int
		if err := rows.Scan(&r.ID, &r.Month, &r.Content, &autoInt, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.AutoGenerated = autoInt == 1
		reports = append(reports, r)
	}
	return reports, nil
}

func WeeklyReportsByMonth(month string) ([]*WeeklyReport, error) {
	rows, err := DB.Query(`SELECT id,week_start,content,auto_generated,created_at FROM weekly_reports WHERE week_start LIKE ? ORDER BY week_start`, month+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reports []*WeeklyReport
	for rows.Next() {
		r := &WeeklyReport{}
		var autoInt int
		if err := rows.Scan(&r.ID, &r.WeekStart, &r.Content, &autoInt, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.AutoGenerated = autoInt == 1
		reports = append(reports, r)
	}
	return reports, nil
}

// Mentor memory

func GetMentorMemory() (map[string]string, error) {
	rows, err := DB.Query(`SELECT key, value FROM mentor_memory`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		m[k] = v
	}
	return m, nil
}

func SetMentorMemoryKey(key, value string) error {
	_, err := DB.Exec(`INSERT INTO mentor_memory(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=CURRENT_TIMESTAMP`, key, value)
	return err
}

// Mentor conversations

type Conversation struct {
	ID        int64  `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

func AddConversation(role, content string) error {
	_, err := DB.Exec(`INSERT INTO mentor_conversations(role,content) VALUES(?,?)`, role, content)
	return err
}

func RecentConversations(n int) ([]*Conversation, error) {
	rows, err := DB.Query(`SELECT id,role,content,created_at FROM (SELECT id,role,content,created_at FROM mentor_conversations ORDER BY id DESC LIMIT ?) ORDER BY id`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var convs []*Conversation
	for rows.Next() {
		c := &Conversation{}
		if err := rows.Scan(&c.ID, &c.Role, &c.Content, &c.CreatedAt); err != nil {
			return nil, err
		}
		convs = append(convs, c)
	}
	return convs, nil
}

func ClearConversations() error {
	_, err := DB.Exec(`DELETE FROM mentor_conversations`)
	return err
}

// Daily plans

type DailyPlan struct {
	ID        int64  `json:"id"`
	Date      string `json:"date"`
	Tasks     string `json:"tasks"`
	Confirmed bool   `json:"confirmed"`
	CreatedAt string `json:"created_at"`
}

func UpsertDailyPlan(date, tasks string) error {
	_, err := DB.Exec(`INSERT INTO daily_plans(date,tasks) VALUES(?,?) ON CONFLICT(date) DO UPDATE SET tasks=excluded.tasks, confirmed=0`, date, tasks)
	return err
}

func GetDailyPlan(date string) (*DailyPlan, error) {
	p := &DailyPlan{}
	var confirmedInt int
	err := DB.QueryRow(`SELECT id,date,tasks,confirmed,created_at FROM daily_plans WHERE date=?`, date).
		Scan(&p.ID, &p.Date, &p.Tasks, &confirmedInt, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	p.Confirmed = confirmedInt == 1
	return p, err
}

// Settings

func GetSetting(key string) (string, error) {
	var v string
	err := DB.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

func SetSetting(key, value string) error {
	_, err := DB.Exec(`INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=CURRENT_TIMESTAMP`, key, value)
	return err
}

func GetAllSettings(keys []string) (map[string]string, error) {
	m := make(map[string]string, len(keys))
	for _, k := range keys {
		v, err := GetSetting(k)
		if err != nil {
			return nil, err
		}
		m[k] = v
	}
	return m, nil
}

func ConfirmDailyPlan(date string) error {
	_, err := DB.Exec(`UPDATE daily_plans SET confirmed=1 WHERE date=?`, date)
	return err
}

// WorkLog — quick on-the-fly work entries

type WorkLog struct {
	ID        int64  `json:"id"`
	Date      string `json:"date"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

func AddWorkLog(date, content string) (*WorkLog, error) {
	res, err := DB.Exec(`INSERT INTO work_logs(date,content) VALUES(?,?)`, date, content)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &WorkLog{ID: id, Date: date, Content: content}, nil
}

func ListWorkLogs(date string) ([]*WorkLog, error) {
	rows, err := DB.Query(`SELECT id,date,content,created_at FROM work_logs WHERE date=? ORDER BY id`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var logs []*WorkLog
	for rows.Next() {
		w := &WorkLog{}
		if err := rows.Scan(&w.ID, &w.Date, &w.Content, &w.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, w)
	}
	return logs, nil
}

// DailyTask — manual todo items

type DailyTask struct {
	ID        int64  `json:"id"`
	Date      string `json:"date"`
	Content   string `json:"content"`
	Done      bool   `json:"done"`
	SortOrder int    `json:"sort_order"`
	CreatedAt string `json:"created_at"`
}

func ListDailyTasks(date string) ([]*DailyTask, error) {
	rows, err := DB.Query(`SELECT id,date,content,done,sort_order,created_at FROM daily_tasks WHERE date=? ORDER BY sort_order,id`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []*DailyTask
	for rows.Next() {
		t := &DailyTask{}
		var done int
		if err := rows.Scan(&t.ID, &t.Date, &t.Content, &done, &t.SortOrder, &t.CreatedAt); err != nil {
			return nil, err
		}
		t.Done = done == 1
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func AddDailyTask(date, content string) (*DailyTask, error) {
	var maxOrder int
	_ = DB.QueryRow(`SELECT COALESCE(MAX(sort_order),0) FROM daily_tasks WHERE date=?`, date).Scan(&maxOrder)
	res, err := DB.Exec(`INSERT INTO daily_tasks(date,content,sort_order) VALUES(?,?,?)`, date, content, maxOrder+1)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &DailyTask{ID: id, Date: date, Content: content, Done: false, SortOrder: maxOrder + 1}, nil
}

func ToggleDailyTask(id int64, done bool) error {
	d := 0
	if done { d = 1 }
	_, err := DB.Exec(`UPDATE daily_tasks SET done=? WHERE id=?`, d, id)
	return err
}

func DeleteDailyTask(id int64) error {
	_, err := DB.Exec(`DELETE FROM daily_tasks WHERE id=?`, id)
	return err
}

func DeleteWorkLog(id int64) error {
	_, err := DB.Exec(`DELETE FROM work_logs WHERE id=?`, id)
	return err
}

// Stats

type StatsData struct {
	MonthCount     int      `json:"month_count"`
	WeekCount      int      `json:"week_count"`
	Streak         int      `json:"streak"`
	TasksTotalWeek int      `json:"tasks_total_week"`
	TasksDoneWeek  int      `json:"tasks_done_week"`
	CalendarDates  []string `json:"calendar_dates"`
}

func GetStats(month, weekStart string) (*StatsData, error) {
	s := &StatsData{}

	_ = DB.QueryRow(`SELECT COUNT(*) FROM daily_reports WHERE date LIKE ?`, month+"%").Scan(&s.MonthCount)
	_ = DB.QueryRow(`SELECT COUNT(*) FROM daily_reports WHERE date >= ? AND date < date(?, '+7 days')`, weekStart, weekStart).Scan(&s.WeekCount)
	_ = DB.QueryRow(`SELECT COUNT(*) FROM daily_tasks WHERE date >= ? AND date < date(?, '+7 days')`, weekStart, weekStart).Scan(&s.TasksTotalWeek)
	_ = DB.QueryRow(`SELECT COUNT(*) FROM daily_tasks WHERE date >= ? AND date < date(?, '+7 days') AND done=1`, weekStart, weekStart).Scan(&s.TasksDoneWeek)

	// Streak: count backwards from today
	s.Streak = 0
	cur := time.Now()
	for {
		dateStr := cur.Format("2006-01-02")
		var cnt int
		_ = DB.QueryRow(`SELECT COUNT(*) FROM daily_reports WHERE date=?`, dateStr).Scan(&cnt)
		if cnt == 0 {
			break
		}
		s.Streak++
		cur = cur.AddDate(0, 0, -1)
	}

	// Calendar dates
	rows, err := DB.Query(`SELECT date FROM daily_reports WHERE date LIKE ? ORDER BY date`, month+"%")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var d string
			if rows.Scan(&d) == nil {
				s.CalendarDates = append(s.CalendarDates, d)
			}
		}
	}
	if s.CalendarDates == nil {
		s.CalendarDates = []string{}
	}
	return s, nil
}
