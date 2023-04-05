package allocrunner

import (
	"fmt"
	"time"

	"github.com/umbracle/vesta/internal/client/runner/hooks"
)

func (a *AllocRunner) initHooks() error {
	a.runnerHooks = []hooks.RunnerHook{
		newNetworkHook(a.logger, a.driver, a.alloc, a),
		newVolumeHook(a.logger, a.driver, a.allocDir, a.alloc),
	}

	return nil
}

// prerun is used to run the runners prerun hooks.
func (a *AllocRunner) prerun() error {
	if a.logger.IsTrace() {
		start := time.Now()
		a.logger.Trace("running pre-run hooks", "start", start)
		defer func() {
			end := time.Now()
			a.logger.Trace("finished pre-run hooks", "end", end, "duration", end.Sub(start))
		}()
	}

	for _, hook := range a.runnerHooks {
		pre, ok := hook.(hooks.RunnerPrerunHook)
		if !ok {
			continue
		}

		name := pre.Name()
		var start time.Time
		if a.logger.IsTrace() {
			start = time.Now()
			a.logger.Trace("running pre-run hook", "name", name, "start", start)
		}

		if err := pre.Prerun(); err != nil {
			return fmt.Errorf("pre-run hook %q failed: %v", name, err)
		}

		if a.logger.IsTrace() {
			end := time.Now()
			a.logger.Trace("finished pre-run hook", "name", name, "end", end, "duration", end.Sub(start))
		}
	}

	return nil
}

// postrun is used to run the runners postrun hooks.
func (a *AllocRunner) postrun() error {
	if a.logger.IsTrace() {
		start := time.Now()
		a.logger.Trace("running post-run hooks", "start", start)
		defer func() {
			end := time.Now()
			a.logger.Trace("finished post-run hooks", "end", end, "duration", end.Sub(start))
		}()
	}

	for _, hook := range a.runnerHooks {
		post, ok := hook.(hooks.RunnerPostrunHook)
		if !ok {
			continue
		}

		name := post.Name()
		var start time.Time
		if a.logger.IsTrace() {
			start = time.Now()
			a.logger.Trace("running post-run hook", "name", name, "start", start)
		}

		if err := post.Postrun(); err != nil {
			return fmt.Errorf("hook %q failed: %v", name, err)
		}

		if a.logger.IsTrace() {
			end := time.Now()
			a.logger.Trace("finished post-run hooks", "name", name, "end", end, "duration", end.Sub(start))
		}
	}

	return nil
}
