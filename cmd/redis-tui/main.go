package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ajxv/redis-tui/internal/tui"
)

func run() error {
	// parse cmdline args
	// define flags
	host := flag.String("host", "localhost:6379", "Redis Server host: <host:port>")
	//parse flags
	flag.Parse()

	// define menu items
	items := []list.Item{
		tui.NewListItem("SET", "Set a key-value pair"),
		tui.NewListItem("GET", "Get the value of a key"),
		tui.NewListItem("HSET", "Set a hash field"),
		tui.NewListItem("HGET", "Get the value of a hash field"),
		tui.NewListItem("EXPLORE", "Browse keys and values"),
	}

	// initialize the menu list
	menuList := list.New(items, list.NewDefaultDelegate(), 0, 0)
	menuList.Title = "Redis TUI"

	// initialize fields list
	fieldsList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	fieldsList.Title = "Select a field"

	// intialize key list
	keyList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	keyList.Title = "Select a key"

	// initialize the input
	input := textinput.New()

	// initialize viewport
	vp := viewport.New(0, 0)

	// define initialModel
	initialModel := tui.Model{
		CurrentState: tui.StateMenu,
		MenuList:     menuList,
		FieldsList:   fieldsList,
		KeyList:      keyList,
		Input:        input,
		ViewPort:     vp,
		RedisAddress: *host,
	}

	// start BubbleTea program
	p := tea.NewProgram(initialModel)
	if _, err := p.Run(); err != nil {
		fmt.Printf("An error occured: %v", err)
		return err
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}
