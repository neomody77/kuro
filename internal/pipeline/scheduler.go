package pipeline

import (
	"context"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Scheduler manages cron-triggered and manually-triggered pipeline executions.
type Scheduler struct {
	mu            sync.Mutex
	executor      *Executor
	workflowStore *WorkflowStore
	entries       map[string]*schedEntry
	manual        chan *Pipeline
	ctx           context.Context
	cancel        context.CancelFunc
}

type schedEntry struct {
	pipeline *Pipeline
	cancel   context.CancelFunc
}

// NewScheduler creates a scheduler that uses the given executor.
func NewScheduler(executor *Executor, store *WorkflowStore) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		executor:      executor,
		workflowStore: store,
		entries:       make(map[string]*schedEntry),
		manual:        make(chan *Pipeline, 16),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start begins the scheduler loop that listens for manual trigger requests.
func (s *Scheduler) Start() {
	go func() {
		for {
			select {
			case <-s.ctx.Done():
				return
			case p := <-s.manual:
				go func(pip *Pipeline) {
					_, err := s.executor.Execute(s.ctx, pip)
					if err != nil {
						log.Printf("manual run of %q failed: %v", pip.Name, err)
					}
				}(p)
			}
		}
	}()
}

// Stop cancels all scheduled pipelines and stops the scheduler.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, entry := range s.entries {
		entry.cancel()
	}
	s.cancel()
}

// Register schedules a workflow for execution. It detects trigger nodes
// and starts appropriate polling loops.
func (s *Scheduler) Register(p *Pipeline) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Unregister existing entry if present.
	if existing, ok := s.entries[p.ID]; ok {
		existing.cancel()
	}

	// Check for trigger nodes (IMAP, cron, webhook, etc.)
	for _, node := range p.Nodes {
		if !isTriggerNode(&node) {
			continue
		}
		ctx, cancel := context.WithCancel(s.ctx)
		s.entries[p.ID] = &schedEntry{pipeline: p, cancel: cancel}

		if node.Type == "n8n-nodes-base.emailReadImap" {
			// Use IMAP IDLE for near-instant notification
			go s.runImapIdleLoop(ctx, p, &node)
			log.Printf("[scheduler] registered IMAP IDLE workflow %q (%s)", p.Name, p.ID)
		} else {
			interval := getTriggerInterval(&node)
			go s.runTriggerLoop(ctx, p, interval)
			log.Printf("[scheduler] registered trigger workflow %q (%s) interval=%v", p.Name, p.ID, interval)
		}
		return
	}

	// Legacy cron scheduling
	if p.Settings.ExecutionTimeout <= 0 {
		return
	}

	ctx, cancel := context.WithCancel(s.ctx)
	s.entries[p.ID] = &schedEntry{pipeline: p, cancel: cancel}
	go s.runCronLoop(ctx, p)
}

// RegisterActiveWorkflows loads all active workflows and registers them.
func (s *Scheduler) RegisterActiveWorkflows() {
	if s.workflowStore == nil {
		return
	}
	workflows, err := s.workflowStore.List()
	if err != nil {
		log.Printf("[scheduler] failed to list workflows: %v", err)
		return
	}
	for _, w := range workflows {
		if w.Active {
			s.Register(w)
		}
	}
}

// runImapIdleLoop uses IMAP IDLE to wait for new mail, then executes the workflow.
func (s *Scheduler) runImapIdleLoop(ctx context.Context, p *Pipeline, triggerNode *Node) {
	// Resolve IMAP credentials
	creds := s.executor.resolveNodeCredentials(triggerNode)
	var imapCreds map[string]string
	if c, ok := creds["imap"]; ok {
		imapCreds = c
	}
	if imapCreds == nil {
		for _, c := range creds {
			if c["imap_host"] != "" {
				imapCreds = c
				break
			}
		}
	}
	if imapCreds == nil {
		log.Printf("[scheduler] IMAP IDLE: no credentials found for %q, falling back to polling", p.Name)
		s.runTriggerLoop(ctx, p, 60*time.Second)
		return
	}

	host := imapCreds["imap_host"]
	user := imapCreds["imap_user"]
	pass := imapCreds["imap_pass"]
	if host == "" || user == "" || pass == "" {
		log.Printf("[scheduler] IMAP IDLE: incomplete credentials for %q, falling back to polling", p.Name)
		s.runTriggerLoop(ctx, p, 60*time.Second)
		return
	}

	// Do an initial run to catch up on any missed emails
	log.Printf("[scheduler] IMAP IDLE: initial run for %q", p.Name)
	s.executeAndSave(ctx, p)

	// Start IDLE watcher
	watcher := NewImapIdleWatcher(host, user, pass, func() {
		// Small delay to let the email fully arrive
		time.Sleep(2 * time.Second)
		log.Printf("[scheduler] IMAP IDLE: new mail for %q, executing workflow", p.Name)
		s.executeAndSave(ctx, p)
	})
	watcher.Run(ctx)
}

// executeAndSave runs a workflow and saves staticData updates.
func (s *Scheduler) executeAndSave(ctx context.Context, p *Pipeline) {
	exec, err := s.executor.Execute(ctx, p)
	if err != nil {
		log.Printf("[scheduler] workflow %q failed: %v", p.Name, err)
		return
	}
	log.Printf("[scheduler] workflow %q completed: %s", p.Name, exec.Status)
	if s.workflowStore != nil {
		if err := s.workflowStore.Save(p); err != nil {
			log.Printf("[scheduler] failed to save workflow staticData: %v", err)
		}
	}
}

// runTriggerLoop polls a trigger-based workflow at regular intervals.
func (s *Scheduler) runTriggerLoop(ctx context.Context, p *Pipeline, interval time.Duration) {
	// Initial delay to let the system start up
	timer := time.NewTimer(5 * time.Second)
	select {
	case <-ctx.Done():
		timer.Stop()
		return
	case <-timer.C:
	}

	for {
		log.Printf("[scheduler] executing trigger workflow %q", p.Name)
		exec, err := s.executor.Execute(ctx, p)
		if err != nil {
			log.Printf("[scheduler] trigger workflow %q failed: %v", p.Name, err)
		} else {
			log.Printf("[scheduler] trigger workflow %q completed: %s", p.Name, exec.Status)
			// Save updated staticData back to disk
			if s.workflowStore != nil {
				if err := s.workflowStore.Save(p); err != nil {
					log.Printf("[scheduler] failed to save workflow staticData: %v", err)
				}
			}
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// getTriggerInterval returns the poll interval for a trigger node.
// For cron/scheduleTrigger nodes, it parses the n8n rule to determine interval.
// Falls back to 60s.
func getTriggerInterval(node *Node) time.Duration {
	switch node.Type {
	case "n8n-nodes-base.cron", "n8n-nodes-base.scheduleTrigger":
		return parseCronInterval(node.Parameters)
	case "n8n-nodes-base.emailReadImap":
		return 60 * time.Second
	default:
		return 60 * time.Second
	}
}

// parseCronInterval extracts the interval from n8n schedule trigger parameters.
// n8n scheduleTrigger uses: {"rule": {"interval": [{"field": "minutes", "minutesInterval": 5}]}}
// or {"rule": {"interval": [{"field": "hours", "hoursInterval": 1}]}}
// Also supports simple cron: {"triggerTimes": {"item": [{"mode": "everyMinute"}]}}
func parseCronInterval(params map[string]any) time.Duration {
	// Try n8n scheduleTrigger format
	if rule, ok := params["rule"].(map[string]any); ok {
		if intervals, ok := rule["interval"].([]any); ok && len(intervals) > 0 {
			if entry, ok := intervals[0].(map[string]any); ok {
				field, _ := entry["field"].(string)
				switch field {
				case "seconds":
					if v, ok := numVal(entry, "secondsInterval"); ok && v > 0 {
						return time.Duration(v) * time.Second
					}
				case "minutes":
					if v, ok := numVal(entry, "minutesInterval"); ok && v > 0 {
						return time.Duration(v) * time.Minute
					}
				case "hours":
					if v, ok := numVal(entry, "hoursInterval"); ok && v > 0 {
						return time.Duration(v) * time.Hour
					}
				}
			}
		}
	}

	// Try simple cron trigger
	if tt, ok := params["triggerTimes"].(map[string]any); ok {
		if items, ok := tt["item"].([]any); ok && len(items) > 0 {
			if item, ok := items[0].(map[string]any); ok {
				mode, _ := item["mode"].(string)
				switch mode {
				case "everyMinute":
					return time.Minute
				case "everyHour":
					return time.Hour
				}
			}
		}
	}

	return 60 * time.Second
}

func numVal(m map[string]any, key string) (int, bool) {
	switch v := m[key].(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	default:
		return 0, false
	}
}

// Unregister removes a workflow from the scheduler.
func (s *Scheduler) Unregister(workflowID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entry, ok := s.entries[workflowID]; ok {
		entry.cancel()
		delete(s.entries, workflowID)
	}
}

// TriggerManual enqueues a pipeline for immediate execution.
func (s *Scheduler) TriggerManual(p *Pipeline) {
	s.manual <- p
}

// runCronLoop is a goroutine that fires the workflow on a cron schedule.
// TODO: Extract cron expression from workflow trigger node or settings.
func (s *Scheduler) runCronLoop(ctx context.Context, p *Pipeline) {
	for {
		// For now, use ExecutionTimeout as a simple interval in seconds.
		next := time.Now().Add(time.Duration(p.Settings.ExecutionTimeout) * time.Second)
		delay := time.Until(next)
		if delay < 0 {
			delay = time.Minute
		}

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			_, err := s.executor.Execute(ctx, p)
			if err != nil {
				log.Printf("cron run of %q failed: %v", p.Name, err)
			}
		}
	}
}

// nextCronTime computes the next time a cron expression matches after 'after'.
// Simple cron format: "minute hour dom month dow"
func nextCronTime(expr string, after time.Time) time.Time {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return after.Add(time.Hour)
	}

	minute := parseCronField(fields[0], 0, 59)
	hour := parseCronField(fields[1], 0, 23)
	dom := parseCronField(fields[2], 1, 31)
	month := parseCronField(fields[3], 1, 12)
	dow := parseCronField(fields[4], 0, 6) // 0=Sunday

	// Scan forward up to 366 days.
	candidate := after.Truncate(time.Minute).Add(time.Minute)
	for i := 0; i < 366*24*60; i++ {
		if matchesCron(candidate, minute, hour, dom, month, dow) {
			return candidate
		}
		candidate = candidate.Add(time.Minute)
	}

	return after.Add(time.Hour)
}

func matchesCron(t time.Time, minute, hour, dom, month, dow []int) bool {
	if !intIn(t.Minute(), minute) {
		return false
	}
	if !intIn(t.Hour(), hour) {
		return false
	}
	if !intIn(t.Day(), dom) {
		return false
	}
	if !intIn(int(t.Month()), month) {
		return false
	}
	if !intIn(int(t.Weekday()), dow) {
		return false
	}
	return true
}

func intIn(val int, set []int) bool {
	if set == nil { // wildcard
		return true
	}
	for _, v := range set {
		if v == val {
			return true
		}
	}
	return false
}

// parseCronField parses a single cron field into a list of matching integers.
// Supports: *, single number, comma-separated, and ranges with -.
// Returns nil for wildcard (*).
func parseCronField(field string, min, max int) []int {
	if field == "*" {
		return nil
	}

	var result []int
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			lo, err1 := strconv.Atoi(bounds[0])
			hi, err2 := strconv.Atoi(bounds[1])
			if err1 != nil || err2 != nil {
				continue
			}
			if lo < min {
				lo = min
			}
			if hi > max {
				hi = max
			}
			for i := lo; i <= hi; i++ {
				result = append(result, i)
			}
		} else {
			v, err := strconv.Atoi(part)
			if err != nil {
				continue
			}
			if v >= min && v <= max {
				result = append(result, v)
			}
		}
	}
	return result
}
