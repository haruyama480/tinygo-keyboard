//go:build tinygo

package keyboard

import (
	"machine"
)

// Keep each driven line as an output and drive it low before the next select.
// After it goes low, wait until the sense lines read idle. The diode blocks
// discharge, so a pulldown fall otherwise looks like the next key on that line.
const (
	matrixIdleSpins = 2000
	matrixIdleReads = 3
)

type MatrixKeyboard struct {
	State    []State
	Keys     [][]Keycode
	options  Options
	callback Callback

	Col          []machine.Pin
	Row          []machine.Pin
	cycleCounter []uint8
	debounce     uint8
}

func (d *Device) AddMatrixKeyboard(colPins, rowPins []machine.Pin, keys [][]Keycode, opt ...Option) *MatrixKeyboard {
	col := len(colPins)
	row := len(rowPins)
	state := make([]State, row*col)
	cycleCnt := make([]uint8, len(state))

	o := Options{}
	for _, f := range opt {
		f(&o)
	}

	keydef := make([][]Keycode, LayerCount)
	for l := 0; l < len(keydef); l++ {
		keydef[l] = make([]Keycode, len(state))
	}
	for l := 0; l < len(keys); l++ {
		for kc := 0; kc < len(keys[l]); kc++ {
			keydef[l][kc] = keys[l][kc]
		}
	}

	k := &MatrixKeyboard{
		Col:          colPins,
		Row:          rowPins,
		State:        state,
		Keys:         keydef,
		options:      o,
		callback:     func(layer, index int, state State) {},
		cycleCounter: cycleCnt,
		debounce:     8,
	}
	k.configurePins()

	d.kb = append(d.kb, k)
	return k
}

func (d *MatrixKeyboard) configurePins() {
	if d.options.InvertDiode {
		for _, p := range d.Col {
			p.Configure(machine.PinConfig{Mode: machine.PinInputPulldown})
		}
		for _, p := range d.Row {
			p.Configure(machine.PinConfig{Mode: machine.PinOutput})
			p.Low()
		}
		return
	}
	for _, p := range d.Row {
		p.Configure(machine.PinConfig{Mode: machine.PinInputPulldown})
	}
	for _, p := range d.Col {
		p.Configure(machine.PinConfig{Mode: machine.PinOutput})
		p.Low()
	}
}

func (d *MatrixKeyboard) SetCallback(fn Callback) {
	d.callback = fn
}

func (d *MatrixKeyboard) Callback(layer, index int, state State) {
	if d.callback != nil {
		d.callback(layer, index, state)
	}
}

func (d *MatrixKeyboard) Get() []State {
	if d.options.InvertDiode {
		d.scanRows()
	} else {
		d.scanCols()
	}
	return d.State
}

// scanCols drives one column high and reads every row, then drives that column low.
func (d *MatrixKeyboard) scanCols() {
	ncol := len(d.Col)
	for c := range d.Col {
		d.Col[c].High()
		for r := range d.Row {
			d.step(r*ncol+c, d.Row[r].Get())
		}
		d.Col[c].Low()
		d.waitIdle(d.Row)
	}
}

// scanRows is the InvertDiode path: drive one row high and read every column.
func (d *MatrixKeyboard) scanRows() {
	ncol := len(d.Col)
	for r := range d.Row {
		d.Row[r].High()
		for c := range d.Col {
			d.step(r*ncol+c, d.Col[c].Get())
		}
		d.Row[r].Low()
		d.waitIdle(d.Col)
	}
}

// waitIdle spins until pins read low, or until matrixIdleSpins runs out.
// A stuck line must not block the rest of the scan.
func (d *MatrixKeyboard) waitIdle(pins []machine.Pin) {
	idle := 0
	for i := 0; i < matrixIdleSpins; i++ {
		busy := false
		for _, p := range pins {
			if p.Get() {
				busy = true
				break
			}
		}
		if busy {
			idle = 0
			continue
		}
		idle++
		if idle == matrixIdleReads {
			return
		}
	}
}

func (d *MatrixKeyboard) step(idx int, current bool) {
	switch d.State[idx] {
	case None:
		if current {
			if d.cycleCounter[idx] >= d.debounce {
				d.State[idx] = NoneToPress
				d.cycleCounter[idx] = 0
			} else {
				d.cycleCounter[idx]++
			}
		} else {
			d.cycleCounter[idx] = 0
		}
	case NoneToPress:
		d.State[idx] = Press
	case Press:
		if current {
			d.cycleCounter[idx] = 0
		} else {
			if d.cycleCounter[idx] >= d.debounce {
				d.State[idx] = PressToRelease
				d.cycleCounter[idx] = 0
			} else {
				d.cycleCounter[idx]++
			}
		}
	case PressToRelease:
		d.State[idx] = None
	}
}

func (d *MatrixKeyboard) Key(layer, index int) Keycode {
	if layer >= LayerCount {
		return 0
	}
	if index >= len(d.Keys[layer]) {
		return 0
	}
	return d.Keys[layer][index]
}

func (d *MatrixKeyboard) SetKeycode(layer, index int, key Keycode) {
	if layer >= LayerCount {
		return
	}
	if index >= len(d.Keys[layer]) {
		return
	}
	d.Keys[layer][index] = key
}

func (d *MatrixKeyboard) GetKeyCount() int {
	return len(d.State)
}

func (d *MatrixKeyboard) Init() error {
	d.configurePins()
	return nil
}
