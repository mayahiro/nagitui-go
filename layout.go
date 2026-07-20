package tui

import "math"

// LengthKind identifies a main-axis sizing rule
type LengthKind uint8

const (
	// LengthAuto uses the child's measured size
	LengthAuto LengthKind = iota
	// LengthFixed requests an exact cell count
	LengthFixed
	// LengthFlex shares cells left after non-flexible requests
	LengthFlex
	// LengthPercent requests a percentage of the parent
	LengthPercent
	// LengthMinMax requests a preferred size constrained by minimum and maximum
	LengthMinMax
)

// Length is a main-axis sizing rule for a child node
//
// Its zero value is Auto.
type Length struct {
	kind               LengthKind
	value              uint32
	minimum, preferred uint32
	maximum            uint32
}

// Auto returns a length that uses the child's measured size
func Auto() Length {
	return Length{}
}

// Fixed returns a length requesting an exact cell count
func Fixed(cells uint32) Length {
	return Length{kind: LengthFixed, value: cells}
}

// Flex returns a length sharing remaining cells by weight
func Flex(weight uint32) Length {
	return Length{kind: LengthFlex, value: weight}
}

// Percent returns a parent percentage rounded down and capped at 100
func Percent(percent uint32) Length {
	return Length{kind: LengthPercent, value: percent}
}

// MinMax returns a preferred length constrained by minimum and maximum
func MinMax(minimum, preferred, maximum uint32) Length {
	return Length{
		kind:      LengthMinMax,
		minimum:   minimum,
		preferred: preferred,
		maximum:   maximum,
	}
}

// Kind returns the length representation
func (l Length) Kind() LengthKind {
	return l.kind
}

// Value returns the cell count, weight, or percentage for Fixed, Flex, and
// Percent lengths
func (l Length) Value() (uint32, bool) {
	switch l.kind {
	case LengthFixed, LengthFlex, LengthPercent:
		return l.value, true
	default:
		return 0, false
	}
}

// Bounds returns the minimum, preferred, and maximum values for a MinMax
// length
func (l Length) Bounds() (minimum, preferred, maximum uint32, ok bool) {
	return l.minimum, l.preferred, l.maximum, l.kind == LengthMinMax
}

type layoutTrack struct {
	length  Length
	desired uint32
}

func allocate(available uint32, tracks []layoutTrack) []uint32 {
	allocations := make([]uint32, 0, len(tracks))
	minimums := make([]uint32, 0, len(tracks))
	var total uint64
	for _, track := range tracks {
		base, minimum := trackBaseAndMinimum(available, track)
		allocations = append(allocations, base)
		minimums = append(minimums, min(minimum, base))
		total = saturatingAdd64(total, uint64(base))
	}

	if total > uint64(available) {
		shrinkFromEnd(allocations, minimums, total-uint64(available))
	}

	var used uint64
	for _, allocation := range allocations {
		used += uint64(allocation)
	}
	distributeFlex(allocations, tracks, uint64(available)-used)
	return allocations
}

func trackBaseAndMinimum(available uint32, track layoutTrack) (uint32, uint32) {
	switch track.length.kind {
	case LengthAuto:
		return track.desired, 0
	case LengthFixed:
		return track.length.value, track.length.value
	case LengthFlex:
		return 0, 0
	case LengthPercent:
		percent := min(track.length.value, uint32(100))
		return uint32(uint64(available) * uint64(percent) / 100), 0
	case LengthMinMax:
		lower := min(track.length.minimum, track.length.maximum)
		upper := max(track.length.maximum, lower)
		return min(max(track.length.preferred, lower), upper), lower
	default:
		panic("nagi-tui: invalid length kind")
	}
}

func shrinkFromEnd(allocations, minimums []uint32, excess uint64) {
	for index := len(allocations) - 1; index >= 0 && excess != 0; index-- {
		reducible := allocations[index] - minimums[index]
		reduction := min(uint64(reducible), excess)
		allocations[index] -= uint32(reduction)
		excess -= reduction
	}
	for index := len(allocations) - 1; index >= 0 && excess != 0; index-- {
		reduction := min(uint64(allocations[index]), excess)
		allocations[index] -= uint32(reduction)
		excess -= reduction
	}
}

func distributeFlex(allocations []uint32, tracks []layoutTrack, remaining uint64) {
	if remaining == 0 {
		return
	}
	var totalWeight uint64
	for _, track := range tracks {
		if track.length.kind == LengthFlex {
			totalWeight = saturatingAdd64(totalWeight, uint64(track.length.value))
		}
	}
	if totalWeight == 0 {
		return
	}

	var assigned uint64
	for index, track := range tracks {
		if track.length.kind != LengthFlex {
			continue
		}
		share := remaining * uint64(track.length.value) / totalWeight
		allocation := min(share, uint64(math.MaxUint32))
		allocations[index] = saturatingAdd32(allocations[index], uint32(allocation))
		assigned = saturatingAdd64(assigned, share)
	}

	remainder := remaining - min(remaining, assigned)
	for remainder != 0 {
		progressed := false
		for index, track := range tracks {
			if track.length.kind != LengthFlex || track.length.value == 0 {
				continue
			}
			if allocations[index] != math.MaxUint32 {
				allocations[index]++
				remainder--
				progressed = true
			}
			if remainder == 0 {
				return
			}
		}
		if !progressed {
			return
		}
	}
}

func horizontalRect(parent Rect, offset, width uint32) Rect {
	return Rect{X: saturatingCoordinate(parent.X, offset), Y: parent.Y, Width: width, Height: parent.Height}
}

func verticalRect(parent Rect, offset, height uint32) Rect {
	return Rect{X: parent.X, Y: saturatingCoordinate(parent.Y, offset), Width: parent.Width, Height: height}
}

func insetRect(rect Rect, left, top, right, bottom uint32) Rect {
	return Rect{
		X:      saturatingCoordinate(rect.X, min(left, rect.Width)),
		Y:      saturatingCoordinate(rect.Y, min(top, rect.Height)),
		Width:  rect.Width - min(rect.Width, saturatingAdd32(left, right)),
		Height: rect.Height - min(rect.Height, saturatingAdd32(top, bottom)),
	}
}

func saturatingCoordinate(origin int32, offset uint32) int32 {
	value := int64(origin) + int64(offset)
	return int32(min(max(value, int64(math.MinInt32)), int64(math.MaxInt32)))
}

func saturatingAdd32(left, right uint32) uint32 {
	if math.MaxUint32-left < right {
		return math.MaxUint32
	}
	return left + right
}

func saturatingAdd64(left, right uint64) uint64 {
	if math.MaxUint64-left < right {
		return math.MaxUint64
	}
	return left + right
}
