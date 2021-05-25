package proof_client

import (
	"context"
	"github.com/filecoin-project/venus-sealer/storage"
	types2 "github.com/filecoin-project/venus-sealer/types"
)

func StartProofEvent(ctx context.Context, prover storage.WinningPoStProver, client *ProofEventClient, mAddr types2.MinerAddress) error {
	proofEvent := &ProofEvent{prover: prover, client: client, mAddr: mAddr}
	go proofEvent.listenProofRequest(ctx)
	return nil
}