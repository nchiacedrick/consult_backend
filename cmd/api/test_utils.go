package main

import (
	"testing"

	"go.uber.org/zap"
)

func newTestApplication(t *testing.T) *application {
	t.Helper()

	return &application{
		logger: zap.Must(zap.NewProduction()).Sugar(),
	}
}
