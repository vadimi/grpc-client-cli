package app

import (
	"image/color"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textarea"
	"charm.land/lipgloss/v2"
)

// theme holds the styling of the interactive application. It is rebuilt
// when the terminal reports its background color; until then the dark
// variant is used.
type theme struct {
	isDark bool

	// question renders the question mark that prefixes prompt and summary
	// lines. ANSI color 6 (cyan) is readable on both light and dark
	// terminals.
	question lipgloss.Style
	// title renders prompt titles.
	title lipgloss.Style
	// header renders the breadcrumb line with the selected service and
	// method.
	header lipgloss.Style
	// headerDim renders the inactive parts of the breadcrumb line.
	headerDim lipgloss.Style
	// footer renders help and status lines.
	footer lipgloss.Style
	// notice renders inline hints and errors.
	notice lipgloss.Style
	// error renders error messages.
	err lipgloss.Style

	// badge renders the application name at the left of the header bar.
	badge lipgloss.Style
	// statusIdle, statusBusy, statusOK and statusErr render the status
	// badge at the right of the header bar.
	statusIdle, statusBusy, statusOK, statusErr lipgloss.Style
	// key and keyDesc render the key and the description of a footer hint.
	key, keyDesc lipgloss.Style
	// rule draws the horizontal rules around the active screen.
	rule lipgloss.Style
}

func newTheme(isDark bool) theme {
	light := lipgloss.LightDark(isDark)
	pill := func(bg, fg color.Color) lipgloss.Style {
		return lipgloss.NewStyle().Bold(true).Padding(0, 1).Background(bg).Foreground(fg)
	}
	white := lipgloss.Color("#ffffff")
	dim := light(lipgloss.Color("#A49FA5"), lipgloss.Color("#777777"))

	return theme{
		isDark:   isDark,
		question: lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true),
		title:    lipgloss.NewStyle().Bold(true),
		header: lipgloss.NewStyle().Bold(true).Foreground(
			lipgloss.LightDark(isDark)(lipgloss.Color("#1a1a1a"), lipgloss.Color("#dddddd")),
		),
		headerDim: lipgloss.NewStyle().Foreground(
			lipgloss.LightDark(isDark)(lipgloss.Color("#A49FA5"), lipgloss.Color("#777777")),
		),
		footer: lipgloss.NewStyle().Foreground(
			lipgloss.LightDark(isDark)(lipgloss.Color("#A49FA5"), lipgloss.Color("#777777")),
		),
		notice: lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
		err:    lipgloss.NewStyle().Foreground(lipgloss.Color("9")),

		badge:      pill(light(lipgloss.Color("#0b6e99"), lipgloss.Color("#1f6f8b")), white),
		statusIdle: pill(light(lipgloss.Color("#8a8a8a"), lipgloss.Color("#4a4a4a")), white),
		statusBusy: pill(lipgloss.Color("#b45f06"), white),
		statusOK:   pill(lipgloss.Color("#2f7d32"), white),
		statusErr:  pill(lipgloss.Color("#c62828"), white),
		key:        lipgloss.NewStyle().Bold(true).Foreground(light(lipgloss.Color("#1a1a1a"), lipgloss.Color("#dddddd"))),
		keyDesc:    lipgloss.NewStyle().Foreground(dim),
		rule:       lipgloss.NewStyle().Foreground(light(lipgloss.Color("#b0b0b0"), lipgloss.Color("#555555"))),
	}
}

// listStyles returns the list component styles for the theme.
func (t theme) listStyles() list.Styles {
	return list.DefaultStyles(t.isDark)
}

// itemStyles returns the list item styles used by the selector delegate.
func (t theme) itemStyles() list.DefaultItemStyles {
	return list.NewDefaultItemStyles(t.isDark)
}

// textAreaStyles returns the textarea styles for the theme.
func (t theme) textAreaStyles() textarea.Styles {
	styles := textarea.DefaultStyles(t.isDark)
	// The editor is the only input on the screen; drop the prompt and line
	// number decorations.
	styles.Focused.Prompt = lipgloss.NewStyle()
	styles.Focused.CursorLineNumber = lipgloss.NewStyle()
	styles.Focused.LineNumber = lipgloss.NewStyle()
	styles.Blurred = styles.Focused
	// a steady cursor reads better in a message editor than a blinking one
	styles.Cursor.Blink = false
	return styles
}
