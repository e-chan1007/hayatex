package styles

import "charm.land/bubbles/v2/progress"

func NewProgressBar() progress.Model {
	p := progress.New(progress.WithColors(ColorPrimary(), ColorPrimary()))
	return p
}
