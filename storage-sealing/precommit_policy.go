package sealing

import (
	"context"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"

	"github.com/filecoin-project/venus-sealer/types"

	"github.com/filecoin-project/venus/pkg/types/specactors/builtin/miner"
	"github.com/filecoin-project/venus/pkg/types/specactors/policy"
)

type PreCommitPolicy interface {
	Expiration(ctx context.Context, ps ...types.Piece) (abi.ChainEpoch, error)
}

type Chain interface {
	ChainHead(ctx context.Context) (types.TipSetToken, abi.ChainEpoch, error)
	StateNetworkVersion(ctx context.Context, tok types.TipSetToken) (network.Version, error)
}

// BasicPreCommitPolicy satisfies PreCommitPolicy. It has two modes:
//
// Mode 1: The sector contains a non-zero quantity of pieces with deal info
// Mode 2: The sector contains no pieces with deal info
//
// The BasicPreCommitPolicy#Expiration method is given a slice of the pieces
// which the miner has encoded into the sector, and from that slice picks either
// the first or second mode.
//
// If we're in Mode 1: The pre-commit expiration epoch will be the maximum
// deal end epoch of a piece in the sector.
//
// If we're in Mode 2: The pre-commit expiration epoch will be set to the
// current epoch + the provided default duration.
type BasicPreCommitPolicy struct {
	api Chain

	provingBuffer    abi.ChainEpoch
	ccLifetimeEpochs abi.ChainEpoch
}

// NewBasicPreCommitPolicy produces a BasicPreCommitPolicy.
//
// The provided duration is used as the default sector expiry when the sector
// contains no deals. The proving boundary is used to adjust/align the sector's expiration.
func NewBasicPreCommitPolicy(api Chain, ccLifetimeEpochs abi.ChainEpoch, provingBuffer abi.ChainEpoch) BasicPreCommitPolicy {
	return BasicPreCommitPolicy{
		api:              api,
		ccLifetimeEpochs: ccLifetimeEpochs,
		provingBuffer:    provingBuffer,
	}
}

// Expiration produces the pre-commit sector expiration epoch for an encoded
// replica containing the provided enumeration of pieces and deals.
func (p *BasicPreCommitPolicy) Expiration(ctx context.Context, ps ...types.Piece) (abi.ChainEpoch, error) {
	_, epoch, err := p.api.ChainHead(ctx)
	if err != nil {
		return 0, err
	}

	var end *abi.ChainEpoch

	for _, p := range ps {
		if p.DealInfo == nil {
			continue
		}

		if p.DealInfo.DealSchedule.EndEpoch < epoch {
			log.Warnf("piece schedule %+v ended before current epoch %d", p, epoch)
			continue
		}

		if end == nil || *end < p.DealInfo.DealSchedule.EndEpoch {
			tmp := p.DealInfo.DealSchedule.EndEpoch
			end = &tmp
		}
	}

	if end == nil {
		// no deal pieces, get expiration for committed capacity sector
		expirationDuration, err := p.getCCSectorLifetime()
		if err != nil {
			return 0, err
		}

		tmp := epoch + expirationDuration
		end = &tmp
	}

	// Ensure there is at least one day for the PC message to land without falling below min sector lifetime
	// TODO: The "one day" should probably be a config, though it doesn't matter too much
	minExp := epoch + policy.GetMinSectorExpiration() + miner.WPoStProvingPeriod
	if *end < minExp {
		end = &minExp
	}

	return *end, nil
}

func (p *BasicPreCommitPolicy) getCCSectorLifetime() (abi.ChainEpoch, error) {
	// if zero value in config, assume maximum sector extension
	if p.ccLifetimeEpochs == 0 {
		p.ccLifetimeEpochs = policy.GetMaxSectorExpirationExtension()
	}

	if minExpiration := abi.ChainEpoch(miner.MinSectorExpiration); p.ccLifetimeEpochs < minExpiration {
		log.Warnf("value for CommittedCapacitySectorLiftime is too short, using default minimum (%d epochs)", minExpiration)
		return minExpiration, nil
	}
	if maxExpiration := policy.GetMaxSectorExpirationExtension(); p.ccLifetimeEpochs > maxExpiration {
		log.Warnf("value for CommittedCapacitySectorLiftime is too long, using default maximum (%d epochs)", maxExpiration)
		return maxExpiration, nil
	}

	return p.ccLifetimeEpochs - p.provingBuffer, nil
}
