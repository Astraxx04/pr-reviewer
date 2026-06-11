package jobs

import (
	"context"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// JobEnqueuer abstracts river.Client so handlers can enqueue jobs without depending on the river concrete type.
// Satisfied by *river.Client[pgx.Tx].
type JobEnqueuer interface {
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}
