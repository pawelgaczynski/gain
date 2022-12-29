package gain_test

import (
	"testing"
)

func TestSharding(t *testing.T) {
	fixtures := []testServerConfig{
		{
			numberOfClients: 1,
			numberOfWorkers: 1,
		},
		{
			numberOfClients: 2,
			numberOfWorkers: 1,
		},
		{
			numberOfClients: 8,
			numberOfWorkers: 2,
		},
		{
			numberOfClients: 8,
			numberOfWorkers: 1,
		},
		{
			numberOfClients: 8,
			numberOfWorkers: 8,
		},
		{
			numberOfClients: 8,
			numberOfWorkers: 8,
			batchSubmitter:  true,
		},
		{
			numberOfClients:       8,
			numberOfWorkers:       8,
			prefillConnectionPool: true,
		},
		{
			numberOfClients: 8,
			numberOfWorkers: 16,
		},
		{
			numberOfClients: 8,
			numberOfWorkers: 1,
			asyncHandler:    true,
		},
		{
			numberOfClients: 8,
			numberOfWorkers: 1,
			asyncHandler:    true,
			goroutinePool:   true,
		},
		{
			numberOfClients:       8,
			numberOfWorkers:       4,
			waitForDialAllClients: true,
		},
	}
	for _, fixture := range fixtures {
		testServer(t, fixture, false)
	}
}
