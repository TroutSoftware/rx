// Code generated by "stringer -type IntentType"; DO NOT EDIT.

package rx

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[NoIntent-0]
	_ = x[Click-1]
	_ = x[DoubleClick-2]
	_ = x[DragStart-3]
	_ = x[DragOver-4]
	_ = x[DragEnd-5]
	_ = x[Drop-6]
	_ = x[EscPress-7]
	_ = x[Scroll-8]
	_ = x[Filter-9]
	_ = x[Change-10]
	_ = x[Blur-11]
	_ = x[ChangeView-12]
	_ = x[ManifestChange-13]
	_ = x[ShowDebugMenu-14]
	_ = x[CellSizeChange-15]
	_ = x[Submit-16]
	_ = x[Seppuku-17]
}

const _IntentType_name = "NoIntentClickDoubleClickDragStartDragOverDragEndDropEscPressScrollFilterChangeBlurChangeViewManifestChangeShowDebugMenuCellSizeChangeSubmitSeppuku"

var _IntentType_index = [...]uint8{0, 8, 13, 24, 33, 41, 48, 52, 60, 66, 72, 78, 82, 92, 106, 119, 133, 139, 146}

func (i IntentType) String() string {
	if i < 0 || i >= IntentType(len(_IntentType_index)-1) {
		return "IntentType(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _IntentType_name[_IntentType_index[i]:_IntentType_index[i+1]]
}
