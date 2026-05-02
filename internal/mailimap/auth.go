// SPDX-License-Identifier: MIT

package mailimap

import (
	"errors"

	"github.com/glw907/poplar/internal/config"
)

// dialCommand and dialIdle are filled in by Task 8.
func dialCommand(cfg config.AccountConfig) (imapClient, error) {
	return nil, errors.New("dialCommand: not implemented (Task 8)")
}

func dialIdle(cfg config.AccountConfig) (imapClient, error) {
	return nil, errors.New("dialIdle: not implemented (Task 8)")
}
