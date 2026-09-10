package longcontext

import (
	"encoding/json"
	"time"
)

type Status string

type Phase string

const (
	StatusQueued       Status = "queued"
	StatusIngesting    Status = "ingesting"
	StatusMapping      Status = "mapping"
	StatusIndexing     Status = "indexing"
	StatusRetrieving   Status = "retrieving"
	StatusSynthesizing Status = "synthesizing"
	StatusVerifying    Status = "verifying"
	StatusSucceeded    Status = "succeeded"
	StatusFailed       Status = "failed"
	StatusCanceled     Status = "canceled"
)

const (
	PhaseQueued       Phase = "queued"
	PhaseIngesting    Phase = "ingesting"
	PhaseMapping      Phase = "mapping"
	PhaseIndexing     Phase = "indexing"
	PhaseRetrieving   Phase = "retrieving"
	PhaseSynthesizing Phase = "synthesizing"
	PhaseVerifying    Phase = "verifying"
)

type Task struct {
	ID                 string
	TenantID           string
	UserID             string
	SessionID          string
	IdempotencyKey     string
	Model              string
	Mode               string
	Query              string
	Status             Status
	Phase              Phase
	InputSHA256        string
	InputTokens        int64
	ChunkCount         int
	CompletedChunks    int
	WorkerCount        int
	RetrievalTopK      int
	FinalContextBudget int
	MaxCost            *float64
	UsedTokens         int64
	EstimatedCost      float64
	RetryCount         int
	LastError          string
	Result             json.RawMessage
	LeaseOwner         string
	LeaseUntil         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
	StartedAt          *time.Time
	FinishedAt         *time.Time
}

type CreateTaskInput struct {
	ID             string
	TenantID       string
	UserID         string
	SessionID      string
	IdempotencyKey string
	Model          string
	Mode           string
	Query          string
	InputSHA256    string
	InputTokens    int64
	WorkerCount    int
	RetrievalTopK  int
	FinalBudget    int
	MaxCost        *float64
	LeaseOwner     string        // optional: set lease atomically on create
	LeaseDuration  time.Duration // optional: lease TTL (requires LeaseOwner)
}

type Chunk struct {
	ID            string
	TaskID        string
	Ordinal       int
	StartChar     int64
	EndChar       int64
	StartToken    int64
	EndToken      int64
	Content       string
	ContentHash   string
	Status        string
	MapAttempts   int
	MapResult     json.RawMessage
	MapModel      string
	MapProvider   string
	MapLeaseUntil time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Evidence struct {
	ID                 int64
	TaskID             string
	ChunkID            string
	Claim              string
	Quote              string
	StartChar          int64
	EndChar            int64
	Entities           json.RawMessage
	Relations          json.RawMessage
	Confidence         float64
	VerificationStatus string
	Embedding          []float64
	CreatedAt          time.Time
}

func (s Status) Terminal() bool {
	return s == StatusSucceeded || s == StatusFailed || s == StatusCanceled
}

func ValidTransition(from, to Status) bool {
	if from.Terminal() {
		return false
	}
	if to == StatusCanceled {
		return true
	}
	allowed := map[Status]Status{
		StatusQueued:       StatusIngesting,
		StatusIngesting:    StatusMapping,
		StatusMapping:      StatusIndexing,
		StatusIndexing:     StatusRetrieving,
		StatusRetrieving:   StatusSynthesizing,
		StatusSynthesizing: StatusVerifying,
		StatusVerifying:    StatusSucceeded,
	}
	return allowed[from] == to
}
