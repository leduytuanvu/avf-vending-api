package activation

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidActivationMachineCode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want bool
	}{
		{"AVF000001", true},
		{"avf000001", true},
		{" AVF000001 ", true},
		{"AVF00001", false},
		{"AVF0000001", false},
		{"AVF00000A", false},
		{"XYZ000001", false},
		{"", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, validActivationMachineCode(tc.in))
		})
	}
}

func TestResolveMachineRef_emptyRef(t *testing.T) {
	t.Parallel()
	_, _, err := ResolveMachineRef(t.Context(), nil, "")
	require.ErrorIs(t, err, ErrMachineIdentifierRequired)
}

func TestResolveMachineRef_invalidFormat(t *testing.T) {
	t.Parallel()
	_, _, err := ResolveMachineRef(t.Context(), nil, "not-a-code")
	require.ErrorIs(t, err, ErrInvalidMachineIdentifier)
}

func TestResolveMachineRef_shortAVFCode(t *testing.T) {
	t.Parallel()
	_, _, err := ResolveMachineRef(t.Context(), nil, "AVF00001")
	require.ErrorIs(t, err, ErrInvalidMachineIdentifier)
}

func TestResolveMachineBody_empty(t *testing.T) {
	t.Parallel()
	_, _, err := ResolveMachineBody(t.Context(), nil, "", "", "", "")
	require.ErrorIs(t, err, ErrMachineIdentifierRequired)
}

func TestResolveMachineBody_invalidCode(t *testing.T) {
	t.Parallel()
	_, _, err := ResolveMachineBody(t.Context(), nil, "", "", "BAD", "")
	require.ErrorIs(t, err, ErrInvalidMachineIdentifier)
}
