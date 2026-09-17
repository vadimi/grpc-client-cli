package app

import (
	"fmt"
	"io"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	// defaultPageSize is the number of options visible at once. It matches
	// the page size of the previous survey-based prompt.
	defaultPageSize = 20

	// listChromeHeight reserves list rows for the filter input and the
	// pagination sections so that pageSize options remain visible.
	listChromeHeight = 2

	// defaultListWidth is used until the terminal reports its real width.
	defaultListWidth = 80
)

// SelectorOptions configures a listSelector.
type SelectorOptions struct {
	// Title is shown above the option list, e.g. "Choose a service:".
	Title string
	// Options holds the selectable choices in display order.
	Options []string
	// PageSize is the number of options visible at once. It defaults to
	// 20 when zero.
	PageSize int
}

// listSelector is a single-choice list component built on the bubbles
// list. The containing model routes key messages to it for navigation,
// filtering and pagination, and interprets the selection keys (enter, esc)
// around it.
type listSelector struct {
	list      list.Model
	title     string
	maxHeight int
	theme     theme
}

func newListSelector(opts SelectorOptions) *listSelector {
	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}

	items := make([]list.Item, len(opts.Options))
	for i, opt := range opts.Options {
		items[i] = optionItem(opt)
	}

	// Dark styles are used until the terminal reports its actual background
	// color; see applyTheme.
	maxHeight := pageSize + listChromeHeight
	l := list.New(items, newSelectorDelegate(newTheme(true)), defaultListWidth, maxHeight)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	// The prompt must only end via an explicit choice or an interrupt, so
	// the built-in quit keybindings are disabled.
	l.DisableQuitKeybindings()

	return &listSelector{
		list:      l,
		title:     opts.Title,
		maxHeight: maxHeight,
		theme:     newTheme(true),
	}
}

// Update forwards a message to the list for navigation, filtering and
// pagination.
func (s *listSelector) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)
	return cmd
}

// View renders the question line and the option list.
func (s *listSelector) View() string {
	question := s.theme.question.Render("?") + " " + s.theme.title.Render(s.title)
	return question + "\n" + s.list.View()
}

// SetSize resizes the list. The height never exceeds the page size but is
// reduced on short terminals.
func (s *listSelector) SetSize(width, height int) {
	// one row for the question line
	listHeight := max(min(height-1, s.maxHeight), 1)
	s.list.SetSize(width, listHeight)
}

// Selected returns the highlighted option, if any.
func (s *listSelector) Selected() (string, bool) {
	item, ok := s.list.SelectedItem().(optionItem)
	if !ok {
		return "", false
	}

	return string(item), true
}

// Filtering reports whether a filter is set or being edited. The
// containing model consults it before interpreting esc.
func (s *listSelector) Filtering() bool {
	return s.list.FilterState() != list.Unfiltered
}

// applyTheme switches the list and item styles to a light or dark palette.
func (s *listSelector) applyTheme(t theme) {
	s.theme = t
	s.list.Styles = t.listStyles()
	s.list.SetDelegate(newSelectorDelegate(t))
}

// optionItem is a single selector entry.
type optionItem string

// Title implements list.DefaultItem.
func (o optionItem) Title() string { return string(o) }

// Description implements list.DefaultItem.
func (o optionItem) Description() string { return "" }

// FilterValue implements list.Item.
func (o optionItem) FilterValue() string { return string(o) }

// cursorMarker marks the selected option, matching the look of the
// previous survey-based prompt. Unselected options are indented by the
// normal item style so they line up with the marker.
const cursorMarker = "→"

// selectorDelegate renders single-line options with a cursor marker on the
// selected row.
type selectorDelegate struct {
	styles list.DefaultItemStyles
}

func newSelectorDelegate(t theme) selectorDelegate {
	styles := t.itemStyles()
	// The default selected style draws a left border as the cursor; use the
	// arrow marker instead and keep the adaptive colors.
	styles.SelectedTitle = lipgloss.NewStyle().
		Foreground(styles.SelectedTitle.GetForeground()).
		PaddingLeft(0)

	return selectorDelegate{styles: styles}
}

// Height implements list.ItemDelegate.
func (d selectorDelegate) Height() int { return 1 }

// Spacing implements list.ItemDelegate.
func (d selectorDelegate) Spacing() int { return 0 }

// Update implements list.ItemDelegate.
func (d selectorDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

// Render implements list.ItemDelegate.
func (d selectorDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(list.DefaultItem)
	if !ok {
		return
	}

	if m.Width() <= 0 {
		// short-circuit, mirroring the default delegate
		return
	}

	title := it.Title()

	// Prevent text from exceeding the list width.
	textwidth := m.Width() - d.styles.NormalTitle.GetPaddingLeft() - d.styles.NormalTitle.GetPaddingRight()
	title = ansi.Truncate(title, textwidth, "…")

	var (
		isSelected  = index == m.Index()
		emptyFilter = m.FilterState() == list.Filtering && m.FilterValue() == ""
		isFiltered  = m.FilterState() == list.Filtering || m.FilterState() == list.FilterApplied
	)

	var matchedRunes []int
	if isFiltered && index < len(m.VisibleItems()) {
		matchedRunes = m.MatchesForItem(index)
	}

	switch {
	case emptyFilter:
		title = d.styles.DimmedTitle.Render(title)
	case isSelected && m.FilterState() != list.Filtering:
		if isFiltered {
			unmatched := d.styles.SelectedTitle.Inline(true)
			matched := unmatched.Inherit(d.styles.FilterMatch)
			title = lipgloss.StyleRunes(title, matchedRunes, matched, unmatched)
		}
		title = d.styles.SelectedTitle.Render(cursorMarker + " " + title)
	default:
		if isFiltered {
			unmatched := d.styles.NormalTitle.Inline(true)
			matched := unmatched.Inherit(d.styles.FilterMatch)
			title = lipgloss.StyleRunes(title, matchedRunes, matched, unmatched)
		}
		title = d.styles.NormalTitle.Render(title)
	}

	//nolint:errcheck // mirrors the default delegate
	fmt.Fprint(w, title)
}
