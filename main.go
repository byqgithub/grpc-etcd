package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	command "github.com/spf13/cobra"

	client "grpc-etcd/client"
	server "grpc-etcd/service"
	"grpc-etcd/internal"
)

var rootCmd *command.Command

func parseOpt()  {
	rootCmd = &command.Command{
		Use:     "proto-test",
		Short:   "proto test",
		Version: "1.0.0",
	}

	var serverCmd = &command.Command{
		Use:     "server",
		Short:   "start server",
		Args:  command.MaximumNArgs(10),
		RunE: func(cmd *command.Command, args []string) error {
			err := runServer()
			if err != nil {
				fmt.Printf("Start server error: %v\n", err)
				return err
			}
			return nil
		},
	}

	var clientCmd = &command.Command{
		Use:     "client",
		Short:   "start client",
		Args:  command.MaximumNArgs(10),
		RunE: func(cmd *command.Command, args []string) error {
			err := runClient()
			if err != nil {
				fmt.Printf("Start client error: %v\n", err)
				return err
			}
			return nil
		},
	}

	var versionCmd = &command.Command{
		Use:   "version",
		Short: "show version",
		Run: func(cmd *command.Command, args []string) {
			fmt.Printf("proto-test version %s\n", "1.0.0")
		},
	}

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(clientCmd)
	rootCmd.SetVersionTemplate("1.0.0")
}

func runServer() error {
	abs, err := filepath.Abs(".")
	if err != nil {
		fmt.Printf("Get work dir error: %v\n", err)
		return nil
	}
	fmt.Printf("Current abs path: %v\n", abs)
	return server.StartServer(internal.ServicePort, abs)
}

func runClient() error {
	ctx := context.Background()
	abs, err := filepath.Abs(".")
	if err != nil {
		fmt.Printf("Get work dir error: %v\n", err)
		return nil
	}

	conn, cli:= client.InitClient(ctx, abs, internal.ClientDialAddr)
	defer client.CloseClient(conn)

	var (
		a, b, opt string
		num1, num2 float64
	)
	for {
		inputReader := bufio.NewReader(os.Stdin)
		fmt.Println("Please input a: ")
		a, err = inputReader.ReadString('\n')
		if err != nil {
			fmt.Printf("Input error: %s\n", err)
			break
		}
		a = strings.TrimRight(a, "\n")
		num1, err = strconv.ParseFloat(a, 64)
		if err != nil {
			fmt.Printf("String to float64 error: %s\n", err)
			break
		}

		fmt.Println("Please input operator: ")
		opt, err = inputReader.ReadString('\n')
		if err != nil {
			fmt.Printf("Input error: %s\n", err)
			break
		}
		opt = strings.TrimRight(opt, "\n")

		fmt.Println("Please input b: ")
		b, err = inputReader.ReadString('\n')
		if err != nil {
			fmt.Printf("Input error: %s\n", err)
			break
		}
		b = strings.TrimRight(b, "\n")
		num2, err = strconv.ParseFloat(b, 64)
		if err != nil {
			fmt.Printf("String to float64 error: %s\n", err)
			break
		}

		result, err := client.Calcuate(ctx, cli, num1, num2, opt)
		if err == nil {
			fmt.Printf("Calcuation result: %v\n", result)
		}
	}

	return nil
}

func main() {
	parseOpt()

	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("Main panic %v\n", err)
		}
	}()

	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("Root command error: %v\n", err)
	}
}