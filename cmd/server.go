package cmd

import (
	"fmt"
	"net/http"

	"buh/internal/web"

	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the web UI server for processing paušal payment slips",
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetString("port")
		password, _ := cmd.Flags().GetString("password")
		addr := ":" + port
		fmt.Printf("Server running at http://localhost%s\n", addr)
		return http.ListenAndServe(addr, web.NewHandler(password))
	},
}

func init() {
	serverCmd.Flags().StringP("port", "p", "8080", "Port to listen on")
	serverCmd.Flags().String("password", "pausal", "Password for web UI access")
	rootCmd.AddCommand(serverCmd)
}
