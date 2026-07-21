package themerunner

type runnerManifest struct {
	Runners []runnerSpec `toml:"runner"`
}

type runnerSpec struct {
	Name        string   `toml:"name"`
	Programs    []string `toml:"programs"`
	SkipEnv     []string `toml:"skip_env"`
	DefaultArgs []string `toml:"default_args"`
	Env         []string `toml:"env"`
	EnvUnset    []string `toml:"env_unset"`
	Integration string   `toml:"integration"`
}
