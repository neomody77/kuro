package pipeline

import (
	"context"
	"time"
)

// CronTriggerHandler implements NodeHandler for n8n-nodes-base.cron / n8n-nodes-base.scheduleTrigger.
// As a trigger node, it simply emits a single item with the current timestamp.
// The actual scheduling (cron timing) is handled by the Scheduler, not this handler.
type CronTriggerHandler struct{}

func (h *CronTriggerHandler) ExecuteNode(_ context.Context, node *Node, _ []NodeItem, _ map[string]map[string]string) (*NodeOutput, error) {
	now := time.Now()
	item := NodeItem{
		"timestamp":  now.Format(time.RFC3339),
		"unixTime":   now.Unix(),
		"triggerNode": node.Name,
	}
	return &NodeOutput{
		Items: [][]NodeItem{{item}},
	}, nil
}
