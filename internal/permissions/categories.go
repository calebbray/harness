package permissions

import (
	"encoding/json"
	"strings"
)

type CommandCategorizer func(input json.RawMessage) string

var categorizers = map[string]CommandCategorizer{
	"bash": bashCategorizer,
}

func MatchToolCategory(tool string, input json.RawMessage) string {
	if fn, ok := categorizers[tool]; ok {
		return fn(input)
	}
	return string(input)
}

var riskyBash = []string{
	"docker run", "docker exec", "docker rm",
	"git push", "git reset --hard", "rm",
}

func bashCategorizer(input json.RawMessage) string {
	var in struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return string(input)
	}
	for _, verb := range riskyBash {
		if strings.HasPrefix(in.Command, verb) {
			return string(input)
		}
	}
	return firstNWords(in.Command, 2)
}

func firstNWords(s string, n int) string {
	words := strings.Fields(s)
	if len(words) > n {
		words = words[:n]
	}
	return strings.Join(words, " ")
}
