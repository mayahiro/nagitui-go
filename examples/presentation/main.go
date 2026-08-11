package main

import (
	"fmt"

	"github.com/mayahiro/nagi-go/content"
	"github.com/mayahiro/nagi-go/vt"
	tui "github.com/mayahiro/nagitui-go"
)

func main() {
	heading := mustRole("heading")
	muted := mustState("muted")
	element := mustRoles(content.NewParagraph(nil), heading)

	headingRule := tui.NewPresentationRule(
		tui.RolePresentationSelector(heading),
		(tui.PresentationDeclaration{}).WithTextStyle(
			(tui.TextStyleDeclaration{}).
				WithForeground(tui.SetDeclarationValue(vt.IndexedColor(6))).
				WithBold(tui.SetDeclarationValue(true)),
		),
	)
	mutedRule := mustRequired(
		tui.NewPresentationRule(
			tui.RolePresentationSelector(heading),
			(tui.PresentationDeclaration{}).WithTextStyle(
				(tui.TextStyleDeclaration{}).
					WithForeground(tui.InitialDeclarationValue[vt.Color]()).
					WithBold(tui.SetDeclarationValue(false)).
					WithDim(tui.SetDeclarationValue(true)),
			),
		),
		muted,
	)
	sheet := tui.NewPresentationSheet([]tui.PresentationRule{headingRule, mutedRule})

	computed := sheet.Resolve(element, vt.Style{}, []tui.PresentationState{muted})
	style := computed.Style()
	fmt.Printf("display=%s\n", displayName(computed.Display()))
	fmt.Printf("foreground=%s\n", colorName(style.Foreground))
	fmt.Printf("bold=%t dim=%t\n", style.Bold, style.Dim)
}

func mustRole(value string) content.Role {
	role, err := content.NewRole(value)
	if err != nil {
		panic(err)
	}
	return role
}

func mustState(value string) tui.PresentationState {
	state, err := tui.NewPresentationState(value)
	if err != nil {
		panic(err)
	}
	return state
}

func mustRoles(element content.Element, roles ...content.Role) content.Element {
	element, err := element.WithRoles(roles)
	if err != nil {
		panic(err)
	}
	return element
}

func mustRequired(rule tui.PresentationRule, states ...tui.PresentationState) tui.PresentationRule {
	rule, err := rule.Requiring(states...)
	if err != nil {
		panic(err)
	}
	return rule
}

func displayName(display tui.PresentationDisplay) string {
	switch display {
	case tui.PresentationDisplayFlow:
		return "flow"
	case tui.PresentationDisplayParagraph:
		return "paragraph"
	case tui.PresentationDisplaySequence:
		return "sequence"
	default:
		return "inline"
	}
}

func colorName(color vt.Color) string {
	switch color.Kind() {
	case vt.ColorIndexed:
		return "indexed"
	case vt.ColorRGB:
		return "rgb"
	default:
		return "default"
	}
}
