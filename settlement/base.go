package settlement

import (
	"context"
	"fmt"

	"github.com/dymensionxyz/dymint/da"
	"github.com/dymensionxyz/dymint/log"
	"github.com/dymensionxyz/dymint/types"
	"github.com/tendermint/tendermint/libs/pubsub"
)

// BaseLayerClient is intended only for usage in tests.
type BaseLayerClient struct {
	logger         log.Logger
	pubsub         *pubsub.Server
	sequencersList []*types.Sequencer
	config         Config
	ctx            context.Context
	cancel         context.CancelFunc
	client         HubClient
}

var _ LayerI = &BaseLayerClient{}

// WithHubClient is an option which sets the hub client.
func WithHubClient(hubClient HubClient) Option {
	return func(settlementClient LayerI) {
		settlementClient.(*BaseLayerClient).client = hubClient
	}
}

// Init is called once. it initializes the struct members.
func (b *BaseLayerClient) Init(config Config, pubsub *pubsub.Server, logger log.Logger, options ...Option) error {
	var err error
	b.config = config
	b.pubsub = pubsub
	b.logger = logger
	b.ctx, b.cancel = context.WithCancel(context.Background())
	// Apply options
	for _, apply := range options {
		apply(b)
	}

	b.sequencersList, err = b.fetchSequencersList()
	if err != nil {
		return err
	}
	b.logger.Info("Updated sequencers list from settlement layer", "sequencersList", b.sequencersList)

	return nil
}

// Start is called once, after init. It initializes the query client.
func (b *BaseLayerClient) Start() error {
	b.logger.Debug("settlement Layer Client starting.")

	err := b.client.Start()
	if err != nil {
		return err
	}

	return nil
}

// Stop is called once, after Start.
func (b *BaseLayerClient) Stop() error {
	b.logger.Info("Settlement Layer Client stopping")
	err := b.client.Stop()
	if err != nil {
		return err
	}
	b.cancel()
	return nil
}

// SubmitBatch tries submitting the batch in an async broadcast mode to the settlement layer. Events are emitted on success or failure.
func (b *BaseLayerClient) SubmitBatch(batch *types.Batch, daClient da.Client, daResult *da.ResultSubmitBatch) error {
	b.logger.Debug("Submitting batch to settlement layer", "start height", batch.StartHeight, "end height", batch.EndHeight)
	return b.client.PostBatch(batch, daClient, daResult)
}

// RetrieveBatch Gets the batch which contains the given slHeight. Empty slHeight returns the latest batch.
func (b *BaseLayerClient) RetrieveBatch(stateIndex ...uint64) (*ResultRetrieveBatch, error) {
	var resultRetrieveBatch *ResultRetrieveBatch
	var err error
	if len(stateIndex) == 0 {
		b.logger.Debug("Getting latest batch from settlement layer")
		resultRetrieveBatch, err = b.client.GetLatestBatch(b.config.RollappID)
		if err != nil {
			return nil, err
		}
	} else if len(stateIndex) == 1 {
		b.logger.Debug("Getting batch from settlement layer", "state index", stateIndex)
		resultRetrieveBatch, err = b.client.GetBatchAtIndex(b.config.RollappID, stateIndex[0])
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("invalid number of arguments. Expected 0 or 1, got %d", len(stateIndex))
	}
	return resultRetrieveBatch, nil
}

// GetSequencersList returns the current list of sequencers from the settlement layer
func (b *BaseLayerClient) GetSequencersList() []*types.Sequencer {
	return b.sequencersList
}

// GetProposer returns the sequencer which is currently the proposer
func (b *BaseLayerClient) GetProposer() *types.Sequencer {
	for _, sequencer := range b.sequencersList {
		if sequencer.Status == types.Proposer {
			return sequencer
		}
	}
	return nil
}

func (b *BaseLayerClient) fetchSequencersList() ([]*types.Sequencer, error) {
	sequencers, err := b.client.GetSequencers(b.config.RollappID)
	if err != nil {
		return nil, err
	}
	return sequencers, nil
}
