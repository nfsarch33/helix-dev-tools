package health

import "testing"

func TestWindowsHooksJSONPolicy(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		wantOK bool
	}{
		{
			name:   "tilde_unix_style_rejected",
			body:   `{"command":"~/bin/cursor-tools hook guard-shell"}`,
			wantOK: false,
		},
		{
			name:   "absolute_exe_accepted",
			body:   `{"command":"C:\\Users\\x\\bin\\cursor-tools.exe hook guard-shell"}`,
			wantOK: true,
		},
		{
			name:   "forward_slashes_exe_accepted",
			body:   `{"command":"C:/Users/x/bin/cursor-tools.exe hook guard-shell"}`,
			wantOK: true,
		},
		{
			name:   "noconsole_release_exe_accepted",
			body:   `{"command":"C:\\Users\\x\\bin\\cursor-tools-windows-amd64-noconsole.exe hook guard-shell"}`,
			wantOK: true,
		},
		{
			name:   "no_exe_suffix_rejected",
			body:   `{"command":"C:\\Users\\x\\bin\\cursor-tools hook guard-shell"}`,
			wantOK: false,
		},
		{
			name:   "missing_cursor_tools_rejected",
			body:   `{"command":"echo hi"}`,
			wantOK: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, _ := windowsHooksJSONPolicy(tc.body)
			if ok != tc.wantOK {
				t.Fatalf("windowsHooksJSONPolicy(...) ok=%v want %v", ok, tc.wantOK)
			}
		})
	}
}
