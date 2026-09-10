package permissions

import "strings"

var sensitivePatterns = []string{
	".env",
	".git/config",
	"id_rsa",
	"id_ed25519",
	".aws/credentials",
	".ssh/",
}

func IsForbidden(cmd string) bool {
	for _, p := range sensitivePatterns {
		if strings.Contains(cmd, p) {
			return true
		}
	}
	return false
}
