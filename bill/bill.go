package bill

import (
	"context"
	"fmt"
	"os"
	"regexp"

	"encore.app/bill/billworkflow"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

//encore:service
type Service struct {
	client client.Client
	worker worker.Worker
}

func initService() (*Service, error) {
	hostPort := os.Getenv("TEMPORAL_HOST_PORT")
	if hostPort == "" {
		hostPort = "127.0.0.1:7233"
	}

	c, err := client.Dial(client.Options{HostPort: hostPort})
	if err != nil {
		return nil, fmt.Errorf("create temporal client: %w", err)
	}

	w := worker.New(c, billworkflow.TaskQueue, worker.Options{})
	w.RegisterWorkflow(billworkflow.BillWorkflow)

	if err := w.Start(); err != nil {
		c.Close()
		return nil, fmt.Errorf("start temporal worker: %w", err)
	}

	return &Service{client: c, worker: w}, nil
}

func (s *Service) Shutdown(force context.Context) {
	s.worker.Stop()
	s.client.Close()
}

func billWorkflowID(billID string) string {
	return "bill:" + billID
}

var billIDPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)

func isValidBillID(billID string) bool {
	return billIDPattern.MatchString(billID)
}
