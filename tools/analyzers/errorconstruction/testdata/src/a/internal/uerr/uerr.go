// Package uerr is the error seam.
package uerr

// Error is poplar's error type. Callers outside this package build
// one only through New.
type Error struct {
	Code    string
	Message string
}

// Error implements the error interface.
func (e Error) Error() string { return e.Message }

// New is the exported constructor. It is the only sanctioned way
// to build an Error outside this package.
func New(code, message string) Error {
	return Error{Code: code, Message: message}
}
