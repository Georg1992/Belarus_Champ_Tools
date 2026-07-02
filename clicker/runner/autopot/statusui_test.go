package autopot

import (
	"context"
	"testing"

	"experimental-clicker/runner/statusui"
)

// TestRunStatusUIReturnsNilOnCancel verifies that runStatusUI returns nil
// when the context is cancelled immediately — signalling a normal Stop
// rather than triggering a fallback. The ctx.Err() check at the top
// ensures Stop works even during initialisation.
func TestRunStatusUIReturnsNilOnCancel(t *testing.T) {
	a := NewAutoPot(AutoPotConfig{
		HPEnabled: true,
		HPKeyVK:   'W',
		Log:       func(string) {},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Build a poller from a default pipeline so the function can be called.
	pipeline, err := statusui.NewDefaultPipeline()
	if err != nil {
		t.Skipf("skipping: cannot create pipeline in test env: %v", err)
	}
	poller := statusui.NewStripPoller(pipeline)

	err = a.runStatusUI(ctx, poller)
	if err != nil {
		t.Fatalf("runStatusUI with cancelled ctx: want nil, got %v", err)
	}
}
