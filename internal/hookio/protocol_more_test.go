package hookio

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

type localTestHandler struct {
	resp *Response
}

func (h *localTestHandler) Handle(_ context.Context, _ *Input) (*Response, error) {
	return h.resp, nil
}

func TestReadStdinWriteStdoutAndRun(t *testing.T) {
	t.Run("ReadStdin parses os.Stdin", func(t *testing.T) {
		oldStdin := os.Stdin
		defer func() { os.Stdin = oldStdin }()

		file := filepath.Join(t.TempDir(), "stdin.json")
		if err := os.WriteFile(file, []byte(`{"command":"echo hi"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		in, err := os.Open(file)
		if err != nil {
			t.Fatal(err)
		}
		defer in.Close()
		os.Stdin = in

		input, err := ReadStdin()
		if err != nil {
			t.Fatalf("ReadStdin() error = %v", err)
		}
		if input.Command != "echo hi" {
			t.Fatalf("ReadStdin() command = %q", input.Command)
		}
	})

	t.Run("WriteStdout writes to os.Stdout", func(t *testing.T) {
		oldStdout := os.Stdout
		defer func() { os.Stdout = oldStdout }()

		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		os.Stdout = w
		if err := WriteStdout(Allow()); err != nil {
			t.Fatalf("WriteStdout() error = %v", err)
		}
		_ = w.Close()
		data := make([]byte, 64)
		n, err := r.Read(data)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(data[:n], []byte(`"permission":"allow"`)) {
			t.Fatalf("WriteStdout() output = %q", string(data[:n]))
		}
	})

	t.Run("Run exits with deny code", func(t *testing.T) {
		oldStdin := os.Stdin
		oldStdout := os.Stdout
		oldExit := runExit
		defer func() {
			os.Stdin = oldStdin
			os.Stdout = oldStdout
			runExit = oldExit
		}()

		inputFile := filepath.Join(t.TempDir(), "stdin.json")
		if err := os.WriteFile(inputFile, []byte(`{"command":"rm -rf /"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		in, err := os.Open(inputFile)
		if err != nil {
			t.Fatal(err)
		}
		defer in.Close()
		os.Stdin = in

		_, out, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		defer out.Close()
		os.Stdout = out

		exitCode := -1
		runExit = func(code int) {
			exitCode = code
			panic(code)
		}

		handler := &localTestHandler{resp: Deny("blocked", "danger")}
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic from runExit")
			}
			if exitCode != 2 {
				t.Fatalf("runExit code = %d, want 2", exitCode)
			}
		}()
		Run(handler)
	})
}
