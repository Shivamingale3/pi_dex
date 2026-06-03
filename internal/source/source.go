package source

import (
	"context"
)

type Source interface {
	Run(ctx context.Context) error
}
