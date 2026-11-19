package query

import (
	"encoding/json"
	"strconv"
)

// CountArray counts the final selected array, not JSONPath matches. Absence is
// handled by the caller; null, scalars and objects are not arrays.
func CountArray(value any) (json.Number, bool) {
	array, ok := value.([]any)
	if !ok {
		return "", false
	}
	return json.Number(strconv.Itoa(len(array))), true
}
