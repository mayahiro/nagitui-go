package widget

import (
	"fmt"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
)

// CalendarDate is a proleptic Gregorian date between years 1 and 9999
type CalendarDate struct {
	// Year is the Gregorian year
	Year int
	// Month is the one-based month
	Month uint8
	// Day is the one-based day of month
	Day uint8
}

// NewCalendarDate returns a date clamped into the supported Gregorian range
func NewCalendarDate(year, month, day int) CalendarDate {
	return normalizeCalendarDate(CalendarDate{Year: year, Month: uint8(min(max(month, 0), 255)), Day: uint8(min(max(day, 0), 255))})
}

// CalendarWeekStart controls the first displayed weekday column
type CalendarWeekStart uint8

const (
	// CalendarWeekStartsMonday renders Monday in the first column
	CalendarWeekStartsMonday CalendarWeekStart = iota
	// CalendarWeekStartsSunday renders Sunday in the first column
	CalendarWeekStartsSunday
)

// CalendarStyle contains the visual styles used by a Calendar
type CalendarStyle struct {
	// Header is used by the displayed year and month
	Header vt.Style
	// Weekday is used by weekday headings
	Weekday vt.Style
	// Normal is used by days in the displayed month
	Normal vt.Style
	// Weekend is merged over Saturdays and Sundays
	Weekend vt.Style
	// Adjacent is used by dates outside the displayed month
	Adjacent vt.Style
	// Selected is merged over the application-selected date
	Selected vt.Style
	// Focused is merged over the date that owns focus
	Focused vt.Style
	// Disabled is used when date changes are unavailable
	Disabled vt.Style
}

// DefaultCalendarStyle returns the standard calendar styles
func DefaultCalendarStyle() CalendarStyle {
	return CalendarStyle{
		Header: vt.Style{Bold: true}, Weekday: vt.Style{Dim: true},
		Weekend: vt.Style{Dim: true}, Adjacent: vt.Style{Dim: true},
		Selected: vt.Style{Reverse: true}, Focused: vt.Style{Underline: true},
		Disabled: vt.Style{Dim: true},
	}
}

// Calendar is a controlled month grid over proleptic Gregorian dates
type Calendar[Message any] struct {
	id           tui.NodeID
	year         int
	month        int
	selected     CalendarDate
	weekStart    CalendarWeekStart
	showAdjacent bool
	enabled      bool
	style        CalendarStyle
	onSelect     func(CalendarDate) Message
}

// NewCalendar returns a controlled month grid
//
// A nil onSelect function creates a disabled calendar.
func NewCalendar[Message any](id tui.NodeID, year, month int, selected CalendarDate, onSelect func(CalendarDate) Message) Calendar[Message] {
	displayed := NewCalendarDate(year, month, 1)
	return Calendar[Message]{
		id: id, year: displayed.Year, month: int(displayed.Month), selected: normalizeCalendarDate(selected),
		enabled: onSelect != nil, style: DefaultCalendarStyle(), onSelect: onSelect,
	}
}

// WeekStart replaces the first displayed weekday column
func (c Calendar[Message]) WeekStart(start CalendarWeekStart) Calendar[Message] {
	c.weekStart = start
	return c
}

// ShowAdjacent sets whether dates outside the displayed month are shown
func (c Calendar[Message]) ShowAdjacent(show bool) Calendar[Message] {
	c.showAdjacent = show
	return c
}

// Enabled sets whether the calendar can receive focus and emit messages
func (c Calendar[Message]) Enabled(enabled bool) Calendar[Message] {
	c.enabled = enabled && c.onSelect != nil
	return c
}

// Style replaces the calendar styles
func (c Calendar[Message]) Style(style CalendarStyle) Calendar[Message] {
	c.style = style
	return c
}

// Node builds the public semantic node for this calendar
func (c Calendar[Message]) Node() tui.Node[Message] {
	first := NewCalendarDate(c.year, c.month, 1)
	active := calendarActiveDate(first, c.selected)
	header := tui.Align(
		tui.StyledText[Message](fmt.Sprintf("%04d-%02d", first.Year, first.Month), c.style.Header),
		tui.AlignCenter,
		tui.AlignTop,
	).WithLength(tui.Fixed(1))
	weekdayLabels := calendarWeekdayLabels(c.weekStart)
	weekdayNodes := make([]tui.Node[Message], len(weekdayLabels))
	for index, label := range weekdayLabels {
		weekdayNodes[index] = tui.StyledText[Message](label+" ", c.style.Weekday).WithLength(tui.Fixed(3))
	}
	rows := []tui.Node[Message]{header, tui.Row(weekdayNodes...).WithLength(tui.Fixed(1))}
	offset := calendarMonthOffset(first, c.weekStart)
	for week := 0; week < 6; week++ {
		days := make([]tui.Node[Message], 0, 7)
		for weekday := 0; weekday < 7; weekday++ {
			position := week*7 + weekday
			date := addCalendarDays(first, position-offset)
			inMonth := date.Year == first.Year && date.Month == first.Month
			if !inMonth && !c.showAdjacent {
				days = append(days, tui.Text[Message]("   ").WithLength(tui.Fixed(3)))
				continue
			}
			isSelected := date == active
			style := c.style.Normal
			if !inMonth {
				style = c.style.Adjacent
			}
			if calendarWeekday(date) == 0 || calendarWeekday(date) == 6 {
				style = style.Merge(c.style.Weekend)
			}
			if isSelected {
				style = style.Merge(c.style.Selected)
			}
			if !c.enabled {
				style = c.style.Disabled
			}
			dayID := calendarDateID(c.id, date)
			dayNode := tui.StyledText[Message](fmt.Sprintf("%2d ", date.Day), style).WithLength(tui.Fixed(3))
			if !c.enabled {
				days = append(days, dayNode.WithID(dayID))
				continue
			}
			if isSelected {
				days = append(days, tui.Column(dayNode.WithID(dayID)).
					WithLength(tui.Fixed(3)).
					Focusable(c.id).
					WithFocusedStyle(c.style.Focused).
					OnEvent(c.id, c.selectedHandler(first, active)))
				continue
			}
			selectedDate := date
			days = append(days, dayNode.WithID(dayID).OnEvent(dayID, func(event vt.Event) tui.EventResult[Message] {
				if !isActivationEvent(event) {
					return tui.IgnoreResult[Message]()
				}
				return tui.ConsumeResult[Message]().Focus(c.id).Emit(c.onSelect(selectedDate))
			}))
		}
		rows = append(rows, tui.Row(days...).WithLength(tui.Fixed(1)))
	}
	root := tui.Column(rows...)
	if !c.enabled {
		return root.WithID(c.id)
	}
	return root
}

func (c Calendar[Message]) selectedHandler(displayed, selected CalendarDate) func(vt.Event) tui.EventResult[Message] {
	return func(event vt.Event) tui.EventResult[Message] {
		if isActivationEvent(event) {
			return tui.ConsumeResult[Message]().Focus(c.id)
		}
		next, handled := calendarDateForEvent(displayed, selected, event)
		if !handled {
			return tui.IgnoreResult[Message]()
		}
		result := tui.ConsumeResult[Message]().Focus(c.id)
		if next != selected {
			result = result.Emit(c.onSelect(next))
		}
		return result
	}
}

func normalizeCalendarDate(date CalendarDate) CalendarDate {
	year := min(max(date.Year, 1), 9999)
	month := min(max(int(date.Month), 1), 12)
	day := min(max(int(date.Day), 1), calendarDaysInMonth(year, month))
	return CalendarDate{Year: year, Month: uint8(month), Day: uint8(day)}
}

func calendarActiveDate(displayed, selected CalendarDate) CalendarDate {
	displayed = normalizeCalendarDate(displayed)
	selected = normalizeCalendarDate(selected)
	if selected.Year != displayed.Year || selected.Month != displayed.Month {
		return CalendarDate{Year: displayed.Year, Month: displayed.Month, Day: 1}
	}
	return selected
}

func calendarDaysInMonth(year, month int) int {
	switch month {
	case 2:
		if year%400 == 0 || year%4 == 0 && year%100 != 0 {
			return 29
		}
		return 28
	case 4, 6, 9, 11:
		return 30
	default:
		return 31
	}
}

func addCalendarDays(date CalendarDate, days int) CalendarDate {
	date = normalizeCalendarDate(date)
	for days > 0 {
		last := calendarDaysInMonth(date.Year, int(date.Month))
		if date.Year == 9999 && date.Month == 12 && int(date.Day) == last {
			return date
		}
		if int(date.Day) < last {
			date.Day++
		} else {
			date = addCalendarMonths(CalendarDate{Year: date.Year, Month: date.Month, Day: 1}, 1)
		}
		days--
	}
	for days < 0 {
		if date.Year == 1 && date.Month == 1 && date.Day == 1 {
			return date
		}
		if date.Day > 1 {
			date.Day--
		} else {
			previous := addCalendarMonths(CalendarDate{Year: date.Year, Month: date.Month, Day: 1}, -1)
			previous.Day = uint8(calendarDaysInMonth(previous.Year, int(previous.Month)))
			date = previous
		}
		days++
	}
	return date
}

func addCalendarMonths(date CalendarDate, months int) CalendarDate {
	date = normalizeCalendarDate(date)
	index := (date.Year-1)*12 + int(date.Month) - 1
	index = min(max(index+months, 0), 9999*12-1)
	year, month := index/12+1, index%12+1
	day := min(int(date.Day), calendarDaysInMonth(year, month))
	return CalendarDate{Year: year, Month: uint8(month), Day: uint8(day)}
}

func calendarWeekday(date CalendarDate) int {
	date = normalizeCalendarDate(date)
	offsets := [...]int{0, 3, 2, 5, 0, 3, 5, 1, 4, 6, 2, 4}
	year := date.Year
	if date.Month < 3 {
		year--
	}
	return (year + year/4 - year/100 + year/400 + offsets[int(date.Month)-1] + int(date.Day)) % 7
}

func calendarMonthOffset(first CalendarDate, start CalendarWeekStart) int {
	weekday := calendarWeekday(first)
	if start == CalendarWeekStartsSunday {
		return weekday
	}
	return (weekday + 6) % 7
}

func calendarWeekdayLabels(start CalendarWeekStart) []string {
	if start == CalendarWeekStartsSunday {
		return []string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"}
	}
	return []string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"}
}

func calendarDateForEvent(displayed, selected CalendarDate, event vt.Event) (CalendarDate, bool) {
	if event.Kind != vt.EventKey || event.Key.Action == vt.KeyRelease {
		return CalendarDate{}, false
	}
	modifiers := event.Key.Modifiers
	if modifiers.Alt || modifiers.Control || modifiers.Meta {
		return CalendarDate{}, false
	}
	displayed = NewCalendarDate(displayed.Year, int(displayed.Month), 1)
	selected = calendarActiveDate(displayed, selected)
	switch event.Key.Code {
	case vt.KeyLeft:
		return addCalendarDays(selected, -1), true
	case vt.KeyRight:
		return addCalendarDays(selected, 1), true
	case vt.KeyUp:
		return addCalendarDays(selected, -7), true
	case vt.KeyDown:
		return addCalendarDays(selected, 7), true
	case vt.KeyPageUp:
		return addCalendarMonths(selected, -1), true
	case vt.KeyPageDown:
		return addCalendarMonths(selected, 1), true
	case vt.KeyHome:
		return displayed, true
	case vt.KeyEnd:
		return CalendarDate{Year: displayed.Year, Month: displayed.Month, Day: uint8(calendarDaysInMonth(displayed.Year, int(displayed.Month)))}, true
	default:
		return CalendarDate{}, false
	}
}

func calendarDateID(root tui.NodeID, date CalendarDate) tui.NodeID {
	return tui.NewNodeID(fmt.Sprintf("%s/date/%04d-%02d-%02d", root, date.Year, date.Month, date.Day))
}
