package httpserver

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/avf/avf-vending-api/internal/domain/deviceconfig"
)

func TestPutAdminMachineTopology_deviceConfigValidationCodes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		meta string
		code string
	}{
		{
			name: "topology without device_config passes",
			meta: `{"board_protocol":"tcn","bill_protocol":"ict_bc_v1","cash_topology":"direct_bill"}`,
			code: "",
		},
		{
			name: "empty device_config object",
			meta: `{"device_config":{}}`,
			code: "device_config_empty",
		},
		{
			name: "invalid denomination",
			meta: `{"device_config":{"schemaVersion":1,"bill":{"enabledDenominationsVnd":[777],"changeDenominationVnd":777},"tcn":{"laneMode":10,"shakeCount":2,"dropWaitSeconds":3}}}`,
			code: "device_config_bill_invalid_denomination",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := deviceconfig.ValidateMetadataDeviceConfig(json.RawMessage(tc.meta))
			if tc.code == "" {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				return
			}
			var ve deviceconfig.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("expected ValidationError, got %T: %v", err, err)
			}
			if ve.Code != tc.code {
				t.Fatalf("code=%q want %q", ve.Code, tc.code)
			}
		})
	}
}
