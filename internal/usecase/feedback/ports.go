// Package feedback (usecase) — создание фидбека (вес, не бан).
package feedback

import "context"

// FeedbackRepository — порт сохранения событий фидбека.
type FeedbackRepository interface {
	Save(ctx context.Context, payload any) error
}
