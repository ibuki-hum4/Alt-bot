package service

import (
	"context"
	"fmt"

	"alt-bot/ent"
	"alt-bot/ent/chainstate"
	"alt-bot/ent/transactionlog"
)

// chainStateSingletonID is the fixed primary key of the one chain_state row,
// mirroring marketStateSingletonID.
const chainStateSingletonID = 1

// ensureChainState loads the singleton chain_state row, creating it on first
// run. When creating it, latest_hash is seeded from the most recent
// TransactionLog row: a database that already has transaction history must
// keep chaining from that history, otherwise the first append after this
// change would write prev_hash="" and silently fork the chain.
func (s *EconomyService) ensureChainState(ctx context.Context) (*ent.ChainState, error) {
	state, err := s.client.ChainState.Get(ctx, chainStateSingletonID)
	if err == nil {
		return state, nil
	}
	if !ent.IsNotFound(err) {
		return nil, fmt.Errorf("failed to load chain_state: %w", err)
	}

	seed := ""
	last, qerr := s.client.TransactionLog.Query().
		Order(ent.Desc(transactionlog.FieldCreatedAt)).
		First(ctx)
	if qerr == nil {
		seed = last.Hash
	} else if !ent.IsNotFound(qerr) {
		s.logger.Warn().Err(qerr).Msg("failed to load latest transaction hash; starting new chain")
	}

	created, createErr := s.client.ChainState.Create().
		SetID(chainStateSingletonID).
		SetLatestHash(seed).
		Save(ctx)
	if createErr != nil {
		if ent.IsConstraintError(createErr) {
			return s.client.ChainState.Get(ctx, chainStateSingletonID)
		}
		return nil, fmt.Errorf("failed to create chain_state: %w", createErr)
	}
	s.logger.Info().Str("seed_hash", seed).Msg("chain state initialized")
	return created, nil
}

// lockChainStateTx locks the singleton chain_state row for update inside tx,
// lazily creating it (seeded like ensureChainState) when missing. Callers
// must write the new tip back with saveChainStateTx before committing.
//
// Lock ordering: chain_state is always the LAST lock a transaction takes.
// Callers must already hold market_state (when they touch it) and any user
// rows (sorted by discord ID when there are several), and must not lock
// either afterwards.
func (s *EconomyService) lockChainStateTx(ctx context.Context, tx *ent.Tx) (*ent.ChainState, error) {
	state, err := tx.ChainState.Query().
		Where(chainstate.IDEQ(chainStateSingletonID)).
		ForUpdate().
		Only(ctx)
	if err == nil {
		return state, nil
	}
	if !ent.IsNotFound(err) {
		return nil, fmt.Errorf("failed to lock chain_state: %w", err)
	}

	seed := ""
	last, lastErr := tx.TransactionLog.Query().
		Order(ent.Desc(transactionlog.FieldCreatedAt)).
		First(ctx)
	if lastErr == nil {
		seed = last.Hash
	} else if !ent.IsNotFound(lastErr) {
		return nil, fmt.Errorf("failed to load latest transaction hash in tx: %w", lastErr)
	}

	if _, createErr := tx.ChainState.Create().
		SetID(chainStateSingletonID).
		SetLatestHash(seed).
		Save(ctx); createErr != nil && !ent.IsConstraintError(createErr) {
		return nil, fmt.Errorf("failed to create chain_state in tx: %w", createErr)
	}

	state, err = tx.ChainState.Query().
		Where(chainstate.IDEQ(chainStateSingletonID)).
		ForUpdate().
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to re-lock chain_state: %w", err)
	}
	return state, nil
}

// saveChainStateTx persists the new chain tip. cs must come from
// lockChainStateTx earlier in the same transaction.
func (s *EconomyService) saveChainStateTx(ctx context.Context, tx *ent.Tx, cs *ent.ChainState, newHash string) error {
	if _, err := tx.ChainState.UpdateOneID(cs.ID).
		SetLatestHash(newHash).
		Save(ctx); err != nil {
		return fmt.Errorf("failed to save chain_state: %w", err)
	}
	return nil
}

// appendSignedLogChain locks chain_state, appends every input in order so
// that each entry chains onto the previous one (the first chains onto the
// current tip), writes the resulting tip back once, and returns it.
//
// Use this rather than calling appendSignedLog repeatedly when one operation
// records several entries — RobUser records both sides of a robbery — so the
// entries chain correctly and chain_state is touched a single time.
func (s *EconomyService) appendSignedLogChain(ctx context.Context, tx *ent.Tx, ins ...txLogInput) (string, error) {
	if len(ins) == 0 {
		return "", nil
	}

	cs, err := s.lockChainStateTx(ctx, tx)
	if err != nil {
		return "", err
	}

	hash := cs.LatestHash
	for _, in := range ins {
		hash, err = s.appendSignedLogWithPrevHash(ctx, tx, hash, in)
		if err != nil {
			return "", err
		}
	}

	if err := s.saveChainStateTx(ctx, tx, cs, hash); err != nil {
		return "", err
	}
	return hash, nil
}
