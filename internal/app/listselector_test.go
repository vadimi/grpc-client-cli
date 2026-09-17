package app

import (
	"image/color"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	enterKey = tea.KeyPressMsg{Code: tea.KeyEnter}
	ctrlCKey = tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	ctrlDKey = tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl}
	escKey   = tea.KeyPressMsg{Code: tea.KeyEscape}
	downKey  = tea.KeyPressMsg{Code: tea.KeyDown}
	tabKey   = tea.KeyPressMsg{Code: tea.KeyTab}
)

func whiteRGB() color.RGBA {
	return color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
}

// runeKey builds a printable character key press, e.g. runeKey("a").
func runeKey(s string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
}

func testSelectorOptions() SelectorOptions {
	return SelectorOptions{
		Title:   "Choose a service:",
		Options: []string{"alpha", "beta", "gamma"},
	}
}

func TestListSelectorView(t *testing.T) {
	s := newListSelector(testSelectorOptions())

	view := s.View()

	assert.Contains(t, view, "Choose a service:")
	assert.Contains(t, view, "alpha")
	assert.Contains(t, view, "beta")
	assert.Contains(t, view, "gamma")
	assert.Contains(t, view, cursorMarker, "the selected option should be marked with the cursor")
}

func TestListSelectorSelected(t *testing.T) {
	s := newListSelector(testSelectorOptions())

	choice, ok := s.Selected()
	require.True(t, ok)
	assert.Equal(t, "alpha", choice)

	s.Update(downKey)
	choice, ok = s.Selected()
	require.True(t, ok)
	assert.Equal(t, "beta", choice)
}

func TestListSelectorEmptySelected(t *testing.T) {
	s := newListSelector(SelectorOptions{Title: "Choose:"})

	_, ok := s.Selected()
	assert.False(t, ok)
}

func TestListSelectorEscapeClearsFilter(t *testing.T) {
	s := newListSelector(testSelectorOptions())

	s.Update(runeKey("/"))
	assert.Equal(t, list.Filtering, s.list.FilterState())
	assert.True(t, s.Filtering())

	// the first esc cancels the filter
	s.Update(escKey)
	assert.Equal(t, list.Unfiltered, s.list.FilterState())
	assert.False(t, s.Filtering())
}

func TestListSelectorPageSize(t *testing.T) {
	s := newListSelector(testSelectorOptions())
	assert.Equal(t, defaultPageSize, s.list.Paginator.PerPage)

	custom := newListSelector(SelectorOptions{Title: "Choose:", Options: []string{"a"}, PageSize: 5})
	assert.Equal(t, 5, custom.list.Paginator.PerPage)
}

func TestListSelectorSetSizeClampsHeight(t *testing.T) {
	s := newListSelector(testSelectorOptions())

	// a short terminal reduces the list height
	s.SetSize(100, 10)
	assert.Equal(t, 9, s.list.Height())

	// a tall terminal caps it at the page size
	s.SetSize(100, 100)
	assert.Equal(t, defaultPageSize+listChromeHeight, s.list.Height())
	assert.Equal(t, 100, s.list.Width())
}

func TestListSelectorApplyTheme(t *testing.T) {
	s := newListSelector(testSelectorOptions())

	lightBg := tea.BackgroundColorMsg{}
	lightBg.Color = whiteRGB()

	s.applyTheme(newTheme(false))

	assert.NotPanics(t, func() {
		_ = s.View()
	})
}
