package tui

import "github.com/charmbracelet/bubbles/key"

type globalKeyMap struct {
	Tab1       key.Binding
	Tab2       key.Binding
	Tab3       key.Binding
	Tab4       key.Binding
	NextTab    key.Binding
	PrevTab    key.Binding
	Quit       key.Binding
	Help       key.Binding
	Escape     key.Binding
	Refresh    key.Binding
	Up         key.Binding
	Down       key.Binding
	Enter      key.Binding
	Done       key.Binding
	Delete     key.Binding
	Timer      key.Binding
	Add        key.Binding
	Edit       key.Binding
	Sync       key.Binding
	Search     key.Binding
	Force      key.Binding
	Clear      key.Binding
	Stop       key.Binding
	Abort      key.Binding
	Snooze     key.Binding
	ViewTask   key.Binding
	WeekToggle key.Binding
	DatePrev   key.Binding
	DateNext   key.Binding
	GoToday    key.Binding
	Move       key.Binding
	Comment    key.Binding
	EditStart  key.Binding
	Interrupt    key.Binding
	TogglePanel  key.Binding
	ViewScore    key.Binding
	Standup      key.Binding
}

var keys = globalKeyMap{
	Tab1:     key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "Tasks")),
	Tab2:     key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "Schedule")),
	Tab3:     key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "Worklog")),
	Tab4:     key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "Config")),
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
	Abort:    key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "abort")),
	Snooze:   key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "snooze")),
	ViewTask: key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "view")),
	WeekToggle: key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "toggle week")),
	DatePrev:   key.NewBinding(key.WithKeys("h", "left"), key.WithHelp("h/left", "prev date")),
	DateNext:   key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l/right", "next date")),
	GoToday:    key.NewBinding(key.WithKeys("T"), key.WithHelp("T", "go to today")),
	Move:       key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "move")),
	Comment:    key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "comment")),
	EditStart:  key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit start")),
	Interrupt:   key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "interrupt")),
	TogglePanel: key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "toggle panel")),
	ViewScore:   key.NewBinding(key.WithKeys("V"), key.WithHelp("V", "score breakdown")),
	Standup:     key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "standup")),
}
