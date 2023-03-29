package hooks

type RunnerHook interface {
	Name() string
}

type RunnerPrerunHook interface {
	RunnerHook
	Prerun() error
}

type RunnerPostrunHook interface {
	RunnerHook
	Postrun() error
}
