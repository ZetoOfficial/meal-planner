package feedback

import "context"

type CreateFeedback struct {
	repo FeedbackRepository
}

func NewCreateFeedback(repo FeedbackRepository) *CreateFeedback { return &CreateFeedback{repo: repo} }

func (uc *CreateFeedback) Handle(_ context.Context, _ any) error { return nil }
