package workfloworch

import (
	"testing"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/workflow"
)

type fakeRegistrar struct {
	workflows  []string
	activities []string
}

func (f *fakeRegistrar) RegisterWorkflowWithOptions(_ any, options workflow.RegisterOptions) {
	f.workflows = append(f.workflows, options.Name)
}

func (f *fakeRegistrar) RegisterActivityWithOptions(_ any, options activity.RegisterOptions) {
	f.activities = append(f.activities, options.Name)
}

func TestRegisterAll_RegistersExpectedWorkflowsAndActivities(t *testing.T) {
	t.Parallel()
	reg := &fakeRegistrar{}
	err := RegisterAll(reg, ActivityDeps{
		Lifecycle:  stubLifecycleStore{},
		RefundSink: &stubRefundSink{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.workflows) != 7 {
		t.Fatalf("workflow registrations=%d", len(reg.workflows))
	}
	if len(reg.activities) != 8 {
		t.Fatalf("activity registrations=%d", len(reg.activities))
	}
}
