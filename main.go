package main

import (
	"bytes"
	"os"

	"github.com/knq/escpos"
	png2escpos "github.com/mugli/png2escpos/escpos"
)

func main() {
	f, err := os.OpenFile("/dev/usb/lp0", os.O_RDWR, 0)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	// w := bufio.NewWriter(f)
	p := escpos.New(f)

	buf := bytes.Buffer{}
	if err := png2escpos.PrintImage("qr.png", &buf); err != nil {
		panic(err)
	}

	p.Init()
	p.SetAlign("center")
	p.WriteRaw(buf.Bytes())

	// p.SetSmooth(1)
	p.SetFontSize(2, 3)
	p.SetFont("B")
	fonts := []string{"A", "B", "C"}
	for _, font := range fonts {
		p.SetFont(font)
		p.FormfeedN(5)
		p.Write("Lorem ipsum dolor sit amet, consetetur sadipscing elitr, sed diam nonumy eirmod tempor invidunt ut labore et dolore magna aliquyam erat, sed diam voluptua.")
		p.FormfeedN(5)
	}

	p.FormfeedN(5)

	p.Cut()
	p.End()

	// w.Flush()
}
