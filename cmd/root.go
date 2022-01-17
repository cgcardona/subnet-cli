// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/ava-labs/subnet-cli/pkg/logutil"
)

var rootCmd = &cobra.Command{
	Use:        "subnet-cli",
	Short:      "subnet-cli CLI",
	SuggestFor: []string{"subnet-cli", "subnetcli", "subnetctl"},
}

var (
	enablePrompt bool
	logLevel     string

	privKeyPath string

	uri string

	pollInterval   time.Duration
	requestTimeout time.Duration

	subnetIDs string
	nodeIDs   string

	validateStarts string
	validateEnds   string
	validateWeight uint64

	vmName        string
	vmIDs         string
	vmGenesisPath string
)

func init() {
	cobra.EnablePrefixMatching = true

	rootCmd.AddCommand(
		CreateCommand(),
		AddCommand(),
		StatusCommand(),
	)

	rootCmd.PersistentFlags().BoolVar(&enablePrompt, "enable-prompt", true, "'true' to enable prompt mode")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", logutil.DefaultLogLevel, "log level")
	rootCmd.PersistentFlags().StringVar(&uri, "uri", "https://api.avax-test.network", "URI for avalanche network endpoints")
	rootCmd.PersistentFlags().DurationVar(&pollInterval, "poll-interval", time.Second, "interval to poll tx/blockchain status")
	rootCmd.PersistentFlags().DurationVar(&requestTimeout, "request-timeout", 2*time.Minute, "request timeout")
}

func Execute() error {
	if err := CreateLogger(); err != nil {
		return err
	}
	return rootCmd.Execute()
}
