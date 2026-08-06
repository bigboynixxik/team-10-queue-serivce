// Package worker provides background processes and cron jobs.
package worker

import (
	"context"
)

// ExpirationService defines the business logic contract strictly required by the background worker.
type ExpirationService interface {
	// ProcessExpirations scans for and handles users who did not react to their offers
	// or complete their purchases within the allowed timeframes.
	ProcessExpirations(ctx context.Context) error
}
