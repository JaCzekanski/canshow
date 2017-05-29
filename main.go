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
)

type Frame struct {
	addr     uint32
	data     []byte
	prevData []byte
}

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
		frame := parseFrame(l)
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
		//time.Sleep(time.Millisecond * 20)
	}
}

func render() {
	title := termui.NewPar("canshow, Author: Jakub Czekanski")
	title.Height = 3
	title.BorderLabel = "Info"

	list := termui.NewList()
	list.BorderLabel = "Frames"
	list.Height = 40

	termui.Body.AddRows(
		termui.NewRow(termui.NewCol(6, 0, title)),
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
