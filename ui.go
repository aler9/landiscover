package main

import (
	"bytes"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/nsf/termbox-go"
)

const (
	drawPeriod      = 1 * time.Second
	uiColumnPadding = 2
)

type uiTableColumn string

type uiTableRow struct {
	id    string
	cells []string
}

type termboxReq struct {
	tevt termbox.Event
	done chan struct{}
}

type ui struct {
	p            *program
	infoText     string
	tableScrollX int
	tableScrollY int
	tableSortBy  string
	tableSortAsc bool
	tableColumns []uiTableColumn
	tableRows    []uiTableRow
	selectables  []string
	selection    string

	termbox   chan termboxReq
	terminate chan struct{}
	done      chan struct{}
}

func newUI(p *program) error {
	err := termbox.Init()
	if err != nil {
		return err
	}

	ui := &ui{
		p:            p,
		infoText:     "",
		tableSortBy:  "mac",
		tableSortAsc: true,
		tableColumns: []uiTableColumn{
			"last seen",
			"mac",
			"ip",
			"vendor",
			"dns",
			"nbns",
			"mdns",
		},
		termbox:   make(chan termboxReq),
		terminate: make(chan struct{}),
		done:      make(chan struct{}),
	}

	p.ui = ui
	return nil
}

func (u *ui) run() {
	u.draw()

	termboxDone := make(chan struct{})
	go func() {
		defer close(termboxDone)
		for {
			tevt := termbox.PollEvent()
			if tevt.Type == termbox.EventInterrupt {
				break
			}
			done := make(chan struct{})
			u.termbox <- termboxReq{tevt, done}
			<-done
		}
	}()

	periodicRedrawTicker := time.NewTicker(drawPeriod)
	defer periodicRedrawTicker.Stop()

outer:
	for {
		select {
		case <-periodicRedrawTicker.C:
			u.draw()

		case req := <-u.termbox:
			switch req.tevt.Type {
			case termbox.EventKey:
				switch req.tevt.Key {
				case termbox.KeyEsc, termbox.KeyCtrlC, termbox.KeyCtrlX:
					close(u.p.terminate)

				case termbox.KeyArrowLeft:
					u.tableScrollX++
					u.draw()

				case termbox.KeyArrowRight:
					u.tableScrollX--
					u.draw()

				case termbox.KeyArrowUp:
					if len(u.selectables) > 0 {
						oldIndex := func() int {
							for i, sel := range u.selectables {
								if sel == u.selection {
									return i
								}
							}
							return 0
						}()
						newIndex := oldIndex - 1
						if newIndex >= len(u.selectables) {
							newIndex = len(u.selectables) - 1
						} else if newIndex < 0 {
							newIndex = 0
						}
						u.selection = u.selectables[newIndex]
					}
					u.draw()

				case termbox.KeyArrowDown:
					if len(u.selectables) > 0 {
						oldIndex := func() int {
							for i, sel := range u.selectables {
								if sel == u.selection {
									return i
								}
							}
							return 0
						}()
						newIndex := oldIndex + 1
						if newIndex >= len(u.selectables) {
							newIndex = len(u.selectables) - 1
						} else if newIndex < 0 {
							newIndex = 0
						}
						u.selection = u.selectables[newIndex]
					}
					u.draw()

				case termbox.KeyPgup:
					_, termHeight := termbox.Size()
					u.onMoveY(-(termHeight - 9))
					u.draw()

				case termbox.KeyPgdn:
					_, termHeight := termbox.Size()
					u.onMoveY(termHeight - 9)
					u.draw()

				case termbox.KeyEnter, termbox.KeySpace:
					if strings.HasPrefix(u.selection, "col_") {
						for _, col := range u.tableColumns {
							if u.selection == "col_"+string(col) {
								if u.tableSortBy == string(col) {
									u.tableSortAsc = !u.tableSortAsc
								} else {
									u.tableSortBy = string(col)
									u.tableSortAsc = true
								}
								break
							}
						}
					}
					u.draw()

				default:
					switch req.tevt.Ch {
					case 'q', 'Q':
						close(u.p.terminate)
					}
				}

			case termbox.EventResize:
				u.draw()

			case termbox.EventError:
				panic(req.tevt.Err)
			}
			close(req.done)

		case <-u.terminate:
			break outer
		}
	}

	go func() {
		for {
			_, ok := <-u.termbox
			if !ok {
				return
			}
		}
	}()

	termbox.Interrupt()
	<-termboxDone
	termbox.Close()

	close(u.termbox)
	close(u.done)
}

func (u *ui) close() {
	close(u.terminate)
	<-u.done
}

func (u *ui) onMoveY(value int) {
	oldIndex := func() int {
		for i, sel := range u.selectables {
			if sel == u.selection {
				return i
			}
		}
		return 0
	}()
	newIndex := oldIndex + value
	if newIndex >= len(u.selectables) {
		newIndex = len(u.selectables) - 1
	} else if newIndex < 0 {
		newIndex = 0
	}
	u.selection = u.selectables[newIndex]
}

func (u *ui) draw() {
	u.gatherData()

	termbox.Clear(termbox.ColorBlack, termbox.ColorBlack) //nolint:errcheck
	termWidth, termHeight := termbox.Size()               // must be called after Clear()

	u.drawRect(0, 0, termWidth, 3)

	u.drawClippedText(1, termWidth-2, 1, 1, u.infoText,
		termbox.ColorWhite, termbox.ColorBlack)

	u.drawRect(0, 3, termWidth, termHeight-3)

	u.drawScrollableTable(1, 4, termWidth-2, termHeight-5,
		u.selection, u.tableSortBy, u.tableSortAsc,
		u.tableColumns, u.tableRows, &u.tableScrollX, &u.tableScrollY)

	termbox.Flush() //nolint:errcheck
}

func (u *ui) gatherData() {
	resNodes := make(chan map[nodeKey]*node)
	done := make(chan struct{})
	u.p.uiGetData <- uiGetDataReq{
		resNodes: resNodes,
		done:     done,
	}
	nodes := <-resNodes

	// program is terminating
	if nodes == nil {
		return
	}

	u.tableRows = func() []uiTableRow {
		var ret []uiTableRow
		for _, n := range nodes {
			row := uiTableRow{
				id: fmt.Sprintf("%s_%s", n.mac.String(), n.ip.String()),
				cells: []string{
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

	close(done)

	u.infoText = fmt.Sprintf("interface: %s%s    entries: %d    last update: %s",
		u.p.intf.Name,
		func() string {
			if u.p.passiveMode {
				return " (passive mode)"
			}
			return ""
		}(),
		len(u.tableRows),
		time.Now().Format("Jan 2 15:04:05"))

	sort.Slice(u.tableRows, func(i, j int) bool {
		n := 0
		switch u.tableSortBy {
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

		if u.tableSortBy == "ip" {
			if u.tableRows[i].cells[n] != u.tableRows[j].cells[n] {
				ipa := net.ParseIP(u.tableRows[i].cells[n])
				ipb := net.ParseIP(u.tableRows[j].cells[n])

				if u.tableSortAsc {
					return bytes.Compare(ipa, ipb) < 0
				}
				return bytes.Compare(ipa, ipb) >= 0
			}

		} else {
			if u.tableRows[i].cells[n] != u.tableRows[j].cells[n] {
				if u.tableSortAsc {
					return u.tableRows[i].cells[n] < u.tableRows[j].cells[n]
				}
				return u.tableRows[i].cells[n] > u.tableRows[j].cells[n]
			}
		}

		return u.tableRows[i].cells[2] < u.tableRows[j].cells[2]
	})

	u.selectables = nil
	for _, col := range u.tableColumns {
		u.selectables = append(u.selectables, "col_"+string(col))
	}
	for _, row := range u.tableRows {
		u.selectables = append(u.selectables, "row_"+row.id)
	}

	if u.selection == "" {
		u.selection = u.selectables[0]
	}
}

func (u *ui) drawRect(startX int, startY int, width int, height int) {
	endX := startX + width - 1
	endY := startY + height - 1

	termbox.SetCell(startX, startY, 0x250C, termbox.ColorWhite, termbox.ColorBlack)
	termbox.SetCell(endX, startY, 0x2510, termbox.ColorWhite, termbox.ColorBlack)
	termbox.SetCell(startX, endY, 0x2514, termbox.ColorWhite, termbox.ColorBlack)
	termbox.SetCell(endX, endY, 0x2518, termbox.ColorWhite, termbox.ColorBlack)

	for x := startX + 1; x < endX; x++ {
		termbox.SetCell(x, startY, 0x2500, termbox.ColorWhite, termbox.ColorBlack)
		termbox.SetCell(x, endY, 0x2500, termbox.ColorWhite, termbox.ColorBlack)
	}
	for y := startY + 1; y < endY; y++ {
		termbox.SetCell(startX, y, 0x2502, termbox.ColorWhite, termbox.ColorBlack)
		termbox.SetCell(endX, y, 0x2502, termbox.ColorWhite, termbox.ColorBlack)
	}
}

func (u *ui) drawClippedText(startX, endX, x, y int, text string, fg, bg termbox.Attribute) {
	for _, r := range text {
		if x >= startX && x <= endX {
			termbox.SetCell(x, y, r, fg, bg)
		}
		x++
	}
}

func (u *ui) drawScrollableTable(startX int, startY int, width int, height int,
	selection string, sortBy string, sortAsc bool, columns []uiTableColumn,
	rows []uiTableRow, scrollX *int, scrollY *int) {
	endX := startX + width - 1
	endY := startY + height - 1

	// compute columns width
	colWidths := make([]int, len(columns))
	for i, col := range columns {
		width := len(col) + 2 // leave additional space for order arrow
		if colWidths[i] < width {
			colWidths[i] = width
		}
	}
	for _, row := range rows {
		for i, cell := range row.cells {
			if colWidths[i] < len(cell) {
				colWidths[i] = len(cell)
			}
		}
	}

	// get table width
	tableWidth := 0
	for i := range columns {
		tableWidth += colWidths[i] + uiColumnPadding
	}

	// get table height
	tableHeight := 2 + len(rows)

	// limit scroll
	xMin := width - tableWidth - 1
	if *scrollX < xMin {
		*scrollX = xMin
	}
	xMax := 0
	if *scrollX > xMax {
		*scrollX = xMax
	}
	selectionY := func() int {
		if strings.HasPrefix(selection, "row_") {
			for i, row := range rows {
				if selection == "row_"+row.id {
					return i
				}
			}
		}
		return 0
	}()
	yMax := height - 4 - selectionY
	if *scrollY > yMax {
		*scrollY = yMax
	}
	yMin := -selectionY
	if *scrollY < yMin {
		*scrollY = yMin
	}

	// draw scrollbars
	u.drawScrollbar(true, endX, startY, height, tableHeight, *scrollY)
	u.drawScrollbar(false, endY, startX, width, tableWidth, *scrollX)

	// reduce space
	endX--
	endY--

	// draw columns
	x := startX + *scrollX
	for i, col := range columns {
		fg := termbox.ColorWhite
		bg := termbox.ColorBlack
		if selection == "col_"+string(col) {
			fg = termbox.ColorBlack
			bg = termbox.ColorWhite
		}

		text := string(col)
		if sortBy == string(col) {
			if sortAsc {
				text += " " + string(rune(0x25B2))
			} else {
				text += " " + string(rune(0x25BC))
			}
		}
		u.drawClippedText(startX, endX, x, startY, text, fg, bg)
		x += colWidths[i] + uiColumnPadding
	}

	// draw rows
	y := startY + 2 + *scrollY
	for _, row := range rows {
		fg := termbox.ColorWhite
		bg := termbox.ColorBlack
		if selection == "row_"+row.id {
			fg = termbox.ColorBlack
			bg = termbox.ColorWhite
		}

		if y >= (startY+2) && y <= endY {
			x := startX + *scrollX
			for i, cell := range row.cells {
				u.drawClippedText(startX, endX, x, y, cell, fg, bg)
				x += colWidths[i] + uiColumnPadding
			}
		}
		y++
	}
}

func (u *ui) drawScrollbar(vertical bool, fixedCoord int, start int,
	screenSize int, pageSize int, cur int) {
	scrollbarMaxSize := screenSize - 1
	scrollbarSize := scrollbarMaxSize
	if pageSize > scrollbarMaxSize {
		scrollbarSize = scrollbarSize * scrollbarMaxSize / pageSize
	}

	scrollZone := (scrollbarMaxSize - scrollbarSize)
	min := scrollbarMaxSize - pageSize
	if min != 0 {
		start += (scrollZone - scrollZone*(cur-min)/(-min))
	}

	if vertical {
		for y := start; y < (start + scrollbarSize); y++ {
			termbox.SetCell(fixedCoord, y, 0x2588, termbox.ColorGreen, termbox.ColorBlack)
		}
	} else {
		for x := start; x < (start + scrollbarSize); x++ {
			termbox.SetCell(x, fixedCoord, 0x2585, termbox.ColorGreen, termbox.ColorBlack)
		}
	}
}
