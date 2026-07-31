package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/uerr"
)

// Dispatcher drains one account's queued outbox intents against a
// backend, one DispatchOnce pass at a time.
type Dispatcher struct {
	accountID int64
	backend   backend.Backend
	writer    *store.Writer
}

// NewDispatcher returns a Dispatcher draining accountID's intents
// against be, reaching the store through w.
func NewDispatcher(accountID int64, be backend.Backend, w *store.Writer) *Dispatcher {
	return &Dispatcher{accountID: accountID, backend: be, writer: w}
}

// Delivered is one intent DispatchOnce dispatched successfully. Move
// carries the resolved payload for KindMoveMessages, PriorMailboxIDs
// included, so a caller can build the exact compensating move without
// reading the store again once this intent's row is gone.
type Delivered struct {
	IntentID  int64
	UndoGroup string
	Kind      Kind
	Move      *MoveMessagesPayload
}

// Failure is one intent DispatchOnce could not deliver. Retrying is
// true when the row stays queued for another attempt: every class but
// ClassNotFound, whose referent retrying cannot fix, and short of the
// failures no later attempt can answer differently. Warn is SY-5's
// distinction for ClassThrottled, which a caller renders as a warn
// state and never an error toast.
type Failure struct {
	IntentID  int64
	UndoGroup string
	Class     uerr.Class
	Detail    string
	Retrying  bool
	Warn      bool
}

// Result is one DispatchOnce pass's outcome.
type Result struct {
	Delivered []Delivered
	Failures  []Failure
}

// claimed is one intent claimed for this pass, with everything
// selectEligible's read phase could resolve from the store before any
// backend I/O.
type claimed struct {
	row
	messageServerIDs map[int64]string
	destServerID     string
	mailboxServerID  string
	parentServerID   string
	preFailed        bool
	class            uerr.Class
	detail           string
}

// outcome is one claimed intent's disposition after its backend call:
// delivered, with move carrying the decoded payload for
// KindMoveMessages, or failed with a class and detail. final marks a
// failure no later attempt may repeat whatever its class says.
type outcome struct {
	c      claimed
	move   *MoveMessagesPayload
	failed bool
	final  bool
	class  uerr.Class
	detail string
}

// finalizeAction is claim's disposition once DispatchOnce decides it,
// applied inside finalize's one writer transaction.
type finalizeAction struct {
	id      int64
	attempt int
	verb    finalizeVerb
	class   string
	detail  string
}

// apply writes a's disposition inside tx.
func (a finalizeAction) apply(tx *sql.Tx, now time.Time) error {
	switch a.verb {
	case finalizeDelete:
		return deleteRow(tx, a.id)
	case finalizeRevert:
		return revertRow(tx, a.id)
	case finalizeRequeue:
		return requeueRow(tx, a.id, now.Add(backoff(a.attempt)), a.class, a.detail)
	}
	return nil
}

// revert returns a's row to queued after the finalize transaction
// failed. A requeue replays as the requeue it was rather than a bare
// state write: a row put back with its expired next_attempt_at and its
// attempt count unchanged is eligible again immediately and grows no
// backoff, so a finalize failure that persists spends every pass on
// another attempt against the live backend. That replay is best
// effort. The recovery is one transaction over every claimed row, so a
// store failure specific to the requeue would otherwise cost every
// other row in it the revert, and reaching queued is worth more to a
// row than its backoff is.
func (a finalizeAction) revert(tx *sql.Tx, now time.Time) error {
	if a.verb == finalizeRequeue {
		err := a.apply(tx, now)
		if err == nil {
			return nil
		}
		slog.Warn("outbox: recovery reverted an intent without its backoff", "id", a.id, "requeue_error", err)
	}
	return revertRow(tx, a.id)
}

type finalizeVerb int

const (
	finalizeDelete finalizeVerb = iota
	finalizeRequeue
	finalizeRevert
)

// DispatchOnce claims every eligible intent and dispatches it in id
// order, stopping early on a connection failure (further calls
// against a dead connection cannot succeed) and reverting whatever it
// claimed but never attempted back to queued untouched. A create whose
// dependent moves were claimed in the same pass dispatches with them
// as one batch.
func (d *Dispatcher) DispatchOnce(ctx context.Context, now time.Time) (Result, error) {
	claimedRows, err := d.claim(ctx, now)
	if err != nil {
		return Result{}, err
	}

	var result Result
	var actions []finalizeAction
	resolvedCreates := map[int64]string{}
	stopped := false

	groups := planBatches(claimedRows, chunkSize(d.backend))

	// A row a create's batch already decided has its action recorded
	// here, so its own turn in the loop passes over it. One record of
	// what has been decided covers both directions: a move whose create
	// dispatched keeps that decision whether or not the pass later
	// stopped, and a move whose create never ran takes the revert every
	// other abandoned claim takes.
	finalized := map[int64]bool{}
	finalize := func(a finalizeAction) {
		actions = append(actions, a)
		finalized[a.id] = true
	}

	for _, c := range claimedRows {
		if finalized[c.id] {
			continue
		}
		if stopped {
			finalize(finalizeAction{id: c.id, verb: finalizeRevert})
			continue
		}

		var outcomes []outcome
		switch {
		case c.preFailed:
			outcomes = []outcome{{c: c, failed: true, class: c.class, detail: c.detail}}
		case len(groups[c.id]) > 0:
			outcomes = d.dispatchBatch(ctx, c, groups[c.id], resolvedCreates)
		default:
			outcomes = []outcome{d.dispatchOne(ctx, c, resolvedCreates)}
		}

		for _, o := range outcomes {
			if o.failed {
				finalize(d.report(&result, o))
				stopped = stopped || o.class == uerr.ClassConnection
				continue
			}
			result.Delivered = append(result.Delivered, delivered(o.c, o.move))
			finalize(finalizeAction{id: o.c.id, verb: finalizeDelete})
		}
	}

	if err := d.finalize(ctx, actions, now); err != nil {
		return Result{}, err
	}
	return result, nil
}

// finalizeRecoveryTimeout bounds the recovery transaction below. It
// runs uncancelled, so it carries a bound of its own rather than
// waiting on the writer for as long as the writer takes.
const finalizeRecoveryTimeout = 5 * time.Second

// finalize applies every claimed row's disposition in one transaction.
// A failure there leaves each of those rows in dispatching, which
// selectEligible never selects again and only ReclaimOrphaned's
// startup sweep can undo, so finalize returns them to queued in a
// second transaction rather than costing the queue every claimed
// intent for the rest of the run. That transaction writes each row's
// state once, replaying a requeue's backoff where the store lets it and
// falling back to the bare state write where it does not. Replay is
// safe for every row it touches, delivered ones included: their
// backend calls are idempotent, and a create whose resolved id never
// reached its payload replays into a refusal the dispatcher reconciles
// against the mailbox already there. DispatchOnce reports no Result
// when this fails, since none of the writes such a Result would
// describe landed.
//
// The recovery drops ctx, since a deadline expiring during the backend
// calls is the ordinary way the transaction above fails and the same
// context would fail the recovery for the identical reason.
func (d *Dispatcher) finalize(ctx context.Context, actions []finalizeAction, now time.Time) error {
	err := d.writer.ApplyInteractive(ctx, func(tx *sql.Tx) error {
		for _, a := range actions {
			if err := a.apply(tx, now); err != nil {
				return err
			}
		}
		return nil
	})
	if err == nil {
		return nil
	}

	recoveryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), finalizeRecoveryTimeout)
	defer cancel()

	revertErr := d.writer.ApplyInteractive(recoveryCtx, func(tx *sql.Tx) error {
		for _, a := range actions {
			if err := a.revert(tx, now); err != nil {
				return err
			}
		}
		return nil
	})
	if revertErr != nil {
		slog.Error("outbox: claimed intents stranded in dispatching",
			"ids", actionIDs(actions), "finalize_error", err, "revert_error", revertErr)
		return errors.Join(err, revertErr)
	}
	return err
}

// actionIDs returns the intent id of every action, for the log line a
// failed revert leaves behind: those rows are unreachable until the
// next startup sweep, and this is the only record of which ones.
func actionIDs(actions []finalizeAction) []int64 {
	ids := make([]int64, len(actions))
	for i, a := range actions {
		ids[i] = a.id
	}
	return ids
}

// dispatchOne executes c on its own, one backend call, recording a
// create's resolved server id in resolvedCreates so a dependent
// dispatching later in this same pass can substitute it.
func (d *Dispatcher) dispatchOne(ctx context.Context, c claimed, resolvedCreates map[int64]string) outcome {
	o := outcome{c: c}
	switch c.kind {
	case KindCreateMailbox:
		return d.dispatchCreateMailbox(ctx, c, resolvedCreates)
	case KindRenameMailbox:
		o.failed, o.class, o.detail = d.dispatchRenameMailbox(ctx, c)
	case KindDeleteMailbox:
		o.failed, o.class, o.detail = d.dispatchDeleteMailbox(ctx, c)
	case KindMoveMessages:
		o.move, o.failed, o.class, o.detail = d.dispatchMoveMessages(ctx, c, resolvedCreates)
	default:
		// A kind this build does not recognize (a newer poplar wrote
		// it, or the schema grew one this dispatcher has not caught up
		// with) fails this one intent rather than aborting the pass:
		// every other claimed row still needs its finalize action. It
		// is final because no later pass of this build has a case for
		// it either, so a retriable classification would retry the row
		// every 30 seconds for the life of the store. No server was
		// involved, so the class is a local one.
		o.failed, o.final, o.class = true, true, uerr.ClassStoreLocal
		o.detail = fmt.Sprintf("unknown kind %q", c.kind)
	}
	return o
}

// planBatches groups each claimed create-mailbox intent with the
// claimed moves whose destination is the mailbox it creates, so an
// offline create-folder-then-move reaches the server as one batch and
// the server resolves the back-reference itself. limit is the
// backend's per-batch object bound: the create spends one of it, and a
// move chunk that would push the batch past the rest stays out and
// dispatches on its own against the server id the create persists into
// its payload. A create replaying with its server id already resolved
// takes no batch, since it has no create mutation left to make.
func planBatches(rows []claimed, limit int) map[int64][]claimed {
	budget := map[int64]int{}
	for _, c := range rows {
		if c.kind != KindCreateMailbox || c.preFailed {
			continue
		}
		var p CreateMailboxPayload
		if err := json.Unmarshal(c.payload, &p); err != nil || p.ResolvedServerID != "" {
			continue
		}
		budget[c.id] = limit - 1
	}

	var groups map[int64][]claimed
	for _, c := range rows {
		if c.kind != KindMoveMessages || c.preFailed {
			continue
		}
		var p MoveMessagesPayload
		if err := json.Unmarshal(c.payload, &p); err != nil || p.DestRef == 0 {
			continue
		}
		left, ok := budget[p.DestRef]
		if !ok || len(p.MessageIDs) > left {
			continue
		}
		budget[p.DestRef] = left - len(p.MessageIDs)
		if groups == nil {
			groups = map[int64][]claimed{}
		}
		groups[p.DestRef] = append(groups[p.DestRef], c)
	}
	return groups
}

// dispatchBatch executes create and the moves destined for the mailbox
// it creates as one ApplyBatch request: the create carries a creation
// id and every message update names the mailbox by that id's
// back-reference, so the server assigns the mailbox and files the
// messages into it in one round trip. Nothing has reached the server
// until that call, so anything failing before it fails the whole
// group; afterwards each move stands or falls on its own messages.
func (d *Dispatcher) dispatchBatch(ctx context.Context, create claimed, moves []claimed, resolvedCreates map[int64]string) []outcome {
	var p CreateMailboxPayload
	if err := json.Unmarshal(create.payload, &p); err != nil {
		return failBatch(create, moves, uerr.ClassServer, err.Error())
	}
	parentID := create.parentServerID
	if p.ParentRef != 0 && parentID == "" {
		parentID = resolvedCreates[p.ParentRef]
	}

	creationID := "c" + strconv.FormatInt(create.id, 10)
	mutations := []backend.Mutation{{
		Op:         backend.MutationCreate,
		Kind:       backend.ObjectKindMailbox,
		CreationID: creationID,
		Fields:     backend.MailboxCreate{Name: p.Name, ParentID: parentID},
	}}
	payloads := make([]*MoveMessagesPayload, len(moves))
	for i, mv := range moves {
		var mp MoveMessagesPayload
		if err := json.Unmarshal(mv.payload, &mp); err != nil {
			return failBatch(create, moves, uerr.ClassServer, err.Error())
		}
		payloads[i] = &mp
		mutations = append(mutations, fileIntoMailbox(mv, mp.MessageIDs, "#"+creationID)...)
	}

	res, err := d.backend.Mail().ApplyBatch(ctx, mutations)
	if err != nil {
		class, detail := classifyFailure(err)
		return failBatch(create, moves, class, detail)
	}

	var newID string
	if mutErr, bad := res.Failed[creationID]; bad {
		if !errors.Is(mutErr, backend.ErrMailboxNameExists) {
			class, detail := classifyFailure(mutErr)
			return failBatch(create, moves, class, detail)
		}
		// The refusal took the whole batch with it: every message
		// update named the mailbox by a creation id the server never
		// resolved. Once the reconcile has the id, the moves are made
		// again against it in a second request, so each one still
		// stands or falls on its own messages below.
		newID, err = d.adoptMailbox(ctx, create.id, p.Name, parentID)
		if err != nil {
			return failBatchAdoption(create, moves, err)
		}
		res, err = d.refile(ctx, moves, payloads, newID)
		if err != nil {
			class, detail := classifyFailure(err)
			return failBatch(create, moves, class, detail)
		}
	} else {
		created, ok := res.Created[creationID]
		if !ok {
			return failBatch(create, moves, uerr.ClassServer, "mailbox create returned no server id")
		}
		newID = created
	}
	resolvedCreates[create.id] = newID

	p.ResolvedServerID = newID
	payload, err := json.Marshal(p)
	if err != nil {
		return failBatch(create, moves, uerr.ClassStoreLocal, err.Error())
	}
	if err := d.persistPayload(ctx, create.id, payload); err != nil {
		return failBatch(create, moves, uerr.ClassStoreLocal, err.Error())
	}
	if err := d.resolveDependentRefs(ctx, create.id, newID); err != nil {
		return failBatch(create, moves, uerr.ClassStoreLocal, err.Error())
	}

	outcomes := make([]outcome, 0, len(moves)+1)
	outcomes = append(outcomes, outcome{c: create})
	for i, mv := range moves {
		outcomes = append(outcomes, moveOutcome(mv, payloads[i], res))
	}
	return outcomes
}

// fileIntoMailbox returns one update mutation per message id, each
// naming dest as the message's only mailbox: a resolved server id for
// a move dispatching on its own, or a "#creationID" back-reference for
// one riding in the same batch as the create that makes the mailbox.
func fileIntoMailbox(c claimed, messageIDs []int64, dest string) []backend.Mutation {
	mutations := make([]backend.Mutation, 0, len(messageIDs))
	for _, msgID := range messageIDs {
		mutations = append(mutations, backend.Mutation{
			Op:     backend.MutationUpdate,
			Kind:   backend.ObjectKindMessage,
			ID:     c.messageServerIDs[msgID],
			Fields: backend.MessagePatch{MailboxIDs: []string{dest}},
		})
	}
	return mutations
}

// moveOutcome reads mv's disposition out of a batch result: failed
// with the first of its messages the server rejected, delivered
// otherwise.
func moveOutcome(mv claimed, p *MoveMessagesPayload, res backend.BatchResult) outcome {
	for _, msgID := range p.MessageIDs {
		mutErr, bad := res.Failed[mv.messageServerIDs[msgID]]
		if !bad {
			continue
		}
		class, detail := classifyFailure(mutErr)
		return outcome{c: mv, failed: true, class: class, detail: detail}
	}
	return outcome{c: mv, move: p}
}

// failBatch fails every intent in a batch with one class and detail:
// the moves exist to file messages into the mailbox the create makes,
// so none of them can stand without it.
func failBatch(create claimed, moves []claimed, class uerr.Class, detail string) []outcome {
	outcomes := make([]outcome, 0, len(moves)+1)
	outcomes = append(outcomes, failedOutcome(create, class, detail))
	for _, mv := range moves {
		outcomes = append(outcomes, failedOutcome(mv, class, detail))
	}
	return outcomes
}

// failBatchAdoption is failBatch for a batch whose create the
// reconcile could not resolve against a single mailbox. It is final
// for every intent in it: the refusal repeats on every attempt, and a
// move whose destination is that create has nothing to resolve without
// it. A lookup call that failed on its own keeps its class's ordinary
// retry, the same distinction adoptionFailure draws.
func failBatchAdoption(create claimed, moves []claimed, err error) []outcome {
	class, detail := classifyFailure(err)
	outcomes := failBatch(create, moves, class, detail)
	if !errors.Is(err, errAmbiguousMailbox) {
		return outcomes
	}
	for i := range outcomes {
		outcomes[i].final = true
	}
	return outcomes
}

// refile files each move's messages into dest as one more ApplyBatch
// request. dispatchBatch's own request named the mailbox by a creation
// id, so a create the server refuses for a mailbox it already holds
// leaves those updates unresolved and they have to be made again
// against the id the reconcile found.
func (d *Dispatcher) refile(ctx context.Context, moves []claimed, payloads []*MoveMessagesPayload, dest string) (backend.BatchResult, error) {
	var mutations []backend.Mutation
	for i, mv := range moves {
		mutations = append(mutations, fileIntoMailbox(mv, payloads[i].MessageIDs, dest)...)
	}
	return d.backend.Mail().ApplyBatch(ctx, mutations)
}

// report appends o's classified failure to result, logs it through
// uerr's one seam when shouldLogFailure says it is new, and returns
// its finalize action: requeue with backoff if the failure is worth
// retrying, delete (give up and let the caller's report stand)
// otherwise.
func (d *Dispatcher) report(result *Result, o outcome) finalizeAction {
	c := o.c
	retry := isRetriable(o.class) && !o.final
	if shouldLogFailure(c.failureClass, o.class, retry) {
		_ = uerr.New("outbox.dispatch", failureIDs(c), o.class, errors.New(o.detail))
	}
	result.Failures = append(result.Failures, Failure{
		IntentID: c.id, UndoGroup: c.undoGroup, Class: o.class, Detail: o.detail, Retrying: retry, Warn: isWarn(o.class),
	})
	if !retry {
		return finalizeAction{id: c.id, verb: finalizeDelete}
	}
	return finalizeAction{id: c.id, attempt: c.attemptCount, verb: finalizeRequeue, class: o.class.String(), detail: o.detail}
}

// shouldLogFailure reports whether a failure classified class, with
// retry as isRetriable(class), is worth a fresh uerr.New call:
// lastClass is the class this row's failure_class column carried
// after its last attempt, empty if it never failed before. A
// retriable failure logs only on its first occurrence or a class
// change from lastClass (ADR-0013 revision 2: construction is the
// surfacing event, not the retry); an unretriable failure always
// logs, since its row is deleted once this pass finalizes and this is
// its only chance to reach the log.
func shouldLogFailure(lastClass string, class uerr.Class, retry bool) bool {
	return !retry || lastClass != class.String()
}

// failureIDs returns the server ids c's failure correlates against,
// for uerr's redaction-safe IDs field: whichever entities c's kind
// resolved before the failure, or nil when nothing resolved yet.
func failureIDs(c claimed) []string {
	switch c.kind {
	case KindMoveMessages:
		ids := make([]string, 0, len(c.messageServerIDs))
		for _, id := range c.messageServerIDs {
			ids = append(ids, id)
		}
		return ids
	case KindRenameMailbox, KindDeleteMailbox:
		if c.mailboxServerID == "" {
			return nil
		}
		return []string{c.mailboxServerID}
	default:
		return nil
	}
}

// delivered builds c's Delivered entry, attaching move (already
// decoded by dispatchMoveMessages) for KindMoveMessages.
func delivered(c claimed, move *MoveMessagesPayload) Delivered {
	return Delivered{IntentID: c.id, UndoGroup: c.undoGroup, Kind: c.kind, Move: move}
}

// claim selects accountID's dispatch-eligible rows and moves each to
// dispatching inside one writer transaction, resolving every
// store-known server id (message and mailbox reads, no backend I/O)
// before the transaction commits: ADR-0006 revision 2's claim
// discipline, queued to dispatching before any I/O, in the same
// transaction undo's annihilation checks against.
func (d *Dispatcher) claim(ctx context.Context, now time.Time) ([]claimed, error) {
	var out []claimed
	err := d.writer.ApplyInteractive(ctx, func(tx *sql.Tx) error {
		rows, err := selectEligible(tx, d.accountID, now, claimLimit(d.backend))
		if err != nil {
			return err
		}
		for _, r := range rows {
			if err := claimRow(tx, r.id); err != nil {
				return err
			}
			c, err := resolveClaim(tx, r)
			if err != nil {
				return err
			}
			out = append(out, c)
		}
		return nil
	})
	return out, err
}

// claimMessageBudget and claimRowBudget bound what one claim
// transaction costs. It runs one message point query per message a
// claimed move chunk names, plus a fixed cost for each row it claims,
// all on the writer's single connection. Measured against a migrated
// store, a message lookup costs roughly 20 microseconds and a row
// roughly 70, so both budgets are around 20ms of work, inside
// ADR-0003's 50ms admission ceiling. Unbounded, a bulk move of 100k
// messages becomes one claim resolving every message in the queue
// while every other writer, interactive lane included, waits.
const (
	claimMessageBudget = 1000
	claimRowBudget     = 200
)

// claimLimit returns the most intents one claim transaction moves to
// dispatching. A backend's per-batch object limit is also the largest
// number of messages one claimed chunk can name, so it divides the
// message budget. Whatever a pass leaves queued the next one takes.
func claimLimit(be backend.Backend) int {
	return max(1, min(claimMessageBudget/chunkSize(be), claimRowBudget))
}

// resolveClaim reads r's referents from the store, marking r
// pre-failed with ClassNotFound when a referent it needs is missing:
// a deleted mailbox or message, or a mailbox still awaiting its own
// server assignment. A referent named by a same-batch creation
// reference (ParentRef, DestRef) resolves later, once that create has
// dispatched, and is left unresolved here.
func resolveClaim(tx *sql.Tx, r row) (claimed, error) {
	c := claimed{row: r}
	switch r.kind {
	case KindCreateMailbox:
		var p CreateMailboxPayload
		if err := json.Unmarshal(r.payload, &p); err != nil {
			return c, err
		}
		if p.ParentServerID != "" {
			c.parentServerID = p.ParentServerID
			return c, nil
		}
		if p.ParentMailboxID == 0 {
			return c, nil
		}
		serverID, err := mailboxServerID(tx, p.ParentMailboxID)
		if err != nil {
			return c, err
		}
		if serverID == "" {
			return notFound(c, "parent mailbox not found"), nil
		}
		c.parentServerID = serverID

	case KindRenameMailbox:
		var p RenameMailboxPayload
		if err := json.Unmarshal(r.payload, &p); err != nil {
			return c, err
		}
		serverID, err := mailboxServerID(tx, p.MailboxID)
		if err != nil {
			return c, err
		}
		if serverID == "" {
			return notFound(c, "mailbox not found"), nil
		}
		c.mailboxServerID = serverID

	case KindDeleteMailbox:
		var p DeleteMailboxPayload
		if err := json.Unmarshal(r.payload, &p); err != nil {
			return c, err
		}
		serverID, err := mailboxServerID(tx, p.MailboxID)
		if err != nil {
			return c, err
		}
		if serverID == "" {
			return notFound(c, "mailbox not found"), nil
		}
		c.mailboxServerID = serverID

	case KindMoveMessages:
		var p MoveMessagesPayload
		if err := json.Unmarshal(r.payload, &p); err != nil {
			return c, err
		}
		c.messageServerIDs = make(map[int64]string, len(p.MessageIDs))
		for _, msgID := range p.MessageIDs {
			serverID, err := messageServerID(tx, msgID)
			if err != nil {
				return c, err
			}
			if serverID == "" {
				return notFound(c, "message not found"), nil
			}
			c.messageServerIDs[msgID] = serverID
		}
		if p.DestServerID != "" {
			c.destServerID = p.DestServerID
			return c, nil
		}
		if p.DestMailboxID == 0 {
			return c, nil
		}
		serverID, err := mailboxServerID(tx, p.DestMailboxID)
		if err != nil {
			return c, err
		}
		if serverID == "" {
			return notFound(c, "destination mailbox not found"), nil
		}
		c.destServerID = serverID
	}
	return c, nil
}

func notFound(c claimed, detail string) claimed {
	c.preFailed, c.class, c.detail = true, uerr.ClassNotFound, detail
	return c
}

// dispatchCreateMailbox executes c, a claimed KindCreateMailbox
// intent, recording the mailbox's resolved server id in
// resolvedCreates on success. A payload already carrying
// ResolvedServerID is an idempotent replay: the earlier attempt's
// CreateMailbox call is not repeated, though resolveDependentRefs
// still runs every time, in case an earlier attempt's own requeue
// happened before it reached that step. A payload that never learned
// the id, because the run that made the mailbox died before recording
// it, makes the call again and adopts whatever the server refuses it
// for.
func (d *Dispatcher) dispatchCreateMailbox(ctx context.Context, c claimed, resolvedCreates map[int64]string) outcome {
	var p CreateMailboxPayload
	if err := json.Unmarshal(c.payload, &p); err != nil {
		return failedOutcome(c, uerr.ClassServer, err.Error())
	}

	newID := p.ResolvedServerID
	if newID == "" {
		parentID := c.parentServerID
		if p.ParentRef != 0 && parentID == "" {
			parentID = resolvedCreates[p.ParentRef]
		}
		var err error
		newID, err = d.backend.Mail().CreateMailbox(ctx, p.Name, parentID)
		if err != nil {
			if !errors.Is(err, backend.ErrMailboxNameExists) {
				class, detail := classifyFailure(err)
				return failedOutcome(c, class, detail)
			}
			newID, err = d.adoptMailbox(ctx, c.id, p.Name, parentID)
			if err != nil {
				return adoptionFailure(c, err)
			}
		}

		p.ResolvedServerID = newID
		payload, err := json.Marshal(p)
		if err != nil {
			return failedOutcome(c, uerr.ClassStoreLocal, err.Error())
		}
		if err := d.persistPayload(ctx, c.id, payload); err != nil {
			return failedOutcome(c, uerr.ClassStoreLocal, err.Error())
		}
	}
	if err := d.resolveDependentRefs(ctx, c.id, newID); err != nil {
		return failedOutcome(c, uerr.ClassStoreLocal, err.Error())
	}
	resolvedCreates[c.id] = newID
	return outcome{c: c}
}

// errAmbiguousMailbox reports that a create the server refused as a
// duplicate could not be reconciled against a single mailbox: the
// account holds none of that name under that parent, or holds more
// than one. More than one is a genuine contradiction: the server holds
// two siblings its own refusal says it forbids. None is not; it is the
// benign case adoptMailbox's own doc comment names, a server whose
// sibling-uniqueness rule matches more loosely than FindMailboxes'
// exact one. Either way there is nothing here to adopt.
var errAmbiguousMailbox = errors.New("outbox: no single mailbox matches the name the server refused as a duplicate")

// adoptMailbox returns the server id of the mailbox a refused create
// says is already there, and is the whole answer to a replayed create.
// RFC 8621 section 2 forbids two sibling mailboxes with the same
// parent and the same name, and both servers poplar is exercised
// against refuse the second create rather than allowing it, so an
// alreadyExists refusal means a sibling the server itself considers
// the same name and parent is already there, whether an earlier
// attempt of this same intent made it (a run killed between the
// create and the transaction recording its id) or the user made it on
// another client. Both wanted the same folder in the same place, and
// the alternative to converging on it is a second folder of that name
// beside it. Nothing here has to tell a replay from a first attempt,
// which is why closing this window costs the outbox no marker for
// one.
//
// Binding is on an exact name and parent, which is FindMailboxes'
// contract, and it can be narrower than the comparison a server's own
// refusal used. Stalwart's sibling-uniqueness rule is case-insensitive
// and trims whitespace where Fastmail's is byte-exact, so Stalwart can
// refuse a create against a case- or whitespace-differing sibling that
// FindMailboxes' exact comparison then finds nothing to adopt: the
// server is perfectly self-consistent, and poplar's own match is just
// narrower than its rule. Two matches and zero are both
// errAmbiguousMailbox and both terminal, for different reasons. Two:
// choosing between them would file mail into a folder nobody named,
// worse than the duplicate this exists to prevent because nothing
// about it is visible. Zero: a server whose rule is looser than this
// exact match answers every retry the same way, so retrying would
// only loop at the 30-second backoff cap forever.
func (d *Dispatcher) adoptMailbox(ctx context.Context, intentID int64, name, parentID string) (string, error) {
	ids, err := d.backend.Mail().FindMailboxes(ctx, name, parentID)
	if err != nil {
		return "", err
	}
	if len(ids) != 1 {
		return "", fmt.Errorf("%w: %q matched %d", errAmbiguousMailbox, name, len(ids))
	}
	// A recovery rather than a failure: the intent goes on to complete
	// against the mailbox that was already there, so nothing surfaces
	// to the user and this line is the only record it happened.
	slog.Info("outbox: adopted the mailbox a refused create already made", "intent", intentID, "mailbox", ids[0])
	return ids[0], nil
}

// adoptionFailure is the disposition of a create the reconcile could
// not resolve. An ambiguous match is final: the mailbox is on the
// server, every later attempt earns the same refusal, and no candidate
// met the exact-match contract adoptMailbox binds on, whether because
// two did or because none did. A lookup call that failed on its own
// says nothing about the account, so it keeps its class's ordinary
// retry.
func adoptionFailure(c claimed, err error) outcome {
	class, detail := classifyFailure(err)
	o := failedOutcome(c, class, detail)
	o.final = errors.Is(err, errAmbiguousMailbox)
	return o
}

func failedOutcome(c claimed, class uerr.Class, detail string) outcome {
	return outcome{c: c, failed: true, class: class, detail: detail}
}

// resolveDependentRefs persists newID, createID's own resolved server
// id, into every other row of this account's outbox whose payload
// still names createID by DestRef (a move) or ParentRef (a nested
// create). createID's own row is deleted once this pass finalizes, so
// without this a dependent that does not finish dispatching in the
// same pass, whether it was never claimed this pass or was claimed
// and then requeued after a failure, finds nothing left in the store
// to resolve its back-reference against on its next attempt.
func (d *Dispatcher) resolveDependentRefs(ctx context.Context, createID int64, newID string) error {
	return d.writer.ApplyInteractive(ctx, func(tx *sql.Tx) error {
		rows, err := selectByAccount(tx, d.accountID, createID)
		if err != nil {
			return err
		}
		for _, r := range rows {
			patched, ok, err := rewriteRef(r, createID, newID)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			if err := updatePayload(tx, r.id, patched); err != nil {
				return err
			}
		}
		return nil
	})
}

// rewriteRef reports whether r's payload names createID by DestRef or
// ParentRef, returning the payload with that reference replaced by
// serverID if so.
func rewriteRef(r row, createID int64, serverID string) (patched []byte, ok bool, err error) {
	switch r.kind {
	case KindMoveMessages:
		var p MoveMessagesPayload
		if err := json.Unmarshal(r.payload, &p); err != nil {
			return nil, false, err
		}
		if p.DestRef != createID {
			return nil, false, nil
		}
		p.DestRef, p.DestServerID = 0, serverID
		patched, err = json.Marshal(p)
	case KindCreateMailbox:
		var p CreateMailboxPayload
		if err := json.Unmarshal(r.payload, &p); err != nil {
			return nil, false, err
		}
		if p.ParentRef != createID {
			return nil, false, nil
		}
		p.ParentRef, p.ParentServerID = 0, serverID
		patched, err = json.Marshal(p)
	default:
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return patched, true, nil
}

func (d *Dispatcher) dispatchRenameMailbox(ctx context.Context, c claimed) (bool, uerr.Class, string) {
	var p RenameMailboxPayload
	if err := json.Unmarshal(c.payload, &p); err != nil {
		return true, uerr.ClassServer, err.Error()
	}
	if err := d.backend.Mail().RenameMailbox(ctx, c.mailboxServerID, p.Name); err != nil {
		class, detail := classifyFailure(err)
		return true, class, detail
	}
	return false, 0, ""
}

func (d *Dispatcher) dispatchDeleteMailbox(ctx context.Context, c claimed) (bool, uerr.Class, string) {
	if err := d.backend.Mail().DeleteMailbox(ctx, c.mailboxServerID); err != nil {
		class, detail := classifyFailure(err)
		return true, class, detail
	}
	return false, 0, ""
}

// dispatchMoveMessages executes c, a claimed KindMoveMessages intent,
// as one ApplyBatch call: each chunk is its own request, so the
// backend's maxObjectsInSet bound applies per chunk rather than to
// the whole bulk action. A chunk fails as a whole if any of its
// messages fail, using the first failure's class. It returns the
// decoded payload on success, so a caller building this intent's
// Delivered entry does not have to decode c.payload a second time.
func (d *Dispatcher) dispatchMoveMessages(ctx context.Context, c claimed, resolvedCreates map[int64]string) (*MoveMessagesPayload, bool, uerr.Class, string) {
	var p MoveMessagesPayload
	if err := json.Unmarshal(c.payload, &p); err != nil {
		return nil, true, uerr.ClassServer, err.Error()
	}

	dest := c.destServerID
	if p.DestRef != 0 && dest == "" {
		dest = resolvedCreates[p.DestRef]
	}
	if dest == "" {
		return nil, true, uerr.ClassNotFound, "move destination not resolved"
	}

	mutations := fileIntoMailbox(c, p.MessageIDs, dest)

	res, err := d.backend.Mail().ApplyBatch(ctx, mutations)
	if err != nil {
		class, detail := classifyFailure(err)
		return nil, true, class, detail
	}
	for _, mut := range mutations {
		if mutErr, bad := res.Failed[mut.ID]; bad {
			class, detail := classifyFailure(mutErr)
			return nil, true, class, detail
		}
	}
	return &p, false, 0, ""
}

// persistPayload writes payload back into id's row in its own
// transaction, immediately after a successful backend call, so a
// crash before this pass's finalize step still leaves the resolution
// durable.
func (d *Dispatcher) persistPayload(ctx context.Context, id int64, payload []byte) error {
	return d.writer.ApplyInteractive(ctx, func(tx *sql.Tx) error {
		return updatePayload(tx, id, payload)
	})
}

// isRetriable reports whether class's failure is worth trying again:
// every class but ClassNotFound, whose referent is gone and will not
// reappear on its own.
func isRetriable(class uerr.Class) bool {
	return class != uerr.ClassNotFound
}

// isWarn reports whether class surfaces as SY-5's warn state rather
// than an error toast: only ClassThrottled, a request the server
// itself asked to be retried.
func isWarn(class uerr.Class) bool {
	return class == uerr.ClassThrottled
}

// backoff returns the delay before attempt's next retry: doubling
// from one second, capped at 30 seconds so a long outage never
// starves the queue of retries entirely.
func backoff(attempt int) time.Duration {
	d := time.Second << attempt
	if d <= 0 || d > 30*time.Second {
		return 30 * time.Second
	}
	return d
}

// classifyFailure recovers the uerr.Class a backend attached to err:
// a backend.Failure from a per-mutation ApplyBatch result, or
// a uerr.Error from a whole-call transport failure. A backend
// returning neither is classified ClassServer, the same fallback
// jmap's own classification tables use for an unrecognized failure.
func classifyFailure(err error) (uerr.Class, string) {
	if mf, ok := errors.AsType[backend.Failure](err); ok {
		return mf.Class, mf.Cause.Error()
	}
	if ue, ok := errors.AsType[uerr.Error](err); ok {
		return ue.Class, ue.Error()
	}
	return uerr.ClassServer, err.Error()
}
