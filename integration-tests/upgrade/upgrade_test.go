//go:build integrationtests

package upgrade

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/client/grpc/tmservice"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/coreum-tools/pkg/retry"
	appupgradev2 "github.com/CoreumFoundation/coreum/app/upgrade/v2"
	integrationtests "github.com/CoreumFoundation/coreum/integration-tests"
)

// TestUpgrade that after accepting upgrade proposal cosmovisor starts a new version of cored.
func TestUpgrade(t *testing.T) {
	// run upgrade v2
	upgradeV2(t)
}

func upgradeV2(t *testing.T) {
	runUpgrade(t, "v1.0.0", appupgradev2.Name, 30)
}

func runUpgrade(
	t *testing.T,
	oldBinaryVersion string,
	upgradeName string,
	blocksToWait int64,
) {
	ctx, chain := integrationtests.NewCoreumTestingContext(t, true)

	log := logger.Get(ctx)
	requireT := require.New(t)
	upgradeClient := upgradetypes.NewQueryClient(chain.ClientContext)

	// Verify that there is no ongoing upgrade plan.
	currentPlan, err := upgradeClient.CurrentPlan(ctx, &upgradetypes.QueryCurrentPlanRequest{})
	requireT.NoError(err)
	requireT.Nil(currentPlan.Plan)

	tmQueryClient := tmservice.NewServiceClient(chain.ClientContext)
	infoBeforeRes, err := tmQueryClient.GetNodeInfo(ctx, &tmservice.GetNodeInfoRequest{})
	requireT.NoError(err)
	// we start with the old binary version
	require.Equal(t, infoBeforeRes.ApplicationVersion.Version, oldBinaryVersion)

	latestBlockRes, err := tmQueryClient.GetLatestBlock(ctx, &tmservice.GetLatestBlockRequest{})
	requireT.NoError(err)

	upgradeHeight := latestBlockRes.Block.Header.Height + blocksToWait

	// Create new proposer.
	proposer := chain.GenAccount()
	proposerBalance, err := chain.Governance.ComputeProposerBalance(ctx)
	requireT.NoError(err)

	err = chain.Faucet.FundAccounts(ctx, integrationtests.NewFundedAccount(proposer, proposerBalance))
	requireT.NoError(err)

	log.Info("Creating proposal for upgrading",
		zap.String("upgradeName", upgradeName),
		zap.Int64("upgradeHeight", upgradeHeight),
	)

	// Create proposal to upgrade chain.
	proposalMsg, err := chain.Governance.NewMsgSubmitProposal(
		ctx,
		proposer,
		upgradetypes.NewSoftwareUpgradeProposal(
			"Upgrade "+upgradeName,
			"Running "+upgradeName+" in integration tests",
			upgradetypes.Plan{
				Name:   upgradeName,
				Height: upgradeHeight,
			},
		))
	requireT.NoError(err)
	proposalID, err := chain.Governance.Propose(ctx, proposalMsg)
	requireT.NoError(err)
	log.Info("Upgrade proposal has been submitted", zap.Uint64("proposalID", proposalID))

	// Verify that voting period started.
	proposal, err := chain.Governance.GetProposal(ctx, proposalID)
	requireT.NoError(err)
	requireT.Equal(govtypes.StatusVotingPeriod, proposal.Status)

	// Vote yes from all vote accounts.
	err = chain.Governance.VoteAll(ctx, govtypes.OptionYes, proposal.ProposalId)
	requireT.NoError(err)

	log.Info("Voters have voted successfully, waiting for voting period to be finished", zap.Time("votingEndTime", proposal.VotingEndTime))

	// Wait for proposal result.
	finalStatus, err := chain.Governance.WaitForVotingToFinalize(ctx, proposalID)
	requireT.NoError(err)
	requireT.Equal(govtypes.StatusPassed, finalStatus)

	// Verify that upgrade plan is there waiting to be applied.
	currentPlan, err = upgradeClient.CurrentPlan(ctx, &upgradetypes.QueryCurrentPlanRequest{})
	requireT.NoError(err)
	requireT.NotNil(currentPlan.Plan)
	assert.Equal(t, upgradeName, currentPlan.Plan.Name)
	assert.Equal(t, upgradeHeight, currentPlan.Plan.Height)

	// Verify that we are before the upgrade
	infoWaitingBlockRes, err := tmQueryClient.GetLatestBlock(ctx, &tmservice.GetLatestBlockRequest{})
	requireT.NoError(err)
	requireT.Less(infoWaitingBlockRes.Block.Header.Height, upgradeHeight)

	retryCtx, cancel := context.WithTimeout(ctx, 6*time.Second*time.Duration(upgradeHeight-infoWaitingBlockRes.Block.Header.Height))
	defer cancel()
	log.Info("Waiting for upgrade", zap.Int64("upgradeHeight", upgradeHeight), zap.Int64("currentHeight", infoWaitingBlockRes.Block.Header.Height))
	err = retry.Do(retryCtx, time.Second, func() error {
		requestCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		var err error
		infoAfterBlockRes, err := tmQueryClient.GetLatestBlock(requestCtx, &tmservice.GetLatestBlockRequest{})
		if err != nil {
			return retry.Retryable(err)
		}
		if infoAfterBlockRes.Block.Header.Height >= upgradeHeight+1 {
			return nil
		}
		return retry.Retryable(errors.Errorf("waiting for upgraded block %d, current block: %d", upgradeHeight, infoAfterBlockRes.Block.Header.Height))
	})
	requireT.NoError(err)

	// Verify that upgrade was applied on chain.
	appliedPlan, err := upgradeClient.AppliedPlan(ctx, &upgradetypes.QueryAppliedPlanRequest{
		Name: upgradeName,
	})
	requireT.NoError(err)
	assert.Equal(t, upgradeHeight, appliedPlan.Height)
	log.Info(fmt.Sprintf("Upgrade passed, applied plan height: %d", appliedPlan.Height))

	// The new binary isn't equal to initial
	infoAfterRes, err := tmQueryClient.GetNodeInfo(ctx, &tmservice.GetNodeInfoRequest{})
	requireT.NoError(err)
	log.Info(fmt.Sprintf("New binary version: %s", infoAfterRes.ApplicationVersion.Version))
	assert.NotEqual(t, infoAfterRes.ApplicationVersion.Version, infoBeforeRes.ApplicationVersion.Version)
}
