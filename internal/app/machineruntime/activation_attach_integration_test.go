package machineruntime

import (
	"testing"

	"github.com/google/uuid"
)

func TestActivationAttachInput_activationCodeDoesNotRequireOperator(t *testing.T) {
	in := AttachInput{
		MachineID:       uuid.New(),
		RequireOperator: false,
	}
	if in.RequireOperator {
		t.Fatal("activation-code attach must not require operator session")
	}
}

func TestActivationAttachInput_noTechnicianField(t *testing.T) {
	in := ActivationAttachInput{
		MachineID:        uuid.New(),
		ActivationSource: "activation_code",
		Reason:           "first_install",
	}
	if in.ActivationSource != "activation_code" {
		t.Fatalf("unexpected activation source %q", in.ActivationSource)
	}
}
