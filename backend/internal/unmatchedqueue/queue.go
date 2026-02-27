package unmatchedqueue

import (
	"sync"
	"time"
)

const defaultMaxSize = 5000

// UnmatchedItem 无匹配规则的计费日志聚合项
type UnmatchedItem struct {
	Tag           string    `json:"tag"`
	RuleName      string    `json:"rule_name"`
	LogLineSample string    `json:"log_line_sample"`
	Count         int64     `json:"count"`
	LastSeen      time.Time `json:"last_seen"`
}

// Queue 有界内存队列，按 tag+rule_name 聚合
type Queue struct {
	mu      sync.RWMutex
	items   map[string]*UnmatchedItem
	order   []string
	maxSize int
}

// New 创建队列，maxSize 为 0 时使用默认 5000
func New(maxSize int) *Queue {
	if maxSize <= 0 {
		maxSize = defaultMaxSize
	}
	return &Queue{
		items:   make(map[string]*UnmatchedItem),
		order:   make([]string, 0, maxSize),
		maxSize: maxSize,
	}
}

func key(tag, ruleName string) string {
	return tag + "\x00" + ruleName
}

// Add 添加无匹配记录，按 tag+rule_name 聚合
func (q *Queue) Add(tag, ruleName, logLine string) {
	if tag == "" && ruleName == "" {
		return
	}
	k := key(tag, ruleName)
	sample := logLine
	if len(sample) > 500 {
		sample = sample[:500] + "..."
	}
	now := time.Now()

	q.mu.Lock()
	defer q.mu.Unlock()

	if it, ok := q.items[k]; ok {
		it.Count++
		it.LogLineSample = sample
		it.LastSeen = now
		return
	}

	if len(q.order) >= q.maxSize {
		oldest := q.order[0]
		q.order = q.order[1:]
		delete(q.items, oldest)
	}

	it := &UnmatchedItem{
		Tag:           tag,
		RuleName:      ruleName,
		LogLineSample: sample,
		Count:         1,
		LastSeen:      now,
	}
	q.items[k] = it
	q.order = append(q.order, k)
}

// Snapshot 返回当前所有项的快照（最近 limit 条，0 表示全部）
func (q *Queue) Snapshot(limit int) []UnmatchedItem {
	q.mu.RLock()
	defer q.mu.RUnlock()

	n := len(q.order)
	if limit > 0 && n > limit {
		n = limit
	}
	result := make([]UnmatchedItem, 0, n)
	for i := len(q.order) - 1; i >= 0 && len(result) < n; i-- {
		k := q.order[i]
		if it, ok := q.items[k]; ok {
			result = append(result, *it)
		}
	}
	return result
}
