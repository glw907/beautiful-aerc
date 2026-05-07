package contacts

// OpenPopoverMsg asks App to open the sender popover for the given display
// name and email address extracted from the focused message's From header.
type OpenPopoverMsg struct {
	DisplayName string
	Email       string
}

// ClosePopoverMsg asks App to dismiss the sender popover.
type ClosePopoverMsg struct{}

// EnterContactsModeMsg asks App to switch the right pane to Contacts mode.
type EnterContactsModeMsg struct{}

// ExitContactsModeMsg asks App to dismiss Contacts mode.
type ExitContactsModeMsg struct{}

// OpenFormMsg asks App to open the contact edit form. FromPopover is true
// when the form was launched from the i-popover's 'n' binding. The caller
// should dismiss the popover before mounting the form.
type OpenFormMsg struct {
	Initial     Contact
	FromPopover bool
}

// ContactSaveMsg carries the edited contact and the destination chosen by
// the user in the save dialog ("Local file" or an account name).
type ContactSaveMsg struct {
	Contact Contact
	SaveTo  string
}

// ContactCancelMsg is emitted when the user exits the edit form. Dirty is
// true when the form had unsaved changes.
type ContactCancelMsg struct{ Dirty bool }
