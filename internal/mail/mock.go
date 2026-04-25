package mail

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

// MockBackend implements Backend with hardcoded data.
// Used for prototype development, testing, and demos.
type MockBackend struct {
	name    string
	folders []Folder
	msgs    []MessageInfo
	updates chan Update
}

// NewMockBackend creates a MockBackend with realistic sample data.
func NewMockBackend() *MockBackend {
	// Mock timestamps live in time.Local so the hour/minute values
	// below render verbatim for an interactive demo, regardless of
	// the developer's timezone. A CI/golden-file setup that needs
	// reproducible output across timezones should pin a fixed
	// location here instead.
	at := func(month time.Month, day, hour, min int) time.Time {
		return time.Date(2026, month, day, hour, min, 0, 0, time.Local)
	}
	return &MockBackend{
		name: "geoff@907.life",
		folders: []Folder{
			{Name: "Inbox", Exists: 14, Unseen: 4, Role: "inbox"},
			{Name: "Drafts", Exists: 2, Unseen: 0, Role: "drafts"},
			{Name: "Sent", Exists: 145, Unseen: 0, Role: "sent"},
			{Name: "Archive", Exists: 1893, Unseen: 0, Role: "archive"},
			{Name: "Junk", Exists: 12, Unseen: 12, Role: ""},
			{Name: "Trash", Exists: 5, Unseen: 0, Role: "trash"},
			{Name: "Notifications", Exists: 47, Unseen: 0, Role: ""},
			{Name: "Remind", Exists: 8, Unseen: 0, Role: ""},
			{Name: "Lists/golang", Exists: 234, Unseen: 0, Role: ""},
			{Name: "Lists/rust", Exists: 89, Unseen: 0, Role: ""},
		},
		msgs: []MessageInfo{
			// Flat single-message threads: ThreadID == UID, no InReplyTo.
			// Only SentAt is set; the UI formats the display string via
			// formatRelativeDate at render time.
			{UID: "1", ThreadID: "1", Subject: "Re: Project update for Q2 launch", From: "Alice Johnson", SentAt: at(time.April, 13, 10, 23), Flags: 0},
			{UID: "2", ThreadID: "2", Subject: "Quick question about the API", From: "Bob Smith", SentAt: at(time.April, 13, 9, 45), Flags: 0},
			{UID: "3", ThreadID: "3", Subject: "Lunch tomorrow?", From: "Carol White", SentAt: at(time.April, 13, 9, 12), Flags: 0},
			{UID: "4", ThreadID: "4", Subject: "Meeting notes from yesterday", From: "David Chen", SentAt: at(time.April, 12, 15, 47), Flags: FlagSeen},
			{UID: "5", ThreadID: "5", Subject: "Invoice #2847 attached", From: "Billing Dept", SentAt: at(time.April, 12, 11, 32), Flags: FlagSeen | FlagFlagged},
			{UID: "6", ThreadID: "6", Subject: "Re: Weekend hiking trip", From: "Emma Wilson", SentAt: at(time.April, 12, 8, 15), Flags: FlagSeen | FlagAnswered},
			{UID: "7", ThreadID: "7", Subject: "Your subscription renewal", From: "Acme Cloud", SentAt: at(time.April, 8, 16, 22), Flags: FlagSeen},
			{UID: "8", ThreadID: "8", Subject: "Code review: auth refactor PR #42", From: "GitHub", SentAt: at(time.April, 8, 9, 30), Flags: FlagSeen},
			{UID: "9", ThreadID: "9", Subject: "New comment on your post", From: "Dev Community", SentAt: at(time.April, 7, 15, 45), Flags: FlagSeen},
			{UID: "10", ThreadID: "10", Subject: "Flight confirmation: SFO → SEA", From: "Alaska Airlines", SentAt: at(time.April, 7, 10, 15), Flags: FlagSeen | FlagFlagged},

			// Threaded conversation T1: branching shape (root + linear chain + sibling).
			// Exercises the full ├─ │ └─ prefix vocabulary. First child unread so a
			// folded thread can still carry "contains unread" status.
			{UID: "20", ThreadID: "T1", InReplyTo: "", Subject: "Server migration plan", From: "Frank Lee", SentAt: at(time.April, 5, 9, 0), Flags: FlagSeen | FlagAnswered},
			{UID: "21", ThreadID: "T1", InReplyTo: "20", Subject: "Re: Server migration plan", From: "Grace Kim", SentAt: at(time.April, 5, 11, 30), Flags: 0},
			{UID: "22", ThreadID: "T1", InReplyTo: "21", Subject: "Re: Server migration plan", From: "Frank Lee", SentAt: at(time.April, 5, 14, 15), Flags: FlagSeen},
			{UID: "23", ThreadID: "T1", InReplyTo: "20", Subject: "Re: Server migration plan", From: "Henry Park", SentAt: at(time.April, 5, 16, 45), Flags: FlagSeen},
		},
		updates: make(chan Update),
	}
}

func (m *MockBackend) AccountName() string              { return m.name }
func (m *MockBackend) Connect(_ context.Context) error { return nil }
func (m *MockBackend) Disconnect() error               { return nil }

// ListFolders returns the hardcoded folder list.
func (m *MockBackend) ListFolders() ([]Folder, error) {
	return m.folders, nil
}

// OpenFolder is a no-op for the mock backend.
func (m *MockBackend) OpenFolder(_ string) error { return nil }

// FetchHeaders returns the hardcoded message list. The uids parameter is
// ignored — the mock always returns all messages.
func (m *MockBackend) FetchHeaders(_ []UID) ([]MessageInfo, error) {
	return m.msgs, nil
}

// FetchBody returns a realistic markdown body for stress-testing the
// content render pipeline. Each mapped UID exercises a distinct wrap
// path (styled spans straddling the 72-col cap, nested quotes, long
// URLs, code blocks, list hanging indent, footnote markers at the
// column boundary). Unmapped UIDs fall back to a short default.
func (m *MockBackend) FetchBody(uid UID) (io.Reader, error) {
	if body, ok := mockBodies[uid]; ok {
		return strings.NewReader(body), nil
	}
	return strings.NewReader(fmt.Sprintf("Mock body for message %s\n\nNo extended content available for this UID.", uid)), nil
}

// mockBodies holds realistic markdown bodies keyed by UID, chosen to
// stress the wrap pipeline (styled spans across the cap, nested
// quotes, long URLs, code blocks, list hanging indent, footnote
// markers at the column boundary).
var mockBodies = map[UID]string{
	"1": `Hi Geoff,

Just wanted to follow up on the Q2 launch timeline. The team has been hard at work on the **infrastructure migration** and we're tracking ahead of schedule on most fronts, though there are a couple of *unexpected blockers* we should discuss in our sync tomorrow.

The most pressing item is the ` + "`config.production.yaml`" + ` rotation — we'll need to coordinate with ops before Friday. I've drafted the changes in [the migration doc](https://docs.example.com/q2-migration) but haven't pushed them to the [shared review folder](https://drive.example.com/q2-shared) yet.

Let me know if you have any concerns about the timeline or if there's anything I should escalate to leadership before the all-hands.

Thanks,
Alice
`,

	"2": `Quick API question — the ` + "`/v2/messages`" + ` endpoint is returning a 422 when I send the new ` + "`thread_id`" + ` parameter you mentioned in standup. Wondering if there's a deployment lag or if I'm holding it wrong.

On Tue, Apr 12, 2026 at 3:47 PM, Alice Johnson wrote:
> The new endpoint is live in staging but production is gated behind the
> rollout flag. You'll want to use the staging URL for now:
>
> > https://staging-api.example.com/v2/messages
>
> Production will flip on Thursday assuming the smoke tests pass.

Got it — thanks for the context. I'll point my client at staging and pick this back up Thursday afternoon if production is green.

Cheers,
Bob
`,

	"3": `Hey! Want to grab lunch tomorrow? Thinking around 12:30 at the place on the corner — they have that special you liked last time.

Carol
`,

	"4": `Quick recap of yesterday's planning meeting. Decisions and follow-ups below.

Decisions made:

- Ship the auth refactor as a separate PR rather than bundling it with the user-profile work — easier to revert if the migration surfaces edge cases.
- Defer the search-index rebuild to next sprint; the current latency numbers from monitoring are within tolerance and the team has higher-priority work this cycle.
- Adopt the new linter ruleset across the monorepo, with a two-week grace period during which existing violations are warning-only.
- Switch the staging deploy cadence from nightly to twice-weekly so the QA team gets longer windows to validate.

Action items:

- David to draft the linter migration plan with dependencies and expected breakage radius.
- Emma to circulate the auth refactor RFC for async review.
- Frank to coordinate the staging cadence change with the platform team and update the runbook.

Reference materials are in the [planning folder](https://drive.example.com/planning-2026-q2) and the raw notes from the meeting are at https://notes.example.com/2026-04-12 if anyone wants the unredacted version.

— David
`,

	"5": `Heads up — the new invoice format includes a structured ` + "`metadata`" + ` block at the top. Here's a representative example so you can update your downstream parsers before the rollout next week:

` + "```json\n" + `{
  "invoice_id": "INV-2026-04-2847",
  "issued_at": "2026-04-12T11:32:00-06:00",
  "billing_period": {"start": "2026-03-01", "end": "2026-03-31"},
  "line_items": [{"sku": "ENT-CLOUD-MO", "quantity": 1, "unit_amount": "499.00"}]
}` + "\n```" + `

Note that the JSON is intentionally formatted with significant whitespace so it stays readable in plaintext mail clients — your parser should not assume single-line. The full schema is documented at [the billing API reference](https://billing.example.com/v3/schema/invoice).

Let me know if there are integration questions before Friday.
`,

	"6": `Saturday is shaping up well — forecast says clear and around 60°F, which is about as good as we're going to get this time of year. Trail conditions on the [Cougar Mountain loop](https://wta.example.org/trails/cougar-mountain-loop) look dry per the latest report, so we should be good to go.

Plan: meet at the trailhead at 8 AM, finish around 1 PM, then back to my place for that taco recipe I've been talking about. Bring water and layers — it'll be chilly until the sun gets above the ridge.

Also — bringing the dogs. Hope that's still good with everyone.

Emma
`,

	"7": `Your Acme Cloud Pro subscription will renew on May 8, 2026 for $29.00 USD. No action required — your payment method ending in 4242 will be charged automatically.

If you'd like to make any changes, you can do so from your account dashboard before the renewal date.

Thanks for being a customer.
`,

	"8": `New activity on PR #42: auth-refactor

> 4 files changed, 287 insertions(+), 412 deletions(-)
>
> The middleware extraction looks clean. One concern: the new
> ` + "`SessionGuard`" + ` type holds a reference to the request context — is
> that intentional, or should it be detached for the goroutine that
> handles refresh? See inline comment on line 184.

Reply directly to this email or visit the discussion at https://github.example.com/acme/server/pull/42 to continue the conversation.
`,
}

func (m *MockBackend) Search(_ SearchCriteria) ([]UID, error) { return nil, nil }
func (m *MockBackend) Move(_ []UID, _ string) error           { return nil }
func (m *MockBackend) Copy(_ []UID, _ string) error           { return nil }
func (m *MockBackend) Delete(_ []UID) error                   { return nil }
func (m *MockBackend) Flag(_ []UID, _ Flag, _ bool) error     { return nil }
func (m *MockBackend) MarkRead(_ []UID) error                 { return nil }
func (m *MockBackend) MarkAnswered(_ []UID) error             { return nil }

func (m *MockBackend) Send(_ string, _ []string, _ io.Reader) error {
	return nil
}

// Updates returns the update channel. The mock backend never sends updates.
func (m *MockBackend) Updates() <-chan Update {
	return m.updates
}
