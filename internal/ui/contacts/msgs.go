package contacts

// OpenPopoverMsg asks App to open the sender popover.
type OpenPopoverMsg struct {
	DisplayName string
	Email       string
}

type ClosePopoverMsg struct{}

// EnterContactsModeMsg asks App to switch the right pane to Contacts mode.
type EnterContactsModeMsg struct{}

type ExitContactsModeMsg struct{}

// OpenFormMsg asks App to open the contact edit form. UID is non-empty when
// editing an existing contact; empty means new. FromPopover is set when
// launched from the i-popover; the caller dismisses the popover before
// mounting the form.
type OpenFormMsg struct {
	Initial     Contact
	UID         string
	FromPopover bool
}

// ContactSaveMsg carries the edited contact and the chosen destination
// ("Local file" or an account name).
type ContactSaveMsg struct {
	Contact Contact
	SaveTo  string
}

// ContactCancelMsg fires when the user exits the form; Dirty is set when
// there were unsaved changes.
type ContactCancelMsg struct{ Dirty bool }

// OpenContactDeleteConfirmMsg asks App to open the deletion confirm modal
// for an existing contact. Form emits this; App handles the confirm
// cascade and emits ContactDeleteMsg on Yes.
type OpenContactDeleteConfirmMsg struct {
	UID         string
	DisplayName string
}

// ContactDeleteMsg fires after the user confirms deletion. App routes it
// to cache.QueueContactDelete and dismisses the form.
type ContactDeleteMsg struct{ UID string }
