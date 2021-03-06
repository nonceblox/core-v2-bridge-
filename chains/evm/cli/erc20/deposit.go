package erc20

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"

	"github.com/ChainSafe/chainbridge-core/chains/evm/calls"
	"github.com/ChainSafe/chainbridge-core/chains/evm/cli/flags"
	"github.com/ChainSafe/chainbridge-core/chains/evm/evmclient"
	"github.com/ChainSafe/chainbridge-core/chains/evm/evmtransaction"
	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	Recipient     string
	Bridge        string
	Amount        string
	DestinationID string
	ResourceID    string
	Decimals      int
)

var depositCmd = &cobra.Command{
	Use:   "deposit",
	Short: "Initiate a transfer of ERC20 tokens",
	Long:  "Initiate a transfer of ERC20 tokens",
	RunE: func(cmd *cobra.Command, args []string) error {
		txFabric := evmtransaction.NewTransaction
		return DepositCmd(cmd, args, txFabric)
	},
	Args: func(cmd *cobra.Command, args []string) error {
		err := validateFlags(cmd, args)
		if err != nil {
			return err
		}
		return nil
	},
}

func validateFlags(cmd *cobra.Command, args []string) error {
	if !common.IsHexAddress(Recipient) {
		return fmt.Errorf("invalid recipient address %s", Recipient)
	}
	if !common.IsHexAddress(Bridge) {
		return fmt.Errorf("invalid bridge address %s", Bridge)
	}
	return nil
}

func init() {
	depositCmd.Flags().StringVarP(&Recipient, "recipient", "r", "", "address of recipient")
	depositCmd.Flags().StringVarP(&Bridge, "bridge", "b", "", "address of bridge contract")
	depositCmd.Flags().StringVarP(&Amount, "amount", "a", "", "amount to deposit")
	depositCmd.Flags().StringVarP(&DestinationID, "destId", "did", "", "destination domain ID")
	depositCmd.Flags().StringVarP(&ResourceID, "resourceId", "rid", "", "resource ID for transfer")
	depositCmd.Flags().IntVarP(&Decimals, "decimals", "r", 0, "ERC20 token decimals")
	depositCmd.MarkFlagRequired("decimals")

}

func DepositCmd(cmd *cobra.Command, args []string, txFabric calls.TxFabric) error {
	recipientAddress := common.HexToAddress(Recipient)
	// fetch global flag values
	url, gasLimit, gasPrice, senderKeyPair, err := flags.GlobalFlagValues(cmd)
	if err != nil {
		return fmt.Errorf("could not get global flags: %v", err)
	}

	decimals := big.NewInt(int64(Decimals))

	bridgeAddr := common.HexToAddress(Bridge)

	realAmount, err := calls.UserAmountToWei(Amount, decimals)
	if err != nil {
		return err
	}

	ethClient, err := evmclient.NewEVMClientFromParams(url, senderKeyPair.PrivateKey(), gasPrice)
	if err != nil {
		log.Error().Err(fmt.Errorf("eth client intialization error: %v", err))
		return err
	}

	if ResourceID[0:2] == "0x" {
		ResourceID = ResourceID[2:]
	}
	resourceIdBytes, err := hex.DecodeString(ResourceID)
	if err != nil {
		return err
	}
	resourceIdBytesArr := calls.SliceTo32Bytes(resourceIdBytes)

	destinationIdInt, err := strconv.Atoi(DestinationID)
	if err != nil {
		log.Error().Err(fmt.Errorf("destination ID conversion error: %v", err))
		return err
	}
	data := calls.ConstructErc20DepositData(recipientAddress.Bytes(), realAmount)
	// TODO: confirm correct arguments
	input, err := calls.PrepareErc20DepositInput(uint8(destinationIdInt), resourceIdBytesArr, data)
	if err != nil {
		log.Error().Err(fmt.Errorf("erc20 deposit input error: %v", err))
		return err
	}

	blockNum, err := ethClient.BlockNumber(context.Background())
	if err != nil {
		log.Error().Err(fmt.Errorf("block fetch error: %v", err))
		return err
	}

	log.Debug().Msgf("blockNum: %v", blockNum)

	// destinationId
	txHash, err := calls.Transact(ethClient, txFabric, &bridgeAddr, input, gasLimit)
	if err != nil {
		log.Error().Err(fmt.Errorf("erc20 deposit error: %v", err))
		return err
	}

	log.Debug().Msgf("erc20 deposit hash: %s", txHash.Hex())

	log.Info().Msgf("%s tokens were transferred to %s from %s", Amount, recipientAddress.Hex(), senderKeyPair.CommonAddress().String())
	return nil
}
