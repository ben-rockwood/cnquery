// Code generated by "stringer -type=Feature"; DO NOT EDIT.

package cnquery

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[MassQueries-1]
	_ = x[PiperCode-2]
	_ = x[BoolAssertions-3]
}

const _Feature_name = "MassQueriesPiperCodeBoolAssertions"

var _Feature_index = [...]uint8{0, 11, 20, 34}

func (i Feature) String() string {
	i -= 1
	if i >= Feature(len(_Feature_index)-1) {
		return "Feature(" + strconv.FormatInt(int64(i+1), 10) + ")"
	}
	return _Feature_name[_Feature_index[i]:_Feature_index[i+1]]
}