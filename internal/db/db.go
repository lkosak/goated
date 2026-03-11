package db

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

var (
	cronsBucket    = []byte("crons")
	cronRunsBucket = []byte("cron_runs")
)

type Store struct {
	db *bolt.DB
}

type CronJob struct {
	ID        uint64 `json:"id"`
	ChatID    string `json:"chat_id"`
	Schedule  string `json:"schedule"`
	Prompt    string `json:"prompt"`
	Timezone  string `json:"timezone"`
	Active    bool   `json:"active"`
	CreatedAt string `json:"created_at"`
}

type CronRun struct {
	ID         uint64 `json:"id"`
	CronID     uint64 `json:"cron_id"`
	RunMinute  string `json:"run_minute"`
	Status     string `json:"status"`
	UserMsg    string `json:"user_message,omitempty"`
	JobLogPath string `json:"job_log_path,omitempty"`
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir db dir: %w", err)
	}
	bdb, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open bolt: %w", err)
	}
	err = bdb.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(cronsBucket); err != nil {
			return err
		}
		_, err := tx.CreateBucketIfNotExists(cronRunsBucket)
		return err
	})
	if err != nil {
		_ = bdb.Close()
		return nil, fmt.Errorf("init buckets: %w", err)
	}
	return &Store{db: bdb}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) AddCron(chatID, schedule, prompt, timezone string) (uint64, error) {
	var id uint64
	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(cronsBucket)
		seq, _ := b.NextSequence()
		id = seq
		job := CronJob{
			ID:        id,
			ChatID:    chatID,
			Schedule:  schedule,
			Prompt:    prompt,
			Timezone:  timezone,
			Active:    true,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		data, err := json.Marshal(job)
		if err != nil {
			return err
		}
		return b.Put(itob(id), data)
	})
	return id, err
}

func (s *Store) ActiveCrons() ([]CronJob, error) {
	var jobs []CronJob
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(cronsBucket)
		return b.ForEach(func(k, v []byte) error {
			var job CronJob
			if err := json.Unmarshal(v, &job); err != nil {
				return nil // skip corrupt entries
			}
			if job.Active {
				jobs = append(jobs, job)
			}
			return nil
		})
	})
	return jobs, err
}

func (s *Store) RecordCronRun(cronID uint64, runMinute, status, userMsg, jobLogPath string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(cronRunsBucket)
		// Dedup: check if this cron+minute already recorded
		dedupKey := fmt.Sprintf("%d:%s", cronID, runMinute)
		if existing := b.Get([]byte(dedupKey)); existing != nil {
			// Update existing
			var run CronRun
			if err := json.Unmarshal(existing, &run); err == nil {
				run.Status = status
				run.UserMsg = userMsg
				run.JobLogPath = jobLogPath
				data, err := json.Marshal(run)
				if err != nil {
					return err
				}
				return b.Put([]byte(dedupKey), data)
			}
		}
		seq, _ := b.NextSequence()
		run := CronRun{
			ID:         seq,
			CronID:     cronID,
			RunMinute:  runMinute,
			Status:     status,
			UserMsg:    userMsg,
			JobLogPath: jobLogPath,
		}
		data, err := json.Marshal(run)
		if err != nil {
			return err
		}
		return b.Put([]byte(dedupKey), data)
	})
}

func (s *Store) AllCrons() ([]CronJob, error) {
	var jobs []CronJob
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(cronsBucket)
		return b.ForEach(func(k, v []byte) error {
			var job CronJob
			if err := json.Unmarshal(v, &job); err != nil {
				return nil
			}
			jobs = append(jobs, job)
			return nil
		})
	})
	return jobs, err
}

func (s *Store) GetCron(id uint64) (*CronJob, error) {
	var job CronJob
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(cronsBucket)
		data := b.Get(itob(id))
		if data == nil {
			return fmt.Errorf("cron %d not found", id)
		}
		return json.Unmarshal(data, &job)
	})
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (s *Store) SetCronActive(id uint64, active bool) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(cronsBucket)
		key := itob(id)
		data := b.Get(key)
		if data == nil {
			return fmt.Errorf("cron %d not found", id)
		}
		var job CronJob
		if err := json.Unmarshal(data, &job); err != nil {
			return err
		}
		job.Active = active
		updated, err := json.Marshal(job)
		if err != nil {
			return err
		}
		return b.Put(key, updated)
	})
}

func (s *Store) DeleteCron(id uint64) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(cronsBucket)
		key := itob(id)
		if b.Get(key) == nil {
			return fmt.Errorf("cron %d not found", id)
		}
		return b.Delete(key)
	})
}

func itob(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	return b
}
