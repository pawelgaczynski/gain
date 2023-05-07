package gain

import (
	"context"
	"testing"

	. "github.com/stretchr/testify/require"
)

func TestConnectionCtx(t *testing.T) {
	conn := newConnection()

	var ctx context.Context

	conn.SetContext(ctx)
	Equal(t, ctx, conn.Context())
}
