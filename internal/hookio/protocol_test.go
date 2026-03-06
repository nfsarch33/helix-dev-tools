package hookio_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nfsarch33/cursor-tools/internal/hookio"
)

func TestHookio(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Hookio Suite")
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
		})

		It("writes ask response", func() {
			var buf bytes.Buffer
			resp := hookio.Ask("confirm?", "needs confirmation")
			Expect(hookio.WriteResponse(&buf, resp)).To(Succeed())

			var parsed hookio.Response
			Expect(json.Unmarshal(buf.Bytes(), &parsed)).To(Succeed())
			Expect(parsed.Continue).To(BeTrue())
			Expect(parsed.Permission).To(Equal("ask"))
		})

		It("writes empty response", func() {
			var buf bytes.Buffer
			resp := hookio.Empty()
			Expect(hookio.WriteResponse(&buf, resp)).To(Succeed())
			Expect(buf.String()).To(Equal("{}"))
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
})

type testHandler struct {
	resp *hookio.Response
}

func (h *testHandler) Handle(_ context.Context, _ *hookio.Input) (*hookio.Response, error) {
	return h.resp, nil
}
