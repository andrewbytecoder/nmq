package main

import (
	"fmt"
	"github.com/nmq/pkg/nmq"
	"github.com/spf13/cobra"
)

func main() {
	run := nmq.NewNcp()
	run.AddCommand(&cobra.Command{
		Use:   "start",
		Short: "启动服务",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("正在启动服务...")
		},
	})

	err := run.Execute()
	if err != nil {
		fmt.Println("Failed to execute nmq")
		return
	}
}
