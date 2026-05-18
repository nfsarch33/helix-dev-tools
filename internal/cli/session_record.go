package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/nfsarch33/helix-dev-tools/internal/platform/sessionhook"
	"github.com/nfsarch33/helix-dev-tools/internal/platform/sprintboard"
	"github.com/spf13/cobra"
)

var sessionRecordCmd = &cobra.Command{
	Use:   "session-record",
	Short: "Record session lifecycle in sprint board (used by hooks)",
	RunE: func(cmd *cobra.Command, args []string) error {
		action, _ := cmd.Flags().GetString("action")

		result, err := sessionhook.RunSessionHook(
			sessionhook.SessionAction(action),
			sprintboard.DefaultDBPath(),
		)
		if err != nil {
			return err
		}

		data, _ := json.Marshal(result)
		fmt.Fprintln(os.Stdout, string(data))
		return nil
	},
}

func init() {
	sessionRecordCmd.Flags().String("action", "start", "Hook action: start or stop")
	hookCmd.AddCommand(sessionRecordCmd)
}
