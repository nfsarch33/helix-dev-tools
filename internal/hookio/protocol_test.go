package hookio_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nfsarch33/helix-dev-tools/internal/hookio"
)

func TestHookio(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Hookio Suite")
}

type errorReader struct{}

func (r *errorReader) Read(_ []byte) (int, error) {
	return 0, errors.New("simulated read failure")
}

type errorWriter struct{}

func (w *errorWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("simulated write failure")
}

type errorHandler struct{}

func (h *errorHandler) Handle(_ context.Context, _ *hookio.Input) (*hookio.Response, error) {
	return nil, errors.New("handler failed")
}

var _ = Describe("Protocol", func() {
	Describe("ReadInput", func() {
		It("parses valid JSON with command field", func() {
			r := strings.NewReader(`{"command":"ls -la"}`)
			input, err := hookio.ReadInput(r)
			Expect(err).NotTo(HaveOccurred())
			Expect(input.Command).To(Equal("ls -la"))
		})

		It("parses valid JSON with file_path field", func() {
			r := strings.NewReader(`{"file_path":"/tmp/test.go"}`)
			input, err := hookio.ReadInput(r)
			Expect(err).NotTo(HaveOccurred())
			Expect(input.FilePath).To(Equal("/tmp/test.go"))
		})

		It("parses valid JSON with tool_name and tool_input", func() {
			r := strings.NewReader(`{"tool_name":"create_issue","tool_input":"{}"}`)
			input, err := hookio.ReadInput(r)
			Expect(err).NotTo(HaveOccurred())
			Expect(input.ToolName).To(Equal("create_issue"))
			Expect(input.ToolInput).To(Equal("{}"))
		})

		It("parses valid JSON with status field", func() {
			r := strings.NewReader(`{"status":"completed"}`)
			input, err := hookio.ReadInput(r)
			Expect(err).NotTo(HaveOccurred())
			Expect(input.Status).To(Equal("completed"))
		})

		It("returns empty input for empty reader", func() {
			r := strings.NewReader("")
			input, err := hookio.ReadInput(r)
			Expect(err).NotTo(HaveOccurred())
			Expect(input.Command).To(BeEmpty())
		})

		It("returns error for invalid JSON", func() {
			r := strings.NewReader("{not json}")
			_, err := hookio.ReadInput(r)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("parse JSON"))
		})

		It("returns error when reader fails", func() {
			_, err := hookio.ReadInput(&errorReader{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("read stdin"))
		})

		It("parses JSON with all fields populated", func() {
			r := strings.NewReader(`{"command":"cmd","file_path":"/f","tool_name":"t","tool_input":"i","status":"s"}`)
			input, err := hookio.ReadInput(r)
			Expect(err).NotTo(HaveOccurred())
			Expect(input.Command).To(Equal("cmd"))
			Expect(input.FilePath).To(Equal("/f"))
			Expect(input.ToolName).To(Equal("t"))
			Expect(input.ToolInput).To(Equal("i"))
			Expect(input.Status).To(Equal("s"))
		})

		It("ignores unknown JSON fields", func() {
			r := strings.NewReader(`{"command":"ls","extra":"ignored"}`)
			input, err := hookio.ReadInput(r)
			Expect(err).NotTo(HaveOccurred())
			Expect(input.Command).To(Equal("ls"))
		})
	})

	Describe("WriteResponse", func() {
		It("writes allow response", func() {
			var buf bytes.Buffer
			resp := hookio.Allow()
			Expect(hookio.WriteResponse(&buf, resp)).To(Succeed())

			var parsed hookio.Response
			Expect(json.Unmarshal(buf.Bytes(), &parsed)).To(Succeed())
			Expect(parsed.Continue).To(BeTrue())
			Expect(parsed.Permission).To(Equal("allow"))
		})

		It("writes deny response with messages", func() {
			var buf bytes.Buffer
			resp := hookio.Deny("blocked", "reason")
			Expect(hookio.WriteResponse(&buf, resp)).To(Succeed())

			var parsed hookio.Response
			Expect(json.Unmarshal(buf.Bytes(), &parsed)).To(Succeed())
			Expect(parsed.Permission).To(Equal("deny"))
			Expect(parsed.UserMessage).To(Equal("blocked"))
			Expect(parsed.AgentMessage).To(Equal("reason"))
			Expect(parsed.Continue).To(BeFalse())
		})

		It("writes ask response", func() {
			var buf bytes.Buffer
			resp := hookio.Ask("confirm?", "needs confirmation")
			Expect(hookio.WriteResponse(&buf, resp)).To(Succeed())

			var parsed hookio.Response
			Expect(json.Unmarshal(buf.Bytes(), &parsed)).To(Succeed())
			Expect(parsed.Continue).To(BeTrue())
			Expect(parsed.Permission).To(Equal("ask"))
			Expect(parsed.UserMessage).To(Equal("confirm?"))
			Expect(parsed.AgentMessage).To(Equal("needs confirmation"))
		})

		It("writes empty response", func() {
			var buf bytes.Buffer
			resp := hookio.Empty()
			Expect(hookio.WriteResponse(&buf, resp)).To(Succeed())
			Expect(buf.String()).To(Equal("{}"))
		})

		It("returns error when writer fails", func() {
			resp := hookio.Allow()
			err := hookio.WriteResponse(&errorWriter{}, resp)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("RunWithIO", func() {
		It("allows valid input with allow handler", func() {
			r := strings.NewReader(`{"command":"ls"}`)
			var w bytes.Buffer
			h := &testHandler{resp: hookio.Allow()}

			code := hookio.RunWithIO(h, r, &w)
			Expect(code).To(Equal(0))

			var parsed hookio.Response
			Expect(json.Unmarshal(w.Bytes(), &parsed)).To(Succeed())
			Expect(parsed.Permission).To(Equal("allow"))
		})

		It("returns exit code 2 for deny", func() {
			r := strings.NewReader(`{"command":"rm -rf /"}`)
			var w bytes.Buffer
			h := &testHandler{resp: hookio.Deny("blocked", "destructive")}

			code := hookio.RunWithIO(h, r, &w)
			Expect(code).To(Equal(2))

			var parsed hookio.Response
			Expect(json.Unmarshal(w.Bytes(), &parsed)).To(Succeed())
			Expect(parsed.Permission).To(Equal("deny"))
		})

		It("falls back to allow on read error", func() {
			var w bytes.Buffer
			h := &testHandler{resp: hookio.Deny("blocked", "should not reach")}

			code := hookio.RunWithIO(h, &errorReader{}, &w)
			Expect(code).To(Equal(0))

			var parsed hookio.Response
			Expect(json.Unmarshal(w.Bytes(), &parsed)).To(Succeed())
			Expect(parsed.Permission).To(Equal("allow"))
		})

		It("falls back to allow on handler error", func() {
			r := strings.NewReader(`{"command":"test"}`)
			var w bytes.Buffer

			code := hookio.RunWithIO(&errorHandler{}, r, &w)
			Expect(code).To(Equal(0))

			var parsed hookio.Response
			Expect(json.Unmarshal(w.Bytes(), &parsed)).To(Succeed())
			Expect(parsed.Permission).To(Equal("allow"))
		})

		It("returns 0 for ask responses", func() {
			r := strings.NewReader(`{"command":"dangerous"}`)
			var w bytes.Buffer
			h := &testHandler{resp: hookio.Ask("confirm?", "risky")}

			code := hookio.RunWithIO(h, r, &w)
			Expect(code).To(Equal(0))
		})

		It("returns 0 for empty responses", func() {
			r := strings.NewReader(`{"status":"completed"}`)
			var w bytes.Buffer
			h := &testHandler{resp: hookio.Empty()}

			code := hookio.RunWithIO(h, r, &w)
			Expect(code).To(Equal(0))
		})

		It("handles empty input gracefully", func() {
			r := strings.NewReader("")
			var w bytes.Buffer
			h := &testHandler{resp: hookio.Allow()}

			code := hookio.RunWithIO(h, r, &w)
			Expect(code).To(Equal(0))
		})
	})

	Describe("Handler interface", func() {
		It("can be implemented and invoked", func() {
			h := &testHandler{resp: hookio.Allow()}
			input := &hookio.Input{Command: "test"}
			resp, err := h.Handle(context.Background(), input)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Permission).To(Equal("allow"))
		})
	})

	Describe("Response constructors", func() {
		It("Deny sets continue to false", func() {
			resp := hookio.Deny("msg", "agent")
			Expect(resp.Continue).To(BeFalse())
		})

		It("Ask sets continue to true", func() {
			resp := hookio.Ask("msg", "agent")
			Expect(resp.Continue).To(BeTrue())
		})

		It("Allow sets continue to true", func() {
			resp := hookio.Allow()
			Expect(resp.Continue).To(BeTrue())
		})

		It("Empty returns zero-value response", func() {
			resp := hookio.Empty()
			Expect(resp.Continue).To(BeFalse())
			Expect(resp.Permission).To(BeEmpty())
		})
	})

	Describe("ReadInput round-trip", func() {
		It("marshalled response can be parsed back", func() {
			original := hookio.Deny("user msg", "agent msg")
			var buf bytes.Buffer
			Expect(hookio.WriteResponse(&buf, original)).To(Succeed())

			var roundTrip hookio.Response
			Expect(json.Unmarshal(buf.Bytes(), &roundTrip)).To(Succeed())
			Expect(roundTrip).To(Equal(*original))
		})
	})
})

// compile-time assertion that Input/Response can be used as io.Reader payloads
var _ io.Reader = strings.NewReader("")

type testHandler struct {
	resp *hookio.Response
}

func (h *testHandler) Handle(_ context.Context, _ *hookio.Input) (*hookio.Response, error) {
	return h.resp, nil
}
