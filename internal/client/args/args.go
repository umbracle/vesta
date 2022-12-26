package args

import "regexp"

var (
	envRe = regexp.MustCompile(`\${[a-zA-Z0-9_\-\.]+}`)
)

func ReplaceEnv(arg string, environments ...map[string]string) string {
	return envRe.ReplaceAllStringFunc(arg, func(arg string) string {
		stripped := arg[2 : len(arg)-1]
		for _, env := range environments {
			if value, ok := env[stripped]; ok {
				return value
			}
		}

		return arg
	})
}
