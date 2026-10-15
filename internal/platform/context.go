package platform

import (
	"context"
	"net/http"
	"time"
)

func timeBound(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), 15*time.Second)
}
