package pipeline

import (
	"context"
	"time"
)

// ActionHandler is the interface that action implementations must satisfy.
type ActionHandler interface {
	Execute(ctx context.Context, params map[string]any, creds map[string]string) (any, error)
}

// NodeItem represents a single data item flowing between nodes (n8n's $json).
type NodeItem map[string]any

// NodeOutput represents the output of a node, organized by output index.
// Index 0 is main/true output, index 1 is false output (for If nodes).
type NodeOutput struct {
	Items [][]NodeItem // Items[outputIndex] = array of items for that output
}

// NodeHandler is the enhanced interface for n8n-compatible node execution.
// It receives input items and returns output items organized by output index.
type NodeHandler interface {
	ExecuteNode(ctx context.Context, node *Node, input []NodeItem, creds map[string]map[string]string) (*NodeOutput, error)
}

// CredentialResolver resolves credential references to decrypted data by ID.
type CredentialResolver interface {
	Resolve(id string) (map[string]string, error)
}

// Workflow is an n8n-compatible workflow definition.
type Workflow struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Active     bool              `json:"active"`
	Nodes      []Node            `json:"nodes"`
	Connections map[string]NodeConnection `json:"connections"`
	Settings   WorkflowSettings  `json:"settings"`
	StaticData any               `json:"staticData,omitempty"`
	Tags       []Tag             `json:"tags,omitempty"`
	CreatedAt  time.Time         `json:"createdAt"`
	UpdatedAt  time.Time         `json:"updatedAt"`
}

// Node is an n8n-compatible workflow node.
type Node struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Type            string         `json:"type"`
	TypeVersion     float64        `json:"typeVersion"`
	Position        [2]float64     `json:"position"`
	Parameters      map[string]any `json:"parameters,omitempty"`
	Credentials     map[string]NodeCredential `json:"credentials,omitempty"`
	Disabled        bool           `json:"disabled,omitempty"`
	Notes           string         `json:"notes,omitempty"`
	OnError         string         `json:"onError,omitempty"`
	ExecuteOnce     bool           `json:"executeOnce,omitempty"`
	RetryOnFail     bool           `json:"retryOnFail,omitempty"`
	MaxTries        int            `json:"maxTries,omitempty"`
	WaitBetweenTries int           `json:"waitBetweenTries,omitempty"`
	AlwaysOutputData bool          `json:"alwaysOutputData,omitempty"`
	WebhookID       string         `json:"webhookId,omitempty"`
}

// NodeCredential references a credential used by a node.
type NodeCredential struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// NodeConnection describes the connections from a node's outputs.
// Key is output type (usually "main"), value is array of output indices,
// each containing an array of connection targets.
type NodeConnection struct {
	Main [][]ConnectionTarget `json:"main,omitempty"`
}

// ConnectionTarget identifies a specific input of another node.
type ConnectionTarget struct {
	Node  string `json:"node"`
	Type  string `json:"type"`
	Index int    `json:"index"`
}

// WorkflowSettings holds n8n-compatible workflow settings.
type WorkflowSettings struct {
	SaveExecutionProgress    *bool  `json:"saveExecutionProgress,omitempty"`
	SaveManualExecutions     *bool  `json:"saveManualExecutions,omitempty"`
	SaveDataErrorExecution   string `json:"saveDataErrorExecution,omitempty"`
	SaveDataSuccessExecution string `json:"saveDataSuccessExecution,omitempty"`
	ExecutionTimeout         int    `json:"executionTimeout,omitempty"`
	ErrorWorkflow            string `json:"errorWorkflow,omitempty"`
	Timezone                 string `json:"timezone,omitempty"`
	ExecutionOrder           string `json:"executionOrder,omitempty"`
	CallerPolicy             string `json:"callerPolicy,omitempty"`
}

// Tag represents a workflow tag.
type Tag struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// --- Execution types (n8n-compatible) ---

// ExecutionStatus represents the status of a workflow execution.
type ExecutionStatus string

const (
	ExecCanceled ExecutionStatus = "canceled"
	ExecCrashed  ExecutionStatus = "crashed"
	ExecError    ExecutionStatus = "error"
	ExecNew      ExecutionStatus = "new"
	ExecRunning  ExecutionStatus = "running"
	ExecSuccess  ExecutionStatus = "success"
	ExecUnknown  ExecutionStatus = "unknown"
	ExecWaiting  ExecutionStatus = "waiting"
)

// ExecutionMode describes how an execution was triggered.
type ExecutionMode string

const (
	ModeManual  ExecutionMode = "manual"
	ModeTrigger ExecutionMode = "trigger"
	ModeWebhook ExecutionMode = "webhook"
	ModeRetry   ExecutionMode = "retry"
	ModeCLI     ExecutionMode = "cli"
	ModeChat    ExecutionMode = "chat"
)

// Execution represents the state of a workflow execution.
type Execution struct {
	ID         string            `json:"id"`
	WorkflowID string           `json:"workflowId"`
	Status     ExecutionStatus   `json:"status"`
	Mode       ExecutionMode     `json:"mode"`
	StartedAt  time.Time         `json:"startedAt"`
	StoppedAt  *time.Time        `json:"stoppedAt,omitempty"`
	Finished   bool              `json:"finished"`
	Data       *ExecutionData    `json:"data,omitempty"`
	CustomData map[string]any    `json:"customData,omitempty"`
	WaitTill   *time.Time        `json:"waitTill,omitempty"`
}

// ExecutionData holds the runtime data of an execution.
type ExecutionData struct {
	ResultData    ResultData               `json:"resultData"`
	ExecutionData map[string]any           `json:"executionData,omitempty"`
}

// ResultData contains per-node execution results.
type ResultData struct {
	RunData    map[string][]NodeRunData `json:"runData,omitempty"`
	LastNodeExecuted string             `json:"lastNodeExecuted,omitempty"`
	Error      *ExecutionError          `json:"error,omitempty"`
}

// NodeRunData is the execution result of a single node run.
type NodeRunData struct {
	StartTime   int64          `json:"startTime"`
	ExecutionTime int64        `json:"executionTime"`
	Data        map[string]any `json:"data,omitempty"`
	Error       *ExecutionError `json:"error,omitempty"`
}

// ExecutionError describes an error during execution.
type ExecutionError struct {
	Message string `json:"message"`
	Node    string `json:"node,omitempty"`
}

// --- Legacy compatibility aliases ---
// These allow existing code to compile while migrating.

// Pipeline is an alias for Workflow for backward compatibility.
type Pipeline = Workflow

// RunState is an alias for Execution for backward compatibility.
type RunState = Execution

// RunStatus maps to ExecutionStatus for backward compatibility.
type RunStatus = ExecutionStatus

// NodeStatus represents the status of a single node execution.
type NodeStatus string

const (
	NodePending   NodeStatus = "pending"
	NodeRunning   NodeStatus = "running"
	NodeCompleted NodeStatus = "completed"
	NodeFailed    NodeStatus = "failed"
	NodeSkipped   NodeStatus = "skipped"
)

// NodeResult records the result of executing a single node (legacy).
type NodeResult struct {
	NodeID   string        `json:"node_id"`
	Status   NodeStatus    `json:"status"`
	Output   any           `json:"output,omitempty"`
	Error    string        `json:"error,omitempty"`
	Duration time.Duration `json:"duration"`
}

// Trigger defines how a pipeline is initiated (legacy, mapped to workflow settings).
type Trigger struct {
	Type string `json:"type"` // cron, webhook, manual
	Expr string `json:"expr,omitempty"`
	Path string `json:"path,omitempty"`
}

// --- Variable ---

// Variable is an n8n-compatible key-value variable.
type Variable struct {
	ID    string `json:"id"`
	Key   string `json:"key"`
	Value string `json:"value"`
	Type  string `json:"type,omitempty"`
}

// --- DataTable ---

// DataTable is an n8n-compatible structured data table.
type DataTable struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Columns   []DataColumn `json:"columns"`
	CreatedAt time.Time    `json:"createdAt"`
	UpdatedAt time.Time    `json:"updatedAt"`
}

// DataColumn defines a column in a data table.
type DataColumn struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"` // string, number, boolean, date
	Index int    `json:"index"`
}

// DataTableRow is a row in a data table.
type DataTableRow struct {
	ID        int            `json:"id"`
	Data      map[string]any `json:"data"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}
