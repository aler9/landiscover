package main

import (
    "strings"
    "github.com/nsf/termbox-go"
)

const (
    COLUMN_PADDING = 2
)

type UilibTableColumn string

type UilibTableRow struct {
    Id      string
    Cells   []string
}

type Uilib struct {
    OnMoveX     func(int)
    OnMoveY     func(int)
    OnEnter     func()
    OnDraw      func(int, int)
}

func NewUilib() (*Uilib,error) {
    err := termbox.Init()
    if err != nil {
        return nil, err
    }

    u := &Uilib{}
    return u, nil
}

func (u *Uilib) Close() {
    termbox.Close()
}

func (u *Uilib) Loop() {
    doDraw := func() {
        termbox.Clear(termbox.ColorBlack, termbox.ColorBlack)
        termWidth,termHeight := termbox.Size() // must be called after Clear()
        u.OnDraw(termWidth, termHeight)
        termbox.Flush()
    }

    doDraw()

    mainloop: for {
        evt := termbox.PollEvent()
        switch evt.Type {
        case termbox.EventKey:
            switch evt.Key {
            case termbox.KeyEsc, termbox.KeyCtrlC, termbox.KeyCtrlX:
                break mainloop

            case termbox.KeyArrowLeft:
                u.OnMoveX(+1)
                doDraw()

            case termbox.KeyArrowRight:
                u.OnMoveX(-1)
                doDraw()

            case termbox.KeyArrowUp:
                u.OnMoveY(-1)
                doDraw()

            case termbox.KeyArrowDown:
                u.OnMoveY(+1)
                doDraw()

            case termbox.KeyPgup:
                _,termHeight := termbox.Size()
                u.OnMoveY(- (termHeight - 9))
                doDraw()

            case termbox.KeyPgdn:
                _,termHeight := termbox.Size()
                u.OnMoveY(termHeight - 9)
                doDraw()

            case termbox.KeyEnter:
                u.OnEnter()
                doDraw()

            default:
                switch evt.Ch {
                case 'q', 'Q':
                    break mainloop
                }
            }

        case termbox.EventInterrupt: // termbox.Interrupt() has been called
            doDraw()

        case termbox.EventResize:
            doDraw()

        case termbox.EventError:
            panic(evt.Err)
        }
    }
}

func (u *Uilib) ForceDraw() {
    termbox.Interrupt()
}

func (u *Uilib) DrawBorder(startX, startY, width, height int) {
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

func (u *Uilib) DrawClippedText(startX, endX, x, y int, text string, fg, bg termbox.Attribute) {
    for _,r := range text {
        if x >= startX && x <= endX {
            termbox.SetCell(x, y, r, fg, bg)
        }
        x++
    }
}

func (u *Uilib) DrawScrollableTable(startX int, startY int, width int, height int,
    selection string, sortBy string, sortAsc bool, columns []UilibTableColumn,
    rows []UilibTableRow, scrollX *int, scrollY *int) {
    endX := startX + width - 1
    endY := startY + height - 1

    // compute columns width
    colWidths := make([]int, len(columns))
    for i,col := range columns {
        width := len(col) + 2 // leave additional space for order arrow
        if colWidths[i] < width {
            colWidths[i] = width
        }
    }
    for _,row := range rows {
        for i,cell := range row.Cells {
            if colWidths[i] < len(cell) {
                colWidths[i] = len(cell)
            }
        }
    }

    // get table width
    tableWidth := 0
    for i,_ := range columns {
        tableWidth += colWidths[i] + COLUMN_PADDING
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
            for i,row := range rows {
                if selection == "row_" + row.Id {
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
    yMin := - selectionY
    if *scrollY < yMin {
        *scrollY = yMin
    }

    // draw scrollbars
    u.DrawScrollbar(true, endX, startY, height, tableHeight, *scrollY)
    u.DrawScrollbar(false, endY, startX, width, tableWidth, *scrollX)

    // reduce space
    endX -= 1
    endY -= 1
    width -= 1
    height -= 1

    // draw columns
    x := startX + *scrollX
    for i,col := range columns {
        fg := termbox.ColorWhite
        bg := termbox.ColorBlack
        if selection == "col_" + string(col) {
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
        u.DrawClippedText(startX, endX, x, startY, text, fg, bg)
        x += colWidths[i] + COLUMN_PADDING
    }

    // draw rows
    y := startY + 2 + *scrollY
    for _,row := range rows {
        fg := termbox.ColorWhite
        bg := termbox.ColorBlack
        if selection == "row_" + row.Id {
            fg = termbox.ColorBlack
            bg = termbox.ColorWhite
        }

        if y >= (startY + 2) && y <= endY {
            x := startX + *scrollX
            for i,cell := range row.Cells {
                u.DrawClippedText(startX, endX, x, y, cell, fg, bg)
                x += colWidths[i] + COLUMN_PADDING
            }
        }
        y += 1
    }
}

func (u *Uilib) DrawScrollbar(vertical bool, fixedCoord int, start int,
    screenSize int, pageSize int, cur int) {
    scrollbarMaxSize := screenSize - 1
    scrollbarSize := scrollbarMaxSize
    if pageSize > scrollbarMaxSize {
        scrollbarSize = scrollbarSize * scrollbarMaxSize / pageSize
    }

    scrollZone := (scrollbarMaxSize - scrollbarSize)
    min := scrollbarMaxSize - pageSize
    if min != 0 {
        start += (scrollZone - scrollZone * (cur - min) / (- min))
    }

    if vertical {
        for y := start; y < (start + scrollbarSize); y++ {
            termbox.SetCell(fixedCoord, y, 0x2588, termbox.ColorYellow, termbox.ColorBlack)
        }
    } else {
        for x := start; x < (start + scrollbarSize); x++ {
            termbox.SetCell(x, fixedCoord, 0x2585, termbox.ColorYellow, termbox.ColorBlack)
        }
    }
}
