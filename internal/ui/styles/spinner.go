package styles

import (
	"charm.land/bubbles/v2/spinner"
)

func NewSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = s.Style.Foreground(ColorPrimary())
	return s
}
