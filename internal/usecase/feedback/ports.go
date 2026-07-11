package feedback

import "context"

type FeedbackRepository interface {
	Save(ctx context.Context, payload any) error
}
