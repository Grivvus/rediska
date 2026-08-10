package lifecycle

import "strings"

/*
from valkey docs:

Supported glob-style patterns:

	h?llo matches hello, hallo and hxllo
	h*llo matches hllo and heeeello
	h[ae]llo matches hello and hallo, but not hillo
	h[^e]llo matches hallo, hbllo, ... but not hello
	h[a-b]llo matches hallo and hbllo
*/
func redisPatternToGoRegexp(pattern string) string {
	// '*' in redis syntax means any number of any characters
	// '?' means any chracter
	return strings.ReplaceAll(strings.ReplaceAll(pattern, "?", "."), "*", ".*")
}
