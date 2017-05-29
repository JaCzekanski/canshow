package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"github.com/gizak/termui"
	"os"
	"strconv"
	"strings"
	"sync"
)

type Frame struct {
	addr     uint32
	data     []byte
	prevData []byte
}

const (
	RDS_ADDR       = 0x0a194005
	RDS_CHAR_COUNT = 8
	FREQUENCY_ADDR = 0x0a114005
	BUTTONS_ADDR   = 0x06354000
	DATE_ADDR      = 0x0c214003
	BREAK_ADDR     = 0x063d4000
)

func parseFrame(l string) Frame {
	var frame Frame
	p := strings.Fields(l)
	addr, _ := strconv.ParseUint(p[1], 16, 32)
	frame.addr = uint32(addr)
	len, _ := strconv.Atoi(strings.Trim(p[2], "[]"))
	for i := 0; i < len; i++ {
		b, _ := strconv.ParseUint(p[3+i], 16, 32)
		frame.data = append(frame.data, byte(b))
		frame.prevData = append(frame.data, byte(b))
	}
	return frame
}

func parseFrameDump(l string) Frame {
	var frame Frame
	p := strings.Fields(l)
	xx := strings.Split(p[2], "#")
	addr, _ := strconv.ParseUint(xx[0], 16, 32)
	frame.addr = uint32(addr)
	bytes, _ := hex.DecodeString(xx[1])
	frame.data = bytes
	frame.prevData = bytes
	return frame
}

var frames = struct {
	sync.RWMutex
	m []Frame
}{}

var status string

func reader(mutex chan bool) {
	reader := bufio.NewReader(os.Stdin)
	for {
		l, err := reader.ReadString('\n')
		if err != nil {
			status = "!!!EOF!!!"
			break
		}
		frame := parseFrameDump(l)
		frames.Lock()
		found := false
		for idx, _ := range frames.m {
			if frames.m[idx].addr == frame.addr {
				frames.m[idx].prevData = frames.m[idx].data
				frames.m[idx].data = frame.data
				found = true
			}
		}
		if !found {
			frames.m = append(frames.m, frame)
		}
		frames.Unlock()
	}
}

func decodeRDS(data []byte) string {
	text := ""
	var ch uint
	for i := 0; i < 6*RDS_CHAR_COUNT; i++ {
		if i != 0 && (i%6 == 0) {
			if ch >= 12 && ch <= 12+25 {
				text += string('A' + ch - 12)
			} else if ch == 40 {
				text += string(' ')
			} else {
				text += string(' ')
			}
			ch = 0
		}
		bit := data[i/8] & (0x80 >> uint(i%8))
		ch <<= 1
		if bit != 0 {
			ch |= 1
		}
	}
	return text
}

func decodeFrequency(data []byte) string {
	if data[0] == 0x46 {
		var freq uint16
		freq = (uint16(data[1]) << 8) | uint16(data[2])
		return fmt.Sprintf("%d.%d", freq/10, freq%10)
	} else if data[0] == 0xc3 {
		text := fmt.Sprintf("Track: %d", data[3])
		if data[4] == 0x02 {
			text += "Play"
		} else if data[4] == 0x01 {
			text += "Pause"
		}
		return text
	}
	return ""
}

func getButton(data byte, which byte, button string) string {
	if data&which != 0 {
		return button
	}
	return " "
}

func decodeButtons(data []byte) string {
	text := ""
	text += getButton(data[0], 0x80, "+")
	text += getButton(data[0], 0x40, "-")
	text += getButton(data[0], 0x10, "^")
	text += getButton(data[0], 0x08, "v")
	text += getButton(data[0], 0x04, "O")
	text += getButton(data[0], 0x20, "E")
	text += getButton(data[1], 0x80, "M")
	text += getButton(data[1], 0x40, "W")
	return text
}

func bcd(data byte) string {
	return string('0'+((data&0xf0)>>4)) + string('0'+(data&0x0f))
}

func decodeDate(data []byte) string {
	return fmt.Sprintf("%s:%s %s/%s/%s%s",
		bcd(data[0]),
		bcd(data[1]),
		bcd(data[2]),
		bcd(data[3]),
		bcd(data[4]),
		bcd(data[5]))
}

func decodeBreak(data []byte) string {
	if data[1] & 1 == 0 {
		return "BREAK"
	}
	return ""
}

func render(mutex chan bool) {
	title := termui.NewPar("")
	title.Height = 3
	title.BorderLabel = "Info"

	list := termui.NewList()
	list.BorderLabel = "Frames"
	list.Height = 40

	lcd := termui.NewPar("")
	lcd.Height = 3
	lcd.BorderLabel = "LCD"

	buttons := termui.NewPar("")
	buttons.Height = 3
	buttons.BorderLabel = "Buttons"

	date := termui.NewPar("")
	date.Height = 3
	date.BorderLabel = "Date"

	break_ := termui.NewPar("")
	break_.Height = 3
	break_.BorderLabel = "Break"

	termui.Body.AddRows(
		termui.NewRow(termui.NewCol(7, 0, title)),
		termui.NewRow(termui.NewCol(2, 0, lcd), termui.NewCol(2, 0, buttons), termui.NewCol(3, 0, date), termui.NewCol(1, 0, break_)),
		termui.NewRow(termui.NewCol(10, 0, list)))
	var strs []string
	for {
		frames.RLock()
		strs = make([]string, 0)
		for _, frame := range frames.m {
			line := ""
			line += fmt.Sprintf("0x%08x: ", frame.addr)
			pad := 3*8 + 2
			for i, d := range frame.data {
				color := "fg-white"
				if frame.prevData[i] != d {
					color = "fg-green"
				}
				line += fmt.Sprintf("[%02x ](%s)", d, color)
				pad -= 3
			}
			for i := 0; i < pad; i++ {
				line += " "
			}
			for i, d := range frame.data {
				color := "fg-white"
				if frame.prevData[i] != d {
					color = "fg-green"
				}
				c := '.'
				if d >= 0x20 && d < 0x7f {
					c = rune(d)
				}
				line += fmt.Sprintf("[%c](%s)", c, color)
			}
			strs = append(strs, line)
		}
		frames.RUnlock()

		lcdRds := ""
		lcdFreq := ""

		for _, frame := range frames.m {
			if frame.addr == RDS_ADDR {
				lcdRds = decodeRDS(frame.data)
			}
			if frame.addr == FREQUENCY_ADDR {
				lcdFreq = decodeFrequency(frame.data)
			}
			if frame.addr == BUTTONS_ADDR {
				buttons.Text = decodeButtons(frame.data)
			}
			if frame.addr == DATE_ADDR {
				date.Text = decodeDate(frame.data)
			}
			if frame.addr == BREAK_ADDR {
				break_.Text = decodeBreak(frame.data)
			}
		}

		title.Text = "canshow, Author: Jakub Czekanski  " + status

		lcd.Text = lcdRds + " " + lcdFreq

		list.Items = strs
		termui.Body.Align()
		termui.Render(termui.Body)
	}
}

func main() {
	err := termui.Init()
	if err != nil {
		panic(err)
	}
	defer termui.Close()

	mutex := make(chan bool)
	go reader(mutex)
	go render(mutex)

	termui.Handle("/sys/kbd/q", func(termui.Event) {
		termui.StopLoop()
	})
	termui.Loop()
}
