package cmd

import (
	"strings"

	"github.com/bttnns/harvey/internal/runtime"
	"github.com/spf13/cobra"
)

var (
	serveName  string
	servePorts []string
)

var serveCmd = &cobra.Command{
	Use:   "serve [command...]",
	Short: "Start a detached, port-published container",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ports := make([]string, len(servePorts))
		for i, p := range servePorts {
			ports[i] = normalizePort(p)
		}
		return launch(runtime.RunOpts{
			Detached: true,
			Name:     serveName,
			Ports:    ports,
			Command:  strings.Join(args, " "),
		})
	},
}

// normalizePort turns 3000 into 3000:3000 and leaves HOST:CONTAINER untouched.
func normalizePort(p string) string {
	if strings.Contains(p, ":") {
		return p
	}
	return p + ":" + p
}

func init() {
	serveCmd.Flags().SetInterspersed(false)
	serveCmd.Flags().StringVar(&serveName, "name", "", "container name (required)")
	serveCmd.Flags().StringArrayVarP(&servePorts, "publish", "p", nil, "publish PORT or HOST:CONTAINER (repeatable)")
	_ = serveCmd.MarkFlagRequired("name")
	rootCmd.AddCommand(serveCmd)
}
