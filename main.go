package main

import (
	"bufio"
	"fmt"
	"github.com/gizak/termui"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"encoding/hex"
)

type Frame struct {
	addr     uint32
	data     []byte
	prevData []byte
}

const (
	LCD_ADDR = 0x0a194005
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

func reader() {
	reader := bufio.NewReader(os.Stdin)
	for {
		l, err := reader.ReadString('\n')
		if err != nil {

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
		time.Sleep(time.Millisecond * 20)
	}
}

func decodeLcd(data []byte) string {
	text := ""
	var ch uint
	for i:=0; i<6*6; i++ {
		bit := data[i/8] & (0x80 >> uint(i%8))
		if bit != 0 {
			ch = ch | 1
		}
		ch = ch << 1
		if i != 0 && (i % 6 == 0) {
			text += string('A' + ((ch>>1) - 12))
			ch = 0
		}
	}
/*	z := new(big.Int)
	z.SetBytes(data)
	for i := 8; i>=2; i-- {
		var ch uint 
		ch = 0
		for c := 5; c>=0; c-- {
			ch = (ch<<1) | z.Bit(i*6+c)
		}
		text += string('A' + ch - 0x0c)
	}*/
/*	hex1 := ((data[0] & 0xfc) >> 2)
	hex2 := ((((data[0] << 6) | (data[1] >> 2)) & 0xfc) >> 2)
	text := ""
	text += string('A' + hex1 - 0x0c)
	text += string('A' + hex2 - 0x0c) */
	return text
}

func render() {
	title := termui.NewPar("canshow, Author: Jakub Czekanski")
	title.Height = 3
	title.BorderLabel = "Info"

	list := termui.NewList()
	list.BorderLabel = "Frames"
	list.Height = 40

	lcd := termui.NewPar("---")
	lcd.Height = 3
	lcd.BorderLabel = "LCD"

	termui.Body.AddRows(
		termui.NewRow(termui.NewCol(6, 0, title)),
		termui.NewRow(termui.NewCol(3, 0, lcd)),
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

	
		// DECODE LCD
		for _, frame := range frames.m {
			if frame.addr != LCD_ADDR {
				continue
			}
			lcd.Text = decodeLcd(frame.data)
		}	
		

		list.Items = strs
		termui.Body.Align()
		termui.Render(termui.Body)
		time.Sleep(time.Millisecond * 20)
	}
}

func main() {
	err := termui.Init()
	if err != nil {
		panic(err)
	}
	defer termui.Close()

	go reader()
	go render()

	termui.Handle("/sys/kbd/q", func(termui.Event) {
		termui.StopLoop()
	})
	termui.Loop()
}
