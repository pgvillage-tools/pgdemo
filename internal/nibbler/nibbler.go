// Package nibbler draws the 8-bit animation of the About window: a small
// pixel critter, the Nibbler, walks up to a byte and eats four of its bits
// (a nibble), hops with joy and walks off again. Frame returns the picture at
// a moment in the loop; the GUI scales it up with sharp pixels.
package nibbler

import (
	"image"
	"image/color"
	"math"
)

// Scene size in (logical) pixels.
const (
	Width  = 120
	Height = 34
)

// Loop is the length of one round of the animation, in seconds.
const Loop = 9.0

var (
	orange = color.NRGBA{0xF2, 0x8C, 0x28, 0xFF} // the orange bar of the icon
	shade  = color.NRGBA{0xC8, 0x6A, 0x14, 0xFF}
	green  = color.NRGBA{0x3E, 0xA3, 0x6C, 0xFF} // the green bar of the icon
	white  = color.NRGBA{0xFF, 0xFF, 0xFF, 0xFF}
	black  = color.NRGBA{0x20, 0x20, 0x28, 0xFF}
	mouth  = color.NRGBA{0x8A, 0x1E, 0x2A, 0xFF}
	pink   = color.NRGBA{0xE8, 0x4A, 0x6A, 0xFF}
)

// sprites: one character per pixel; '.' is transparent.
var palette = map[byte]color.NRGBA{'O': orange, 'o': shade, 'G': green, 'W': white, 'K': black, 'R': mouth, 'P': pink}

var closedFace = []string{
	"...OOOOOO...",
	"..OOOOOOOO..",
	".OOOOOOOOOO.",
	".OWWOOOOWWO.",
	".OWKOOOOWKO.",
	".OOOOOOOOOO.",
	".OOKKKKKKOO.",
	".OOOOOOOOOO.",
	".oOOOOOOOOo.",
	"..oooooooo..",
}

var openFace = []string{
	"...OOOOOO...",
	"..OOOOOOOO..",
	".OWWOOOOWWO.",
	".OWKOOOOWKO.",
	".OOOOOOOOOO.",
	".OKKKKKKKKO.",
	".OKWRRRRWKO.",
	".OKRRRRRRKO.",
	".OKKKKKKKKO.",
	"..oooooooo..",
}

var feet = [][]string{
	{"..GG....GG..", ".GGG...GGG.."},
	{"...GG..GG...", "..GGG.GGG..."},
}

var heart = []string{
	".PP.PP.",
	"PPPPPPP",
	"PPPPPPP",
	".PPPPP.",
	"..PPP..",
	"...P...",
}

// digits 5×7
var digit = map[byte][]string{
	'0': {".###.", "#...#", "#..##", "#.#.#", "##..#", "#...#", ".###."},
	'1': {"..#..", ".##..", "#.#..", "..#..", "..#..", "..#..", "#####"},
}

const bits = "10110100"

const (
	ground  = 30         // y of the bottom of the critter's feet
	bitTop  = ground - 7 // digits stand on the ground
	firstX  = 50         // x of the first bit
	bitStep = 8          // distance between bits
	critW   = 12         // critter width
	speed   = 26.0       // walking speed, pixels per second
	chompT  = 0.75       // time to eat one bit
)

// timeline (seconds)
var (
	arriveAt = float64(firstX-2-critW+critW) / speed // walk from x=-critW to just before bit 0
	eatEnd   = arriveAt + 4*chompT + 3*float64(bitStep)/speed
	hopEnd   = eatEnd + 1.6
)

type state struct {
	x, y   float64 // top-left of the critter
	open   bool    // mouth open
	step   int     // feet frame
	eaten  int     // bits eaten so far (0..4)
	crumbs float64 // 0..1 progress of the crumbs of the bit being eaten, <0: none
	heart  float64 // 0..1 heart above the head, <0: none
	fade   float64 // 1 = bits fully visible again
}

func at(t float64) state {
	t = math.Mod(t, Loop)
	stopX := func(i int) float64 { return float64(firstX + i*bitStep - critW + 1) }
	s := state{crumbs: -1, heart: -1, fade: 0}
	walkFeet := func(t float64) int { return int(t*8) % 2 }
	switch {
	case t < arriveAt: // walk in
		s.x = -critW + (stopX(0)+critW)*t/arriveAt
		s.step = walkFeet(t)
	case t < eatEnd: // eat four bits
		u := t - arriveAt
		per := chompT + float64(bitStep)/speed
		i := int(u / per)
		if i > 3 {
			i = 3
		}
		v := u - float64(i)*per
		if v < chompT {
			s.x = stopX(i)
			s.open = int(v/(chompT/5))%2 == 0 && v < chompT*0.8
			s.eaten = i
			if v > chompT*0.3 {
				s.eaten = i + 1
				s.crumbs = (v - chompT*0.3) / (chompT * 0.7)
			}
		} else {
			w := v - chompT
			s.x = stopX(i) + w*speed
			s.step = walkFeet(t)
			s.eaten = i + 1
		}
	case t < hopEnd: // happy hops with a heart
		u := t - eatEnd
		s.x = stopX(3) + float64(bitStep)
		s.eaten = 4
		s.y = -math.Abs(math.Sin(u*math.Pi/0.4)) * 4
		s.heart = u / (hopEnd - eatEnd)
	default: // walk back out to the left; the bits come back
		u := t - hopEnd
		s.x = stopX(3) + float64(bitStep) - u*speed*1.6
		s.step = walkFeet(t)
		s.eaten = 4
		s.fade = math.Min(1, u/(Loop-hopEnd)*1.5)
	}
	return s
}

// Frame draws the scene at time t (seconds). ink is the colour of the bits
// (the theme's text colour, so it shows on light and dark backgrounds).
func Frame(t float64, ink color.Color) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, Width, Height))
	s := at(t)
	r, g, b, _ := ink.RGBA()
	inkC := color.NRGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 0xFF}

	// the byte: eaten bits are gone (or fade back in at the end)
	for i := 0; i < len(bits); i++ {
		c := inkC
		if i < s.eaten {
			if s.fade <= 0 {
				continue
			}
			c.A = uint8(255 * s.fade)
		}
		if i >= 4 && s.eaten == 4 && s.fade <= 0 {
			c = green // what is left: a nibble
		}
		x := firstX + i*bitStep
		if i == 4 {
			x += 3 // a small gap between the two nibbles
		}
		if i > 4 {
			x += 3
		}
		drawSprite(img, digit[bits[i]], x, bitTop, func(byte) color.NRGBA { return c })
	}

	// crumbs of the bit being eaten
	if s.crumbs >= 0 && s.eaten > 0 {
		bx := float64(firstX + (s.eaten-1)*bitStep + 2)
		for k, d := range [][2]float64{{-5, -6}, {4, -8}, {-2, -9}, {6, -4}, {1, -5}} {
			p := s.crumbs
			cx := bx + d[0]*p
			cy := float64(bitTop+3) + d[1]*p + 16*p*p // up, then down
			c := inkC
			if k%2 == 1 {
				c = orange
			}
			c.A = uint8(255 * (1 - p))
			set(img, int(cx), int(cy), c)
		}
	}

	// the Nibbler
	face := closedFace
	if s.open {
		face = openFace
	}
	x, y := int(math.Round(s.x)), int(math.Round(float64(ground-12)+s.y))
	drawSprite(img, face, x, y, pal)
	drawSprite(img, feet[s.step], x, y+len(face), pal)

	// a heart above the head
	if s.heart >= 0 {
		hc := func(ch byte) color.NRGBA {
			c := palette[ch]
			c.A = uint8(255 * math.Min(1, 3*(1-s.heart)))
			return c
		}
		drawSprite(img, heart, x+3, y-8-int(s.heart*6), hc)
	}
	return img
}

func pal(ch byte) color.NRGBA { return palette[ch] }

func drawSprite(img *image.NRGBA, sprite []string, x0, y0 int, col func(byte) color.NRGBA) {
	for dy, row := range sprite {
		for dx := 0; dx < len(row); dx++ {
			ch := row[dx]
			if ch == '.' {
				continue
			}
			set(img, x0+dx, y0+dy, col(ch))
		}
	}
}

func set(img *image.NRGBA, x, y int, c color.NRGBA) {
	if image.Pt(x, y).In(img.Rect) && c.A > 0 {
		img.SetNRGBA(x, y, c)
	}
}
