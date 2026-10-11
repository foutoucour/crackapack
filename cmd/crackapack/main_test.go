package main

import (
	"io"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		args           []string
		wantErr        bool
		wantErrContain string
		wantUsage      bool
	}{
		{
			name:           "simulate with no flags returns missing required flags error",
			args:           []string{"simulate"},
			wantErr:        true,
			wantErrContain: "crackapack simulate",
			wantUsage:      true,
		},
		{
			name:           "sweep with no arguments returns an error naming the subcommand",
			args:           []string{"sweep"},
			wantErr:        true,
			wantErrContain: "sweep",
		},
		{
			name:           "compare with no arguments returns an error naming the subcommand",
			args:           []string{"compare"},
			wantErr:        true,
			wantErrContain: "compare",
		},
		{
			name:    "demo with no arguments runs the demo sweep and succeeds",
			args:    []string{"demo"},
			wantErr: false,
		},
		{
			name:           "unknown subcommand returns usage error",
			args:           []string{"bogus"},
			wantErr:        true,
			wantErrContain: `unknown subcommand "bogus"`,
			wantUsage:      true,
		},
		{
			name:           "no args returns usage error",
			args:           []string{},
			wantErr:        true,
			wantErrContain: "no subcommand given",
			wantUsage:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var stderr strings.Builder
			err := run(tc.args, io.Discard, &stderr)

			if tc.wantErr && err == nil {
				t.Fatal("got nil error, want non-nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("got error %v, want nil", err)
			}
			if tc.wantErrContain != "" && !strings.Contains(err.Error(), tc.wantErrContain) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantErrContain)
			}
			if tc.wantUsage && !strings.Contains(stderr.String(), "crackapack") {
				t.Errorf("stderr %q does not contain usage text", stderr.String())
			}
		})
	}
}
