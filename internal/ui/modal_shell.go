// SPDX-License-Identifier: MIT

package ui

import "github.com/glw907/poplar/internal/ui/uicore"

// ModalShell is the shared lifecycle shell for modal overlay components.
// The canonical definition lives in internal/ui/uicore; this alias keeps
// internal/ui consumers source-compatible during the subpackage migration.
type ModalShell = uicore.ModalShell
