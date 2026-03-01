package tui

import "github.com/charmbracelet/bubbles/key"

type globalKeyMap struct {
	Tab1     key.Binding
	Tab2     key.Binding
	Tab3     key.Binding
	Tab4     key.Binding
	Tab5     key.Binding
	NextTab  key.Binding
	PrevTab  key.Binding
	Quit     key.Binding
	Help     key.Binding
	Escape   key.Binding
	Refresh  key.Binding
	Up       key.Binding
	Down     key.Binding
	Enter    key.Binding
	Done     key.Binding
	Delete   key.Binding
	Timer    key.Binding
	Add      key.Binding
	Edit     key.Binding
	Sync     key.Binding
	Search   key.Binding
	Force    key.Binding
	Clear    key.Binding
	Stop     key.Binding
	Snooze   key.Binding
	ViewTask key.Binding
	Report   key.Binding
}

var keys = globalKeyMap{
	Tab1:     key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "Timeline")),
	Tab2:     key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "Tasks")),
	Tab3:     key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "Schedule")),
	Tab4:     key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "Timer")),
	Tab5:     key.NewBinding(key.WithKeys("5"), key.WithHelp("5", "Config")),
	NextTab:  key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next tab")),
	PrevTab:  key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev tab")),
	Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Escape:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close/cancel")),
	Refresh:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	Up:       key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/up", "up")),
	Down:     key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/down", "down")),
	Enter:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
	Done:     key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "done")),
	Delete:   key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "delete")),
	Timer:    key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "start timer")),
	Add:      key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add")),
	Edit:     key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
	Sync:     key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sync")),
	Search:   key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	Force:    key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "force")),
	Clear:    key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "clear")),
	Stop:     key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "stop")),
	Snooze:   key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "snooze")),
	ViewTask: key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "view")),
	Report:   key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "report")),
}
