package store

import "a/internal/uerr"

func bad() error {
	return uerr.Error{Code: "ESTORE", Message: "write failed"} // want `uerr\.Error constructed outside internal/uerr; use internal/uerr's exported constructor`
}

func badPointer() *uerr.Error {
	return &uerr.Error{Code: "ESTORE", Message: "write failed"} // want `uerr\.Error constructed outside internal/uerr; use internal/uerr's exported constructor`
}
