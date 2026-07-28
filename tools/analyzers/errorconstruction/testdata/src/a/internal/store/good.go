package store

import "a/internal/uerr"

func good() error {
	return uerr.New("ESTORE", "write failed")
}
