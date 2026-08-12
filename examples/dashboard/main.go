package main

import (
	"fmt"
	"os"
	"time"

	"github.com/mayahiro/nagi-go/vt"
	"github.com/mayahiro/nagitui-go"
	"github.com/mayahiro/nagitui-go/widget"
)

type message struct {
	kind       string
	service    int
	generation uint64
}

type toastRecord struct {
	generation uint64
	label      string
}

type dashboard struct {
	service         int
	toastGeneration uint64
	toast           *toastRecord
}

func (*dashboard) Init() tui.Effect[message] {
	return tui.NoneEffect[message]()
}

func (a *dashboard) Update(received message) tui.Effect[message] {
	switch received.kind {
	case "select-service":
		a.service = received.service
		a.toastGeneration++
		generation := a.toastGeneration
		service := []string{"api", "worker", "database"}[received.service]
		a.toast = &toastRecord{generation: generation, label: "Selected " + service}
		return tui.AfterEffect(3*time.Second, message{kind: "expire-toast", generation: generation})
	case "dismiss-toast", "expire-toast":
		if a.toast != nil && a.toast.generation == received.generation {
			a.toast = nil
		}
	}
	return tui.NoneEffect[message]()
}

func (*dashboard) Subscriptions() tui.Subscription[message] {
	return tui.NoneSubscription[message]()
}

func (a *dashboard) View(context tui.ViewContext) tui.Node[message] {
	requests := tui.Panel(
		tui.Column(
			tui.StyledText[message]("12.8k req/min", vt.Style{Bold: true}),
			widget.NewSparkline[message]([]uint64{4, 6, 5, 9, 8, 12, 11, 15, 13, 18, 16, 20}, 24).Node(),
		),
		"Requests",
	).WithLength(tui.Flex(1))
	latency := tui.Panel(
		tui.Column(
			tui.StyledText[message]("p95 84 ms", vt.Style{Bold: true}),
			widget.NewSparkline[message]([]uint64{70, 74, 69, 77, 81, 75, 79, 84, 82, 80, 84}, 24).
				Bounds(60, 100).
				Node(),
		),
		"Latency",
	).WithLength(tui.Flex(1))
	errors := tui.Panel(
		tui.Column(
			tui.StyledText[message]("18% budget used", vt.Style{Bold: true}),
			widget.NewProgress[message](18, 100, 24).Node(),
		),
		"Errors",
	).WithLength(tui.Flex(1))

	traffic := widget.NewChart[message]([]widget.ChartSeries{
		widget.NewChartSeries("requests", []widget.ChartPoint{
			{X: 0, Y: 3}, {X: 1, Y: 4}, {X: 2, Y: 4}, {X: 3, Y: 7},
			{X: 4, Y: 6}, {X: 5, Y: 9}, {X: 6, Y: 8}, {X: 7, Y: 11},
		}),
	}, 34, 8).Bounds(0, 7, 0, 12).
		WidthProfile(context.WidthProfile).
		Node()
	resources := widget.NewBarChart[message]([]widget.BarChartBar{
		widget.NewBarChartBar("api", 68),
		widget.NewBarChartBar("worker", 47),
		widget.NewBarChartBar("database", 81),
	}, 16).Maximum(100).
		WidthProfile(context.WidthProfile).
		Node()

	services := widget.NewTable(
		tui.NewNodeID("services"),
		[]widget.TableColumn{
			widget.NewTableColumn("Service", tui.Flex(1)),
			widget.NewTableColumn("Status", tui.Fixed(10)),
			widget.NewTableColumn("RPS", tui.Fixed(8)),
			widget.NewTableColumn("p95", tui.Fixed(8)),
		},
		[]widget.TableRow{
			widget.NewTableRow(tui.NewNodeID("service-api"), []string{"api", "healthy", "8,420", "72 ms"}),
			widget.NewTableRow(tui.NewNodeID("service-worker"), []string{"worker", "healthy", "3,910", "84 ms"}),
			widget.NewTableRow(tui.NewNodeID("service-database"), []string{"database", "warning", "470", "41 ms"}),
		},
		a.service,
		func(index int) message { return message{kind: "select-service", service: index} },
	).ColumnAlignment(2, tui.AlignEnd).
		ColumnAlignment(3, tui.AlignEnd).
		Viewport(tui.NewNodeID("service-rows"), tui.Fixed(4)).
		Node()

	activity := tui.Text[message]("idle")
	if a.toast != nil {
		activity = widget.NewSpinner[message](a.toastGeneration).Label("updating").Node()
	}
	status := widget.NewStatusBar([]widget.StatusBarSlot[message]{
		widget.NewStatusBarSlot(tui.Text[message]("connected")).Priority(widget.StatusBarHigh),
		widget.NewStatusBarSlot(activity).
			Placement(tui.ResponsiveRowCenter).
			Priority(widget.StatusBarCritical),
		widget.NewStatusBarSlot(tui.Text[message]("3 services | 18% budget")).
			Placement(tui.ResponsiveRowEnd),
	}).Node()

	base := tui.Column(
		tui.StyledText[message]("Operations dashboard", vt.Style{Bold: true}).WithLength(tui.Fixed(1)),
		tui.Row(requests, latency, errors).WithLength(tui.Fixed(5)),
		tui.Row(
			tui.Panel(traffic, "Traffic").WithLength(tui.Flex(1)),
			tui.Panel(resources, "CPU by service").WithLength(tui.Flex(1)),
		).WithLength(tui.Fixed(10)),
		tui.Panel(services, "Services").WithLength(tui.Flex(1)),
		widget.NewHelp[message]([]widget.HelpBinding{
			widget.NewHelpBinding("Tab", "focus"),
			widget.NewHelpBinding("Up/Down", "select service"),
			widget.NewHelpBinding("Esc", "exit"),
		}).WidthProfile(context.WidthProfile).Node().WithLength(tui.Fixed(1)),
		status,
	)

	var toasts []widget.Toast[message]
	if a.toast != nil {
		record := *a.toast
		toasts = append(toasts,
			widget.NewToast(tui.NewNodeID("service-toast"), func() tui.Node[message] {
				return tui.Text[message](record.label)
			}).
				Tone(widget.ToastInfo).
				OnDismiss(tui.NewNodeID("service-toast-dismiss"), func() message {
					return message{kind: "dismiss-toast", generation: record.generation}
				}),
		)
	}
	return widget.NewToastRegion(base, toasts).Node()
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
	return tui.RunTerminal[message](&dashboard{}, options, mapEvent)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
