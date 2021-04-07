package storiface

import (
	"context"
	"errors"
	"fmt"
	"github.com/filecoin-project/venus-sealer/types"
	"io"
	"time"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-storage/storage"
)

type WorkerInfo struct {
	Hostname string

	Resources WorkerResources
}

type WorkerResources struct {
	MemPhysical uint64
	MemSwap     uint64

	MemReserved uint64 // Used by system / other processes

	CPUs uint64 // Logical cores
	GPUs []string
}

type WorkerStats struct {
	Info    WorkerInfo
	Enabled bool

	MemUsedMin uint64
	MemUsedMax uint64
	GpuUsed    bool   // nolint
	CpuUse     uint64 // nolint
}

const (
	RWRetWait  = -1
	RWReturned = -2
	RWRetDone  = -3
)

type WorkerJob struct {
	ID     types.CallID
	Sector abi.SectorID
	Task   types.TaskType

	// 1+ - assigned
	// 0  - running
	// -1 - ret-wait
	// -2 - returned
	// -3 - ret-done
	RunWait int
	Start   time.Time

	Hostname string `json:",omitempty"` // optional, set for ret-wait jobs
}

type WorkerCalls interface {
	AddPiece(ctx context.Context, sector storage.SectorRef, pieceSizes []abi.UnpaddedPieceSize, newPieceSize abi.UnpaddedPieceSize, pieceData storage.Data) (types.CallID, error)
	SealPreCommit1(ctx context.Context, sector storage.SectorRef, ticket abi.SealRandomness, pieces []abi.PieceInfo) (types.CallID, error)
	SealPreCommit2(ctx context.Context, sector storage.SectorRef, pc1o storage.PreCommit1Out) (types.CallID, error)
	SealCommit1(ctx context.Context, sector storage.SectorRef, ticket abi.SealRandomness, seed abi.InteractiveSealRandomness, pieces []abi.PieceInfo, cids storage.SectorCids) (types.CallID, error)
	SealCommit2(ctx context.Context, sector storage.SectorRef, c1o storage.Commit1Out) (types.CallID, error)
	FinalizeSector(ctx context.Context, sector storage.SectorRef, keepUnsealed []storage.Range) (types.CallID, error)
	ReleaseUnsealed(ctx context.Context, sector storage.SectorRef, safeToFree []storage.Range) (types.CallID, error)
	MoveStorage(ctx context.Context, sector storage.SectorRef, types SectorFileType) (types.CallID, error)
	UnsealPiece(context.Context, storage.SectorRef, UnpaddedByteIndex, abi.UnpaddedPieceSize, abi.SealRandomness, cid.Cid) (types.CallID, error)
	ReadPiece(context.Context, io.Writer, storage.SectorRef, UnpaddedByteIndex, abi.UnpaddedPieceSize) (types.CallID, error)
	Fetch(context.Context, storage.SectorRef, SectorFileType, PathType, AcquireMode) (types.CallID, error)
}

type ErrorCode int

const (
	ErrUnknown ErrorCode = iota
)

const (
	// Temp Errors
	ErrTempUnknown ErrorCode = iota + 100
	ErrTempWorkerRestart
	ErrTempAllocateSpace
)

type CallError struct {
	Code    ErrorCode
	Message string
	sub     error
}

func (c *CallError) Error() string {
	return fmt.Sprintf("storage call error %d: %s", c.Code, c.Message)
}

func (c *CallError) Unwrap() error {
	if c.sub != nil {
		return c.sub
	}

	return errors.New(c.Message)
}

func Err(code ErrorCode, sub error) *CallError {
	return &CallError{
		Code:    code,
		Message: sub.Error(),

		sub: sub,
	}
}

type WorkerReturn interface {
	ReturnAddPiece(ctx context.Context, callID types.CallID, pi abi.PieceInfo, err *CallError) error
	ReturnSealPreCommit1(ctx context.Context, callID types.CallID, p1o storage.PreCommit1Out, err *CallError) error
	ReturnSealPreCommit2(ctx context.Context, callID types.CallID, sealed storage.SectorCids, err *CallError) error
	ReturnSealCommit1(ctx context.Context, callID types.CallID, out storage.Commit1Out, err *CallError) error
	ReturnSealCommit2(ctx context.Context, callID types.CallID, proof storage.Proof, err *CallError) error
	ReturnFinalizeSector(ctx context.Context, callID types.CallID, err *CallError) error
	ReturnReleaseUnsealed(ctx context.Context, callID types.CallID, err *CallError) error
	ReturnMoveStorage(ctx context.Context, callID types.CallID, err *CallError) error
	ReturnUnsealPiece(ctx context.Context, callID types.CallID, err *CallError) error
	ReturnReadPiece(ctx context.Context, callID types.CallID, ok bool, err *CallError) error
	ReturnFetch(ctx context.Context, callID types.CallID, err *CallError) error
}