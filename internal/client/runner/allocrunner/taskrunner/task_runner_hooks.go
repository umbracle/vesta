package taskrunner

import (
	"time"

	"github.com/umbracle/vesta/internal/client/runner/hooks"
)

func (t *TaskRunner) initHooks() {
	sysHooks := []hooks.TaskHook{
		newTaskDirHook(t.logger, t.alloc, t.taskDir, t.task, t),
	}

	for _, hook := range sysHooks {
		t.runnerHooks = append(t.runnerHooks, hook)
	}
}

func (t *TaskRunner) preStart() error {
	if t.logger.IsTrace() {
		start := time.Now()
		t.logger.Trace("running prestart hooks", "start", start)
		defer func() {
			end := time.Now()
			t.logger.Trace("finished prestart hooks", "end", end, "duration", end.Sub(start))
		}()
	}

	for _, hook := range t.runnerHooks {
		pre, ok := hook.(hooks.TaskPrestartHook)
		if !ok {
			continue
		}

		name := pre.Name()

		// Build the request
		req := hooks.TaskPrestartHookRequest{}

		var start time.Time
		if t.logger.IsTrace() {
			start = time.Now()
			t.logger.Trace("running prestart hook", "name", name, "start", start)
		}

		// Run the prestart hook
		if err := pre.Prestart(t.killCtx, &req); err != nil {
			return nil
		}

		if t.logger.IsTrace() {
			end := time.Now()
			t.logger.Trace("finished prestart hook", "name", name, "end", end, "duration", end.Sub(start))
		}
	}

	return nil
}

func (t *TaskRunner) postStart() error {
	if t.logger.IsTrace() {
		start := time.Now()
		t.logger.Trace("running poststart hooks", "start", start)
		defer func() {
			end := time.Now()
			t.logger.Trace("finished poststart hooks", "end", end, "duration", end.Sub(start))
		}()
	}

	for _, hook := range t.runnerHooks {
		post, ok := hook.(hooks.TaskPoststartHook)
		if !ok {
			continue
		}

		name := post.Name()

		// Build the request
		req := hooks.TaskPoststartHookRequest{
			Spec: t.handle.Network,
		}

		var start time.Time
		if t.logger.IsTrace() {
			start = time.Now()
			t.logger.Trace("running poststart hook", "name", name, "start", start)
		}

		// Run the poststart hook
		if err := post.Poststart(t.killCtx, &req); err != nil {
			return nil
		}

		if t.logger.IsTrace() {
			end := time.Now()
			t.logger.Trace("finished poststart hook", "name", name, "end", end, "duration", end.Sub(start))
		}
	}

	return nil
}

func (t *TaskRunner) stop() error {
	if t.logger.IsTrace() {
		start := time.Now()
		t.logger.Trace("running stop hooks", "start", start)
		defer func() {
			end := time.Now()
			t.logger.Trace("finished stop hooks", "end", end, "duration", end.Sub(start))
		}()
	}

	for _, hook := range t.runnerHooks {
		stop, ok := hook.(hooks.TaskStopHook)
		if !ok {
			continue
		}

		name := stop.Name()

		// Build the request
		req := hooks.TaskStopRequest{}

		var start time.Time
		if t.logger.IsTrace() {
			start = time.Now()
			t.logger.Trace("running stop hook", "name", name, "start", start)
		}

		// Run the stop hook
		if err := stop.Stop(t.killCtx, &req); err != nil {
			return nil
		}

		if t.logger.IsTrace() {
			end := time.Now()
			t.logger.Trace("finished stop hook", "name", name, "end", end, "duration", end.Sub(start))
		}
	}

	return nil
}
