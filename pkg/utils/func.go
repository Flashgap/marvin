package utils

import (
	"slices"
	"strings"
)

// CleanFuncName cleans the function name from the package path and receiver type.
// Example of func name: github.com/org/package/module.(*Struct).handlerName-fm
func CleanFuncName(funcName string) string {
	const funcParts = 3

	// Remove the package path
	funcName = funcName[strings.LastIndex(funcName, "/")+1:]
	parts := strings.Split(funcName, ".")

	if len(parts) == funcParts {
		parts = slices.Delete(parts, 1, 2) //nolint:gomnd
	}

	return strings.TrimSuffix(strings.Join(parts, "."), "-fm")
}
