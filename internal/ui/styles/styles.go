package styles

import (
	"fmt"
	"image/color"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
)

func ColorPrimary() color.Color {
	return lipgloss.Color("#009977")
}

func ColorGray() color.Color {
	return lipgloss.Color("8")
}

func AnswerTextStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ColorPrimary()).MarginBottom(1)
}

func RenderAnswerText(s string) string {
	return AnswerTextStyle().Render(fmt.Sprintf("> %s", s))
}

func ErrorTextStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Red).MarginLeft(2)
}

func RenderErrorText(s string) string {
	return ErrorTextStyle().Render(fmt.Sprintf("Error: %s", s))
}

type SelectListStyles struct {
	Item         lipgloss.Style
	DisabledItem lipgloss.Style
	SelectedItem lipgloss.Style
	pagination   lipgloss.Style
	help         lipgloss.Style
}

func NewSelectListStyles() *SelectListStyles {
	var style SelectListStyles
	style.Item = lipgloss.NewStyle().PaddingLeft(4)
	style.DisabledItem = style.Item.Foreground(ColorGray()).Strikethrough(true)
	style.SelectedItem = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("#009977"))
	style.pagination = list.DefaultStyles(true).PaginationStyle.PaddingLeft(4)
	style.help = list.DefaultStyles(true).HelpStyle.PaddingLeft(4).PaddingBottom(1)
	return &style
}

func NewTextInputStyles() textinput.Styles {
	styles := textinput.DefaultDarkStyles()
	styles.Blurred.Text = styles.Blurred.Text.Underline(true).UnderlineSpaces(true)
	styles.Focused.Text = styles.Focused.Text.Underline(true).UnderlineSpaces(true)
	return styles
}
