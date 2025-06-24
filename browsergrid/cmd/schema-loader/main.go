package main

import (
	"fmt"
	"io"
	"os"

	"ariga.io/atlas-provider-gorm/gormschema"

	"github.com/autocrawlerHQ/browsergrid/internal/sessions"
	"github.com/autocrawlerHQ/browsergrid/internal/workpool"
)

func main() {
	stmts, err := gormschema.New("postgres").Load(
		&sessions.Session{},
		&sessions.SessionEvent{},
		&sessions.SessionMetrics{},
		&sessions.Pool{},
		&workpool.WorkPool{},
		&workpool.Worker{},
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load gorm schema: %v\n", err)
		os.Exit(1)
	}
	io.WriteString(os.Stdout, stmts)
}
