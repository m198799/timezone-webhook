// Package cmd ...
package cmd

import (
	"github.com/spf13/cobra"

	"github.com/m198799/timezone-webhook/internal/admission"
)

var webhook = admission.NewAdmissionServer()

var webhookCmd = &cobra.Command{
	Use:   "webhook",
	Short: "Start api-server Webhook AdmissionServer for inject  zoneinfo",
	PreRun: func(cmd *cobra.Command, args []string) {
		// check zoneinfo configmap is existed
	},
	Run: func(cmd *cobra.Command, args []string) {
		// run webhook server
		cobra.CheckErr(webhook.Start(kubeConfigFile))
	},
	PostRun: func(cmd *cobra.Command, args []string) {

	},
}

func init() {
	rootCmd.AddCommand(webhookCmd)

	webhookCmd.Flags().StringVar(&webhook.TLSCertFile, "tls-crt", webhook.TLSCertFile, "TLS Certificate file")
	webhookCmd.Flags().StringVar(&webhook.TLSKeyFile, "tls-key", webhook.TLSKeyFile, "TLS Key file")
	webhookCmd.Flags().StringVar(&webhook.Address, "addr", webhook.Address, "Webhook bind address")
	webhookCmd.Flags().StringVarP(&webhook.Handler.DefaultTimezone, "timezone", "t", webhook.Handler.DefaultTimezone, "Default timezone if not specified explicitly")
	webhookCmd.Flags().StringVar(&webhook.Handler.HostPathPrefix, "hostPathPrefix", webhook.Handler.HostPathPrefix, "Location of zoneinfo on host machines")
	webhookCmd.Flags().StringVar(&webhook.Handler.LocalTimePath, "localTimePath", webhook.Handler.LocalTimePath, "Mount path for TZif file on containers")
	webhookCmd.Flags().StringVarP((*string)(&webhook.Handler.DefaultInjectionStrategy), "injection-strategy", "s", string(webhook.Handler.DefaultInjectionStrategy), "Default injection strategy if not specified explicitly (hostPath/configmap)")
	webhookCmd.Flags().BoolVar(&webhook.Handler.InjectByDefault, "inject", webhook.Handler.InjectByDefault, "Whether injection is enabled by default or should be requested by annotation")
	webhookCmd.Flags().BoolVar(&webhook.Verbose, "verbose", webhook.Verbose, "Print more verbose logs for debugging")
	webhookCmd.Flags().StringVar(&webhook.Handler.ConfigMapName, "configmap", webhook.Handler.ConfigMapName, "When configmap inject timezone,this is configmap name")
	webhookCmd.Flags().StringVar(&webhook.Handler.ZoneInfoNamespaces, "namespaces", webhook.Handler.ZoneInfoNamespaces, "Handler TimeZone Namespace")
	webhookCmd.Flags().BoolVar(&webhook.Handler.InjectNamespaceAnnotation, "injectNamespaceAnnotation", webhook.Handler.InjectNamespaceAnnotation, "Whether namespace annotations are enabled for injection")
}
