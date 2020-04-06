package main

import (
	"os"
	"time"

	"github.com/dotwaffle/ovplusplus/cmd"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	}).With().Caller().Logger()
	cmd.Execute()
}
