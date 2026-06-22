// Vendored from git.sr.ht/~rjarry/aerc (auth/xoauth2.go).
// MIT-licensed. Modifications: package renamed to mailauth. Aerc-specific
// functions (ExchangeRefreshToken, Authenticate, SaveRefreshToken,
// GetRefreshToken) and their imports (xdg, go-imap/client, golang.org/x/oauth2)
// omitted. Poplar does not use aerc's token-exchange infrastructure.
// Provides the XOAUTH2 SASL mechanism that emersion/go-sasl does not ship.
//
// Original copyright notices:
//
// Copyright (c) 2016 emersion
// Copyright (c) 2022, Oracle and/or its affiliates.

package mailauth

import (
	"encoding/json"
	"fmt"

	"github.com/emersion/go-sasl"
)

// Xoauth2Error is an error response returned by the server during XOAUTH2
// authentication.
type Xoauth2Error struct {
	Status  string `json:"status"`
	Schemes string `json:"schemes"`
	Scope   string `json:"scope"`
}

// Error implements the error interface.
func (err *Xoauth2Error) Error() string {
	return fmt.Sprintf("XOAUTH2 authentication error (%v)", err.Status)
}

type xoauth2Client struct {
	Username string
	Token    string
}

func (a *xoauth2Client) Start() (mech string, ir []byte, err error) {
	mech = "XOAUTH2"
	ir = []byte("user=" + a.Username + "\x01auth=Bearer " + a.Token + "\x01\x01")
	return
}

func (a *xoauth2Client) Next(challenge []byte) ([]byte, error) {
	// Server sent an error response.
	xoauth2Err := &Xoauth2Error{}
	if err := json.Unmarshal(challenge, xoauth2Err); err != nil {
		return nil, err
	}
	return nil, xoauth2Err
}

// NewXoauth2Client returns a SASL client implementing the XOAUTH2 mechanism,
// as described in https://developers.google.com/gmail/xoauth2_protocol.
func NewXoauth2Client(username, token string) sasl.Client {
	return &xoauth2Client{username, token}
}
