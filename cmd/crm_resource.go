package cmd

import "github.com/spf13/cobra"

const (
	crmDynamicResourceAnnotation = "solactl.crm.dynamicResource"
	crmDynamicCommandAnnotation  = "solactl.crm.dynamicCommand"
	crmStaticResourceAnnotation  = "solactl.crm.staticResource"
)

func ensureCRMResourceCommand(resource, short string) (*cobra.Command, bool) {
	for _, child := range crmCmd.Commands() {
		if child.Name() == resource {
			if child.Annotations == nil {
				child.Annotations = map[string]string{}
			}
			return child, false
		}
	}

	cmd := &cobra.Command{
		Use:         resource,
		Short:       short,
		Annotations: map[string]string{},
	}
	crmCmd.AddCommand(cmd)
	return cmd, true
}

func ensureStaticCRMResourceCommand(resource, short string) *cobra.Command {
	cmd, _ := ensureCRMResourceCommand(resource, short)
	cmd.Annotations[crmStaticResourceAnnotation] = "true"
	return cmd
}

func markDynamicCRMResourceCommand(cmd *cobra.Command) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	if cmd.Annotations[crmStaticResourceAnnotation] != "true" {
		cmd.Annotations[crmDynamicResourceAnnotation] = "true"
	}
}

func markDynamicCRMCommand(cmd *cobra.Command) *cobra.Command {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[crmDynamicCommandAnnotation] = "true"
	return cmd
}
