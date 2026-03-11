package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"goated/internal/db"
)

type Runner struct {
	Store            *db.Store
	WorkspaceDir     string
	LogDir           string
	TelegramNotifier Notifier
}

type Notifier interface {
	SendMessage(ctx context.Context, chatID, text string) error
}

type runRecord struct {
	Minute      string `json:"minute"`
	CronID      uint64 `json:"cron_id"`
	ChatID      string `json:"chat_id"`
	Schedule    string `json:"schedule"`
	Status      string `json:"status"`
	UserMessage string `json:"user_message,omitempty"`
	JobLogPath  string `json:"job_log_path"`
}

func (r *Runner) Run(ctx context.Context, now time.Time) error {
	nowMinute := now.UTC().Truncate(time.Minute)
	jobs, err := r.dueJobs(nowMinute)
	if err != nil {
		return err
	}
	if len(jobs) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Join(r.LogDir, "cron", "jobs"), 0o755); err != nil {
		return fmt.Errorf("mkdir cron jobs log dir: %w", err)
	}

	records := make([]runRecord, 0, len(jobs))
	for _, job := range jobs {
		rec, err := r.runOne(ctx, nowMinute, job)
		if err != nil {
			return err
		}
		records = append(records, rec)
	}
	return appendRunRecords(filepath.Join(r.LogDir, "cron", "runs.jsonl"), records)
}

func (r *Runner) dueJobs(nowMinute time.Time) ([]db.CronJob, error) {
	all, err := r.Store.ActiveCrons()
	if err != nil {
		return nil, fmt.Errorf("query crons: %w", err)
	}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	var due []db.CronJob
	for _, c := range all {
		loc, err := time.LoadLocation(c.Timezone)
		if err != nil {
			loc = time.Local
		}
		s, err := parser.Parse(c.Schedule)
		if err != nil {
			continue
		}
		nowInLoc := nowMinute.In(loc)
		prev := nowInLoc.Add(-time.Minute)
		next := s.Next(prev)
		if next.Equal(nowInLoc) {
			due = append(due, c)
		}
	}
	return due, nil
}

func (r *Runner) runOne(ctx context.Context, nowMinute time.Time, job db.CronJob) (runRecord, error) {
	runMinute := nowMinute.Format(time.RFC3339)

	if err := r.Store.RecordCronRun(job.ID, runMinute, "started", "", ""); err != nil {
		return runRecord{}, fmt.Errorf("insert cron run: %w", err)
	}

	jobLog := filepath.Join(r.LogDir, "cron", "jobs", fmt.Sprintf("%s-cron-%d.log", nowMinute.Format("20060102-1504"), job.ID))
	prompt := buildCronPrompt(job.ChatID, job.Prompt)
	cmd := exec.CommandContext(ctx, "claude", "--dangerously-skip-permissions", "-p", prompt)
	cmd.Dir = r.WorkspaceDir
	output, err := cmd.CombinedOutput()
	if writeErr := os.WriteFile(jobLog, output, 0o644); writeErr != nil {
		return runRecord{}, fmt.Errorf("write job log: %w", writeErr)
	}
	status := "ok"
	if err != nil {
		status = "error"
	}

	if err := r.Store.RecordCronRun(job.ID, runMinute, status, "", jobLog); err != nil {
		return runRecord{}, fmt.Errorf("update cron run: %w", err)
	}

	// On error, notify via Telegram so the user knows something went wrong
	if status == "error" && r.TelegramNotifier != nil {
		errNotify := fmt.Sprintf("Cron job #%d failed. Check log: %s", job.ID, jobLog)
		_ = r.TelegramNotifier.SendMessage(ctx, job.ChatID, errNotify)
	}

	return runRecord{
		Minute:     runMinute,
		CronID:     job.ID,
		ChatID:     job.ChatID,
		Schedule:   job.Schedule,
		Status:     status,
		JobLogPath: jobLog,
	}, nil
}

func appendRunRecords(path string, records []runRecord) error {
	if len(records) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir runs jsonl dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open runs jsonl: %w", err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, rec := range records {
		if err := enc.Encode(rec); err != nil {
			return fmt.Errorf("write runs jsonl: %w", err)
		}
	}
	return nil
}

func buildCronPrompt(chatID, userPrompt string) string {
	return fmt.Sprintf(`Read CRON.md before executing.

Execute this user cron prompt:
%s

Send your response to the user by piping markdown into:
  ./goat send_user_message --chat %s

See GOATED_CLI_README.md for formatting details.`, strings.TrimSpace(userPrompt), chatID)
}
