package queue

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/agextract/agextract-cli/internal/config"
	bolt "go.etcd.io/bbolt"
)

const (
	bucketName  = "retries"
	dbFileName  = "retry.db"
	maxAttempts = 10
)

// Backoff schedule: 5m, 15m, 1h, 6h, 24h (repeats 24h for remaining)
var backoffSchedule = []time.Duration{
	5 * time.Minute,
	15 * time.Minute,
	1 * time.Hour,
	6 * time.Hour,
	24 * time.Hour,
}

type RetryItem struct {
	FilePath  string    `json:"file_path"`
	Tool      string    `json:"tool"`
	Attempts  int       `json:"attempts"`
	NextRetry time.Time `json:"next_retry"`
	CreatedAt time.Time `json:"created_at"`
}

type RetryQueue struct {
	db *bolt.DB
}

func Open() (*RetryQueue, error) {
	dbPath := filepath.Join(config.Dir(), dbFileName)
	if err := config.EnsureDir(); err != nil {
		return nil, err
	}

	db, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("opening retry db: %w", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		return err
	})
	if err != nil {
		db.Close()
		return nil, err
	}

	return &RetryQueue{db: db}, nil
}

func (q *RetryQueue) Close() error {
	return q.db.Close()
}

func (q *RetryQueue) Add(filePath, tool string) error {
	item := RetryItem{
		FilePath:  filePath,
		Tool:      tool,
		Attempts:  0,
		NextRetry: time.Now().Add(backoffSchedule[0]),
		CreatedAt: time.Now(),
	}

	return q.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		data, err := json.Marshal(item)
		if err != nil {
			return err
		}
		return b.Put([]byte(filePath), data)
	})
}

func (q *RetryQueue) Count() int {
	count := 0
	q.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		count = b.Stats().KeyN
		return nil
	})
	return count
}

// ProcessLoop continuously processes retryable items.
func (q *RetryQueue) ProcessLoop(uploadFn func(RetryItem) error) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		q.processOnce(uploadFn)
	}
}

func (q *RetryQueue) processOnce(uploadFn func(RetryItem) error) {
	var readyItems []RetryItem

	q.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		return b.ForEach(func(k, v []byte) error {
			var item RetryItem
			if err := json.Unmarshal(v, &item); err != nil {
				return nil
			}
			if time.Now().After(item.NextRetry) {
				readyItems = append(readyItems, item)
			}
			return nil
		})
	})

	for _, item := range readyItems {
		if err := uploadFn(item); err != nil {
			// Update attempt count and next retry
			item.Attempts++
			if item.Attempts >= maxAttempts {
				// Give up — remove from queue
				q.remove(item.FilePath)
				fmt.Printf("Giving up on %s after %d attempts\n", item.FilePath, maxAttempts)
				continue
			}

			backoffIdx := item.Attempts
			if backoffIdx >= len(backoffSchedule) {
				backoffIdx = len(backoffSchedule) - 1
			}
			item.NextRetry = time.Now().Add(backoffSchedule[backoffIdx])

			q.db.Update(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte(bucketName))
				data, _ := json.Marshal(item)
				return b.Put([]byte(item.FilePath), data)
			})
		} else {
			q.remove(item.FilePath)
		}
	}
}

func (q *RetryQueue) remove(filePath string) {
	q.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		return b.Delete([]byte(filePath))
	})
}
