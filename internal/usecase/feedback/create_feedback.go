package feedback

import "context"

// CreateFeedback — use case: оценка/комментарий → сдвиг веса (FR-7). TODO.
type CreateFeedback struct {
	repo FeedbackRepository
}

// NewCreateFeedback собирает use case.
func NewCreateFeedback(repo FeedbackRepository) *CreateFeedback { return &CreateFeedback{repo: repo} }

// Handle — TODO.
func (uc *CreateFeedback) Handle(_ context.Context, _ any) error { return nil }
