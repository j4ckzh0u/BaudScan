package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func main() {
	a := newApp()
	mygo.App.WhenReady(func() {
		a.win = mygo.NewWindow(mygo.WindowOptions{
			Title:     "串口波特率扫描",
			Width:     1120,
			Height:    760,
			MinWidth:  980,
			MinHeight: 680,
			StateKey:  "main",
			Content:   ui.View(a.view),
		})
		a.win.OnClose(func(e *mygo.CloseEvent) {
			a.stopScan()
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
