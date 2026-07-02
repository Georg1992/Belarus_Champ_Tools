package autopot

import (
	"context"
	"testing"

	"belarus-champ-tools/runner/statusui"
)

// TestStatusUIReaderCancel verifies that ReadBars returns an error
// (context.Canceled) when called with an already-cancelled context.
func TestStatusUIReaderCancel(t *testing.T) {
	pipeline, err := statusui.NewDefaultPipeline()
	if err != nil {
		t.Skipf("skipping: cannot create pipeline in test env: %v", err)
	}

	reader := &statusUIReader{
		poller: statusui.NewStripPoller(pipeline),
		log:    func(string) {},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := reader.ReadBars(ctx)
	if result.Err == nil {
		t.Fatal("ReadBars with cancelled ctx: want error, got nil")
	}
}
