package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
	"github.com/mayahiro/nagitui-go/widget"
)

type messageKind uint8

const (
	nameMessage messageKind = iota
	emailMessage
	roleMessage
	termsMessage
	dateMessage
	submitMessage
)

type message struct {
	kind  messageKind
	text  string
	index int
	value bool
	date  widget.CalendarDate
}

type registrationForm struct {
	name      string
	email     string
	role      int
	accepted  bool
	date      widget.CalendarDate
	submitted bool
}

func newRegistrationForm() *registrationForm {
	return &registrationForm{date: widget.NewCalendarDate(2026, 7, 18)}
}

func (*registrationForm) Init() tui.Effect[message] {
	return tui.NoneEffect[message]()
}

func (a *registrationForm) Update(received message) tui.Effect[message] {
	switch received.kind {
	case nameMessage:
		a.name = received.text
		a.submitted = false
	case emailMessage:
		a.email = received.text
		a.submitted = false
	case roleMessage:
		a.role = received.index
		a.submitted = false
	case termsMessage:
		a.accepted = received.value
		a.submitted = false
	case dateMessage:
		a.date = received.date
		a.submitted = false
	case submitMessage:
		a.submitted = len(a.validationErrors()) == 0
	}
	return tui.NoneEffect[message]()
}

func (*registrationForm) Subscriptions() tui.Subscription[message] {
	return tui.NoneSubscription[message]()
}

func (a *registrationForm) View(_ tui.ViewContext) tui.Node[message] {
	errors := a.validationErrors()
	errorNodes := make([]tui.Node[message], 0, max(len(errors), 1))
	if len(errors) == 0 {
		errorNodes = append(errorNodes, tui.StyledText[message]("All fields are valid", vt.Style{Bold: true}))
	} else {
		for _, validationError := range errors {
			errorNodes = append(errorNodes, tui.Text[message]("- "+validationError))
		}
	}
	if a.submitted {
		errorNodes = append(errorNodes, tui.StyledText[message]("Registration submitted", vt.Style{Bold: true}))
	}

	fields := tui.Column(
		fieldRow("Name", tui.StyledTextInput(
			tui.NewNodeID("name"), a.name, "Ada Lovelace",
			vt.Style{}, vt.Style{Dim: true},
			func(value string) message { return message{kind: nameMessage, text: value} },
		)),
		fieldRow("Email", tui.StyledTextInput(
			tui.NewNodeID("email"), a.email, "ada@example.com",
			vt.Style{}, vt.Style{Dim: true},
			func(value string) message { return message{kind: emailMessage, text: value} },
		)),
		tui.Row(
			tui.Text[message]("Role: ").WithLength(tui.Fixed(10)),
			widget.NewSelect(
				tui.NewNodeID("role"),
				[]string{"Viewer", "Operator", "Administrator"},
				a.role,
				func(index int) message { return message{kind: roleMessage, index: index} },
			).Node(),
		).WithLength(tui.Fixed(1)),
		widget.NewCheckbox(
			tui.NewNodeID("terms"),
			"I accept the usage policy",
			a.accepted,
			func(value bool) message { return message{kind: termsMessage, value: value} },
		).Node().WithLength(tui.Fixed(1)),
		tui.Gap[message](1),
		widget.NewButton(
			tui.NewNodeID("submit"),
			"Submit",
			func() message { return message{kind: submitMessage} },
		).Enabled(len(errors) == 0).Node(),
	)
	calendar := widget.NewCalendar(
		tui.NewNodeID("start-date"),
		a.date.Year,
		int(a.date.Month),
		a.date,
		func(date widget.CalendarDate) message { return message{kind: dateMessage, date: date} },
	).ShowAdjacent(true).Node()

	return tui.Panel(
		tui.Column(
			tui.Row(
				tui.Panel(fields, "Account").WithLength(tui.Flex(1)),
				tui.Panel(
					tui.Column(
						calendar,
						tui.Text[message](fmt.Sprintf("Selected: %04d-%02d-%02d", a.date.Year, a.date.Month, a.date.Day)),
					),
					"Start date",
				).WithLength(tui.Fixed(27)),
			).WithLength(tui.Flex(1)),
			tui.Panel(tui.Column(errorNodes...), "Validation").WithLength(tui.Fixed(6)),
			widget.NewHelp[message]([]widget.HelpBinding{
				widget.NewHelpBinding("Tab", "next field"),
				widget.NewHelpBinding("Shift-Tab", "previous field"),
				widget.NewHelpBinding("Arrows", "select"),
				widget.NewHelpBinding("Enter/Space", "activate"),
				widget.NewHelpBinding("Esc", "exit"),
			}).Node().WithLength(tui.Fixed(1)),
		),
		"Registration form",
	)
}

func fieldRow(label string, input tui.Node[message]) tui.Node[message] {
	return tui.Row(
		tui.Text[message](label+": ").WithLength(tui.Fixed(10)),
		input.WithLength(tui.Flex(1)),
	).WithLength(tui.Fixed(1))
}

func (a *registrationForm) validationErrors() []string {
	errors := make([]string, 0, 3)
	if strings.TrimSpace(a.name) == "" {
		errors = append(errors, "Name is required")
	}
	if !validEmail(a.email) {
		errors = append(errors, "Email must contain a local part, @, and domain")
	}
	if !a.accepted {
		errors = append(errors, "Usage policy acceptance is required")
	}
	return errors
}

func validEmail(value string) bool {
	value = strings.TrimSpace(value)
	at := strings.IndexByte(value, '@')
	return at > 0 && at < len(value)-1 && strings.Contains(value[at+1:], ".")
}

func mapEvent(event vt.Event) tui.EventAction[message] {
	switch {
	case event.Kind == vt.EventKey && event.Key.Code == vt.KeyEscape:
		return tui.ExitAction[message]()
	case event.Kind == vt.EventKey && event.Key.Code == vt.KeyCharacter && event.Key.Character == 'c' && event.Key.Modifiers.Control:
		return tui.ExitAction[message]()
	default:
		return tui.IgnoreAction[message]()
	}
}

func run() error {
	options := tui.DefaultTerminalOptions()
	options.FocusFirst = true
	mouseTracking := vt.MouseTrackingPress
	options.MouseTracking = &mouseTracking
	return tui.RunTerminal[message](newRegistrationForm(), options, mapEvent)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
