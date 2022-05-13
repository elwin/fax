package main

import (
	"os"

	"github.com/knq/escpos"
)

func main() {
	f, err := os.OpenFile("/dev/usb/lp0", os.O_RDWR, 0)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	// w := bufio.NewWriter(f)
	p := escpos.New(f)

	p.Init()
	p.SetSmooth(1)
	p.SetFontSize(2, 3)
	p.SetFont("A")
	p.Write("test ")
	p.SetFont("B")
	p.Write("test2 ")
	p.SetFont("C")
	p.Write("test3 ")
	p.Formfeed()

	p.SetFont("B")
	p.SetFontSize(1, 1)

	p.SetEmphasize(1)
	p.Write("halle")
	p.Formfeed()

	p.SetUnderline(1)
	p.SetFontSize(4, 4)
	p.Write("halle")

	p.SetReverse(1)
	p.SetFontSize(2, 4)
	p.Write("halle")
	p.Formfeed()

	p.SetFont("C")
	p.SetFontSize(8, 8)
	p.Write("halle")
	p.FormfeedN(5)

	p.Cut()
	p.End()

	// w.Flush()
}
