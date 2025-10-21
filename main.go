package main

import (
	_ "embed"
	"fmt"
	"nicemaxxingbot/app/cmd"
	"nicemaxxingbot/app/util"
	"nicemaxxingbot/app/util/mylog"
	"os"

	"github.com/spf13/cobra"
	"go.szostok.io/version/extension"
)

func main() {
	mylog.Preinit()

	fmt.Fprintln(os.Stderr, util.Banner)

	rootCmd := &cobra.Command{Use: "nicemaxxingbot"}
	rootCmd.AddCommand(cmd.Run)
	rootCmd.AddCommand(extension.NewVersionCobraCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
		return
	}
}
