package util

import "io"

// ReadAll reads all bytes from an io.Reader.
func ReadAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}
