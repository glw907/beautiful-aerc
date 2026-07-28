package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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
// true when the row stays queued for another attempt (every class but
// ClassNotFound, whose referent retrying cannot fix); Warn is SY-5's
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

// finalizeAction is claim's disposition once DispatchOnce decides it,
// applied inside finalize's one writer transaction.
type finalizeAction struct {
	id      int64
	attempt int
	verb    finalizeVerb
	class   string
	detail  string
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
// claimed but never attempted back to queued untouched.
func (d *Dispatcher) DispatchOnce(ctx context.Context, now time.Time) (Result, error) {
	claimedRows, err := d.claim(ctx, now)
	if err != nil {
		return Result{}, err
	}

	var result Result
	var actions []finalizeAction
	resolvedCreates := map[int64]string{}
	stopped := false

	for _, c := range claimedRows {
		if stopped {
			actions = append(actions, finalizeAction{id: c.id, verb: finalizeRevert})
			continue
		}
		if c.preFailed {
			actions = append(actions, d.report(&result, c, c.class, c.detail))
			continue
		}

		var class uerr.Class
		var detail string
		var failed bool
		var move *MoveMessagesPayload
		switch c.kind {
		case KindCreateMailbox:
			var serverID string
			serverID, failed, class, detail = d.dispatchCreateMailbox(ctx, c, resolvedCreates)
			if !failed {
				resolvedCreates[c.id] = serverID
			}
		case KindRenameMailbox:
			failed, class, detail = d.dispatchRenameMailbox(ctx, c)
		case KindDeleteMailbox:
			failed, class, detail = d.dispatchDeleteMailbox(ctx, c)
		case KindMoveMessages:
			move, failed, class, detail = d.dispatchMoveMessages(ctx, c, resolvedCreates)
		default:
			// A kind this build does not recognize (a newer poplar
			// wrote it, or the schema grew one this dispatcher has not
			// caught up with) fails this one intent rather than
			// aborting the pass: every other claimed row still needs
			// its finalize action decided below.
			failed, class, detail = true, uerr.ClassServer, fmt.Sprintf("unknown kind %q", c.kind)
		}

		if failed {
			actions = append(actions, d.report(&result, c, class, detail))
			stopped = class == uerr.ClassConnection
			continue
		}
		result.Delivered = append(result.Delivered, delivered(c, move))
		actions = append(actions, finalizeAction{id: c.id, verb: finalizeDelete})
	}

	err = d.writer.ApplyInteractive(ctx, func(tx *sql.Tx) error {
		for _, a := range actions {
			var err error
			switch a.verb {
			case finalizeDelete:
				err = deleteRow(tx, a.id)
			case finalizeRevert:
				err = revertRow(tx, a.id)
			case finalizeRequeue:
				err = requeueRow(tx, a.id, now.Add(backoff(a.attempt)), a.class, a.detail)
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
	return result, err
}

// report appends c's classified failure to result, logs it through
// uerr's one seam when shouldLogFailure says it is new, and returns
// its finalize action: requeue with backoff if class is worth
// retrying, delete (give up and let the caller's report stand)
// otherwise.
func (d *Dispatcher) report(result *Result, c claimed, class uerr.Class, detail string) finalizeAction {
	retry := isRetriable(class)
	if shouldLogFailure(c.failureClass, class, retry) {
		_ = uerr.New("outbox.dispatch", failureIDs(c), class, errors.New(detail))
	}
	result.Failures = append(result.Failures, Failure{
		IntentID: c.id, UndoGroup: c.undoGroup, Class: class, Detail: detail, Retrying: retry, Warn: isWarn(class),
	})
	if !retry {
		return finalizeAction{id: c.id, verb: finalizeDelete}
	}
	return finalizeAction{id: c.id, attempt: c.attemptCount, verb: finalizeRequeue, class: class.String(), detail: detail}
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
		rows, err := selectEligible(tx, d.accountID, now)
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
// intent, returning the mailbox's resolved server id on success. A
// payload already carrying ResolvedServerID is an idempotent replay:
// the earlier attempt's CreateMailbox call is not repeated, though
// resolveDependentRefs still runs every time, in case an earlier
// attempt's own requeue happened before it reached that step.
func (d *Dispatcher) dispatchCreateMailbox(ctx context.Context, c claimed, resolvedCreates map[int64]string) (string, bool, uerr.Class, string) {
	var p CreateMailboxPayload
	if err := json.Unmarshal(c.payload, &p); err != nil {
		return "", true, uerr.ClassServer, err.Error()
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
			class, detail := classifyFailure(err)
			return "", true, class, detail
		}

		p.ResolvedServerID = newID
		payload, err := json.Marshal(p)
		if err != nil {
			return "", true, uerr.ClassServer, err.Error()
		}
		if err := d.persistPayload(ctx, c.id, payload); err != nil {
			return "", true, uerr.ClassServer, err.Error()
		}
	}
	if err := d.resolveDependentRefs(ctx, c.id, newID); err != nil {
		return "", true, uerr.ClassServer, err.Error()
	}
	return newID, false, 0, ""
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

	mutations := make([]backend.Mutation, 0, len(p.MessageIDs))
	for _, msgID := range p.MessageIDs {
		mutations = append(mutations, backend.Mutation{
			Op:     backend.MutationUpdate,
			ID:     c.messageServerIDs[msgID],
			Fields: map[string]any{"mailbox_ids": []string{dest}},
		})
	}

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
// a backend.MutationFailure from a per-mutation ApplyBatch result, or
// a uerr.Error from a whole-call transport failure. A backend
// returning neither is classified ClassServer, the same fallback
// jmap's own classification tables use for an unrecognized failure.
func classifyFailure(err error) (uerr.Class, string) {
	if mf, ok := errors.AsType[backend.MutationFailure](err); ok {
		return mf.Class, mf.Cause.Error()
	}
	if ue, ok := errors.AsType[uerr.Error](err); ok {
		return ue.Class, ue.Error()
	}
	return uerr.ClassServer, err.Error()
}
