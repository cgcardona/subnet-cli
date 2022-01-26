// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/dustin/go-humanize"
	"github.com/olekukonko/tablewriter"
	"github.com/onsi/ginkgo/v2/formatter"
	"go.uber.org/zap"

	"github.com/ava-labs/subnet-cli/client"
	"github.com/ava-labs/subnet-cli/internal/key"
	"github.com/ava-labs/subnet-cli/pkg/color"
	"github.com/ava-labs/subnet-cli/pkg/logutil"
)

type ValInfo struct {
	start time.Time
	end   time.Time
}

type Info struct {
	uri string

	feeData *info.GetTxFeeResponse
	balance uint64

	txFee           uint64
	stakeAmount     uint64
	requiredBalance uint64

	key key.Key

	networkName string

	subnetIDType string
	subnetID     ids.ID

	nodeIDs    []ids.ShortID
	allNodeIDs []ids.ShortID
	valInfos   map[ids.ShortID]*ValInfo

	blockchainID  ids.ID
	chainName     string
	vmID          ids.ID
	vmGenesisPath string

	validateStart            time.Time
	validateEnd              time.Time
	validateWeight           uint64
	validateRewardFeePercent uint32

	rewardAddr ids.ShortID
	changeAddr ids.ShortID
}

func InitClient(uri string, loadKey bool) (client.Client, *Info, error) {
	cli, err := client.New(client.Config{
		URI:            uri,
		PollInterval:   pollInterval,
		RequestTimeout: requestTimeout,
	})
	if err != nil {
		return nil, nil, err
	}
	txFee, err := cli.Info().Client().GetTxFee()
	if err != nil {
		return nil, nil, err
	}
	networkName, err := cli.Info().Client().GetNetworkName()
	if err != nil {
		return nil, nil, err
	}
	info := &Info{
		uri:         uri,
		feeData:     txFee,
		networkName: networkName,
		valInfos:    map[ids.ShortID]*ValInfo{},
	}
	if !loadKey {
		return cli, info, nil
	}

	info.key, err = key.Load(cli.NetworkID(), privKeyPath)
	if err != nil {
		return nil, nil, err
	}
	info.balance, err = cli.P().Balance(info.key)
	if err != nil {
		return nil, nil, err
	}
	return cli, info, nil
}

func CreateLogger() error {
	lcfg := logutil.GetDefaultZapLoggerConfig()
	lcfg.Level = zap.NewAtomicLevelAt(logutil.ConvertToZapLevel(logLevel))
	logger, err := lcfg.Build()
	if err != nil {
		return err
	}
	_ = zap.ReplaceGlobals(logger)
	return nil
}

func (i *Info) CheckBalance() error {
	if i.balance < i.requiredBalance {
		color.Outf("{{red}}insufficient funds to perform operation. get more at https://faucet.avax-test.network{{/}}\n")
		return fmt.Errorf("%w: on %s (expected=%d, have=%d)", ErrInsufficientFunds, i.key.P(), i.requiredBalance, i.balance)
	}
	return nil
}

func BaseTableSetup(i *Info) (*bytes.Buffer, *tablewriter.Table) {
	// P-Chain balance is denominated by units.Avax or 10^9 nano-Avax
	curPChainDenominatedP := float64(i.balance) / float64(units.Avax)
	curPChainDenominatedBalanceP := humanize.FormatFloat("#,###.#######", curPChainDenominatedP)

	buf := bytes.NewBuffer(nil)
	tb := tablewriter.NewWriter(buf)

	tb.SetAutoWrapText(false)
	tb.SetColWidth(1500)
	tb.SetCenterSeparator("*")

	tb.SetRowLine(true)
	tb.SetAlignment(tablewriter.ALIGN_LEFT)

	tb.Append([]string{formatter.F("{{cyan}}{{bold}}P-CHAIN ADDRESS{{/}}"), formatter.F("{{light-gray}}{{bold}}%s{{/}}", i.key.P())})
	tb.Append([]string{formatter.F("{{coral}}{{bold}}P-CHAIN BALANCE{{/}} "), formatter.F("{{light-gray}}{{bold}}{{underline}}%s{{/}} $AVAX", curPChainDenominatedBalanceP)})
	if i.txFee > 0 {
		txFee := float64(i.txFee) / float64(units.Avax)
		txFees := humanize.FormatFloat("#,###.###", txFee)
		tb.Append([]string{formatter.F("{{red}}{{bold}}TX FEE{{/}}"), formatter.F("{{light-gray}}{{bold}}{{underline}}%s{{/}} $AVAX", txFees)})
	}
	if i.stakeAmount > 0 {
		stakeAmount := float64(i.stakeAmount) / float64(units.Avax)
		stakeAmounts := humanize.FormatFloat("#,###.###", stakeAmount)
		tb.Append([]string{formatter.F("{{red}}{{bold}}STAKE AMOUNT{{/}}"), formatter.F("{{light-gray}}{{bold}}{{underline}}%s{{/}} $AVAX", stakeAmounts)})
	}
	if i.requiredBalance > 0 {
		requiredBalance := float64(i.requiredBalance) / float64(units.Avax)
		requiredBalances := humanize.FormatFloat("#,###.###", requiredBalance)
		tb.Append([]string{formatter.F("{{red}}{{bold}}REQUIRED BALANCE{{/}}"), formatter.F("{{light-gray}}{{bold}}{{underline}}%s{{/}} $AVAX", requiredBalances)})
	}

	tb.Append([]string{formatter.F("{{orange}}URI{{/}}"), formatter.F("{{light-gray}}{{bold}}%s{{/}}", i.uri)})
	tb.Append([]string{formatter.F("{{orange}}NETWORK NAME{{/}}"), formatter.F("{{light-gray}}{{bold}}%s{{/}}", i.networkName)})
	return buf, tb
}

func ParseNodeIDs(cli client.Client, i *Info) error {
	i.nodeIDs = []ids.ShortID{}
	i.allNodeIDs = make([]ids.ShortID, len(nodeIDs))
	for idx, rnodeID := range nodeIDs {
		nodeID, err := ids.ShortFromPrefixedString(rnodeID, constants.NodeIDPrefix)
		if err != nil {
			return err
		}
		i.allNodeIDs[idx] = nodeID

		start, end, err := cli.P().GetValidator(i.subnetID, nodeID)
		i.valInfos[nodeID] = &ValInfo{start, end}
		switch {
		case errors.Is(err, client.ErrValidatorNotFound):
			i.nodeIDs = append(i.nodeIDs, nodeID)
		case err != nil:
			return err
		default:
			color.Outf("\n{{yellow}}%s is already a validator on subnet %s{{/}}", rnodeID, subnetIDs)
		}
	}
	return nil
}
