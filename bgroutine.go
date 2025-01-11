package goflexutils

import (
	"sync"

	"github.com/rs/zerolog"
)

type BGRoutine func(exitChan <-chan struct{})

func StartBackgroundRoutine(log zerolog.Logger, name string, workfn BGRoutine) (stopfunc func()) {
	log.Info().Str("name", name).Msg("routine starting")
	closechan := make(chan struct{}, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		workfn(closechan)
		wg.Done()
	}()
	return func() {
		log.Info().Str("name", name).Msg("shutting down routine")
		closechan <- struct{}{}
		log.Info().Str("name", name).Msg("waiting for routine to exit")
		wg.Wait()
		log.Info().Str("name", name).Msg("routine done")
	}
}

func GoWGFunc(wg *sync.WaitGroup, f func()) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		f()
	}()
}
