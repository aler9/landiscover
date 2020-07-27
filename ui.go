package main

import (
	"fmt"
	"github.com/nsf/termbox-go"
	"sort"
	"strings"
	"time"
)

func (ls *LanDiscover) ui() {
	uilib, err := NewUilib()
	if err != nil {
		panic(err)
	}
	defer uilib.Close()

	// ui state variables
	infoText := ""
	tableScrollX := 0
	tableScrollY := 0
	tableSortBy := "mac"
	tableSortAsc := true
	tableColumns := []UilibTableColumn{
		"last seen",
		"mac",
		"ip",
		"vendor",
		"dns",
		"nbns",
		"mdns",
	}
	var tableRows []UilibTableRow
	var selectables []string
	selection := ""

	regenSelectables := func() {
		sort.Slice(tableRows, func(i, j int) bool {
			n := 0
			switch tableSortBy {
			case "last seen":
				n = 0
			case "mac":
				n = 1
			case "ip":
				n = 2
			case "vendor":
				n = 3
			case "dns":
				n = 4
			case "nbns":
				n = 5
			case "mdns":
				n = 6
			}
			if tableRows[i].Cells[n] != tableRows[j].Cells[n] {
				if tableSortAsc {
					return tableRows[i].Cells[n] < tableRows[j].Cells[n]
				} else {
					return tableRows[i].Cells[n] > tableRows[j].Cells[n]
				}
			}
			return tableRows[i].Cells[2] < tableRows[j].Cells[2]
		})

		selectables = nil
		for _, col := range tableColumns {
			selectables = append(selectables, "col_"+string(col))
		}
		for _, row := range tableRows {
			selectables = append(selectables, "row_"+row.Id)
		}

		if selection == "" {
			selection = selectables[0]
		}
	}

	dataToUi := func() bool {
		ls.mutex.Lock()
		defer ls.mutex.Unlock()

		if ls.uiDrawQueued == false {
			return false
		}
		ls.uiDrawQueued = false

		tableRows = func() []UilibTableRow {
			var ret []UilibTableRow
			for _, n := range ls.nodes {
				row := UilibTableRow{
					Id: fmt.Sprintf("%s_%s", n.Mac.String(), n.Ip.String()),
					Cells: []string{
						n.LastSeen.Format("Jan 2 15:04:05"),
						n.Mac.String(),
						n.Ip.String(),
						macVendor(n.Mac),
						func() string {
							if n.Dns == "" {
								return "-"
							}
							return n.Dns
						}(),
						func() string {
							if n.Nbns == "" {
								return "-"
							}
							return n.Nbns
						}(),
						func() string {
							if n.Mdns == "" {
								return "-"
							}
							return n.Mdns
						}(),
					},
				}
				ret = append(ret, row)
			}
			return ret
		}()
		infoText = fmt.Sprintf("interface: %s%s    entries: %d",
			ls.intf.Name,
			func() string {
				if *argPassiveMode {
					return " (passive mode)"
				}
				return ""
			}(),
			len(tableRows))
		regenSelectables()

		return true
	}

	dataToUi()

	ticker := time.NewTicker(DRAW_PERIOD)
	defer ticker.Stop()
	tickerTerminate := make(chan struct{})
	tickerDone := make(chan struct{})
	go func() {
		defer func() { tickerDone <- struct{}{} }()
		for {
			select {
			case <-tickerTerminate:
				return
			case <-ticker.C:
				if dataToUi() == true {
					uilib.ForceDraw()
				}
			}
		}
	}()

	uilib.OnMoveX = func(value int) {
		tableScrollX += value
	}

	uilib.OnMoveY = func(value int) {
		oldIndex := func() int {
			for i, sel := range selectables {
				if sel == selection {
					return i
				}
			}
			return 0
		}()
		newIndex := oldIndex + value
		if newIndex >= len(selectables) {
			newIndex = len(selectables) - 1
		} else if newIndex < 0 {
			newIndex = 0
		}
		selection = selectables[newIndex]
	}

	uilib.OnEnter = func() {
		if strings.HasPrefix(selection, "col_") {
			for _, col := range tableColumns {
				if selection == "col_"+string(col) {
					if tableSortBy == string(col) {
						tableSortAsc = !tableSortAsc
					} else {
						tableSortBy = string(col)
						tableSortAsc = true
					}
					regenSelectables()
					break
				}
			}
		}
	}

	uilib.OnDraw = func(termWidth int, termHeight int) {
		uilib.DrawBorder(0, 0, termWidth, 3)

		uilib.DrawClippedText(1, termWidth-2, 1, 1, infoText,
			termbox.ColorWhite, termbox.ColorBlack)

		uilib.DrawBorder(0, 3, termWidth, termHeight-3)

		uilib.DrawScrollableTable(1, 4, termWidth-2, termHeight-5,
			selection, tableSortBy, tableSortAsc,
			tableColumns, tableRows, &tableScrollX, &tableScrollY)
	}

	uilib.Loop()

	tickerTerminate <- struct{}{}
	<-tickerDone
}

func (ls *LanDiscover) uiQueueDraw() {
	ls.uiDrawQueued = true
}
