package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/nsf/termbox-go"
)

const (
	drawPeriod = 1 * time.Second
)

func (p *program) ui() {
	uilib, err := newUilib()
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
	tableColumns := []uilibTableColumn{
		"last seen",
		"mac",
		"ip",
		"vendor",
		"dns",
		"nbns",
		"mdns",
	}
	var tableRows []uilibTableRow
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
		p.mutex.Lock()
		defer p.mutex.Unlock()

		if p.uiDrawQueued == false {
			return false
		}
		p.uiDrawQueued = false

		tableRows = func() []uilibTableRow {
			var ret []uilibTableRow
			for _, n := range p.nodes {
				row := uilibTableRow{
					Id: fmt.Sprintf("%s_%s", n.mac.String(), n.ip.String()),
					Cells: []string{
						n.lastSeen.Format("Jan 2 15:04:05"),
						n.mac.String(),
						n.ip.String(),
						macVendor(n.mac),
						func() string {
							if n.dns == "" {
								return "-"
							}
							return n.dns
						}(),
						func() string {
							if n.nbns == "" {
								return "-"
							}
							return n.nbns
						}(),
						func() string {
							if n.mdns == "" {
								return "-"
							}
							return n.mdns
						}(),
					},
				}
				ret = append(ret, row)
			}
			return ret
		}()
		infoText = fmt.Sprintf("interface: %s%s    entries: %d",
			p.intf.Name,
			func() string {
				if p.passiveMode {
					return " (passive mode)"
				}
				return ""
			}(),
			len(tableRows))
		regenSelectables()

		return true
	}

	dataToUi()

	ticker := time.NewTicker(drawPeriod)
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

func (p *program) uiQueueDraw() {
	p.uiDrawQueued = true
}
