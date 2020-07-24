// +build rosetta_rpc

package services

import (
	"context"
	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"

	"github.com/filecoin-project/lotus/api"
	filTypes "github.com/filecoin-project/lotus/chain/types"
)

const DummyHash = "0000000000000000000000000000000000000000"

// NetworkAPIService implements the server.NetworkAPIServicer interface.
type NetworkAPIService struct {
	network *types.NetworkIdentifier
	node api.FullNode
}

// NewNetworkAPIService creates a new instance of a NetworkAPIService.
func NewNetworkAPIService(network *types.NetworkIdentifier, node *api.FullNode) server.NetworkAPIServicer {
	return &NetworkAPIService{
		network: network,
		node: *node,
	}
}

// NetworkList implements the /network/list endpoint
func (s *NetworkAPIService) NetworkList(
	ctx context.Context,
	request *types.MetadataRequest,
) (*types.NetworkListResponse, *types.Error) {

	var blockchainName = "Filecoin" //TODO: Is this available on the api ?
	networkName, err := s.node.StateNetworkName(ctx)

	if err != nil {
		return nil, ErrUnableToGetChainID
	}

	resp := &types.NetworkListResponse{
		NetworkIdentifiers: []*types.NetworkIdentifier{
			{
				Blockchain: blockchainName,
				Network:    string(networkName),
			},
		},
	}

	return resp, nil
}

// NetworkStatus implements the /network/status endpoint.
func (s *NetworkAPIService) NetworkStatus(
	ctx context.Context,
	request *types.NetworkRequest,
) (*types.NetworkStatusResponse, *types.Error) {

	var (
		headTipSet   *filTypes.TipSet
		err          error
		useDummyHead = false
		blockIndex, timeStamp int64
		blockHashedTipSet string
	)

	//Check sync status
	status, syncErr := CheckSyncStatus(ctx, &s.node)
	if syncErr != nil {
		return nil, syncErr
	}
	if !status.IsSynced() {
		//Cannot retrieve any TipSet while node is syncing
		//use a dummy TipSet instead
		useDummyHead = true
	}

	//Get head TipSet
	headTipSet, err = s.node.ChainHead(ctx)

	if err != nil || headTipSet == nil {
		return nil, ErrUnableToGetLatestBlk
	}

	hashHeadTipSet, err := BuildTipSetKeyHash(headTipSet.Key())
	if err != nil {
		return nil, ErrUnableToBuildTipSetHash
	}

    //Get genesis TipSet
	genesisTipSet, err := s.node.ChainGetGenesis(ctx)
	if err != nil || genesisTipSet == nil {
		return nil, ErrUnableToGetGenesisBlk
	}

	hashGenesisTipSet, err := BuildTipSetKeyHash(genesisTipSet.Key())
	if err != nil {
		return nil, ErrUnableToBuildTipSetHash
	}

	//Get peers data
	peersFil, err := s.node.NetPeers(ctx)
	if err != nil {
		return nil, ErrUnableToGetPeers
	}

	var peers []*types.Peer
	for _, peerFil := range peersFil {
		peers = append(peers, &types.Peer{
			PeerID:   peerFil.ID.String(),
			Metadata: nil,
		})
	}

	if !useDummyHead {
		blockIndex = int64(headTipSet.Height())
		timeStamp = int64(headTipSet.MinTimestamp()) * FactorSecondToMillisecond
		blockHashedTipSet = *hashHeadTipSet
	} else {
		blockIndex = status.GetMaxHeight()
		timeStamp = 0
		blockHashedTipSet = DummyHash
	}

	resp := &types.NetworkStatusResponse{
		CurrentBlockIdentifier: &types.BlockIdentifier{
			Index: blockIndex,
			Hash:  blockHashedTipSet,
		},
		CurrentBlockTimestamp: timeStamp, // [ms]
		GenesisBlockIdentifier: &types.BlockIdentifier{
			Index: int64(genesisTipSet.Height()),
			Hash:  *hashGenesisTipSet,
		},
		Peers: peers,
	}

	return resp, nil
}

// NetworkOptions implements the /network/options endpoint. //TODO
func (s *NetworkAPIService) NetworkOptions(
	ctx context.Context,
	request *types.NetworkRequest,
) (*types.NetworkOptionsResponse, *types.Error) {

	version, err := s.node.Version(ctx)
	if err != nil {
		return nil, ErrUnableToGetNodeInfo
	}

	return &types.NetworkOptionsResponse{
		Version: &types.Version{
			RosettaVersion: "1.4.0", //TODO get this from an extern config.yml
			NodeVersion:    version.Version,
		},
		Allow: &types.Allow{
			OperationStatuses: []*types.OperationStatus{
				{
					Status:     "Success",
					Successful: true,
				},
				{
					Status:     "Reverted",
					Successful: false,
				},
			},
			OperationTypes: []string{
				"Transfer",
				"Reward",
			},
			Errors: ErrorList,
		},
	}, nil
}