package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"go.temporal.io/sdk/client"

	temporalclient "github.com/windkube/aws-metrics-exporter/internal/temporal"
	"github.com/windkube/aws-metrics-exporter/internal/temporal/schedules"
)

var scheduleAccount string

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Manage the per-account Temporal schedules",
}

var scheduleApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Reconcile Temporal schedules with the configuration file",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return withTemporal(cmd, func(rt *runtime, c client.Client) error {
			changes, err := schedules.NewReconciler(c, rt.registry, rt.cfg.Global.Temporal.TaskQueue).
				Apply(cmd.Context(), rt.cfg)
			for _, change := range changes {
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", change.Action, change.ID)
			}
			return err
		})
	},
}

var scheduleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the schedules owned by this exporter",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return withTemporal(cmd, func(_ *runtime, c client.Client) error {
			iter, err := c.ScheduleClient().List(cmd.Context(), client.ScheduleListOptions{PageSize: 100})
			if err != nil {
				return err
			}

			for iter.HasNext() {
				entry, err := iter.Next()
				if err != nil {
					return err
				}
				if !strings.HasPrefix(entry.ID, schedules.IDPrefix) {
					continue
				}

				fmt.Fprintf(cmd.OutOrStdout(), "%s\tpaused=%t\tnext=%v\n",
					entry.ID, entry.Paused, entry.NextActionTimes)
			}

			return nil
		})
	},
}

func withTemporal(cmd *cobra.Command, run func(*runtime, client.Client) error) error {
	rt, err := bootstrap(cmd.Context())
	if err != nil {
		return err
	}
	defer rt.Close()

	c, err := temporalclient.NewClient(rt.cfg.Global.Temporal)
	if err != nil {
		return err
	}
	defer c.Close()

	return run(rt, c)
}

func scheduleHandleCmd(use, short string, run func(*cobra.Command, client.ScheduleHandle) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return withTemporal(cmd, func(_ *runtime, c client.Client) error {
				return run(cmd, c.ScheduleClient().GetHandle(cmd.Context(), schedules.ID(scheduleAccount)))
			})
		},
	}

	cmd.Flags().StringVar(&scheduleAccount, "account", "", "account name from the config file (required)")
	_ = cmd.MarkFlagRequired("account")

	return cmd
}

func init() {
	scheduleCmd.AddCommand(
		scheduleApplyCmd,
		scheduleListCmd,
		scheduleHandleCmd("pause", "Pause an account's schedule", func(cmd *cobra.Command, h client.ScheduleHandle) error {
			return h.Pause(cmd.Context(), client.SchedulePauseOptions{Note: "paused via aws-metrics-exporter cli"})
		}),
		scheduleHandleCmd("resume", "Resume an account's schedule", func(cmd *cobra.Command, h client.ScheduleHandle) error {
			return h.Unpause(cmd.Context(), client.ScheduleUnpauseOptions{Note: "resumed via aws-metrics-exporter cli"})
		}),
		scheduleHandleCmd("trigger", "Run an account's schedule immediately", func(cmd *cobra.Command, h client.ScheduleHandle) error {
			return h.Trigger(cmd.Context(), client.ScheduleTriggerOptions{})
		}),
	)

	rootCmd.AddCommand(scheduleCmd)
}
