package productimage

import (
	"context"
	"time"
)

const taskPersistenceTimeout = 5 * time.Second

func persistenceContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), taskPersistenceTimeout)
}
