package allino

import (
	"os"
)

func environMap() map[string]string {
	// collect values from os.Environ
	result := make(map[string]string)
	for _, kv := range os.Environ() {
		// split key=value
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				result[kv[:i]] = kv[i+1:]
				break
			}
		}
	}

	return result
}
