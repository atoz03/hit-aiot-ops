package main

import (
	"crypto/rand"
	"encoding/hex"
)

func newReportID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
