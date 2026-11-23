package theme

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Lift shifts a color one chrome step away from its background: lighter on
// dark palettes, darker on light palettes. It keeps the relative steps of the
// design language the same across every theme.
func Lift(c Color, amount float64) Color {
	v := strings.TrimPrefix(string(c), "#")
	if len(v) != 6 {
		return c
	}
	channels := make([]float64, 3)
	for i := range channels {
		n, err := strconv.ParseUint(v[i*2:i*2+2], 16, 8)
		if err != nil {
			return c
		}
		channels[i] = float64(n)
	}

	luma := 0.2126*channels[0] + 0.7152*channels[1] + 0.0722*channels[2]
	target := 255.0
	if luma >= 128 {
		target = 0
	}
	for i := range channels {
		channels[i] += (target - channels[i]) * amount
	}
	return Color(fmt.Sprintf("#%02x%02x%02x",
		uint8(channels[0]), uint8(channels[1]), uint8(channels[2])))
}

// Contrast returns the WCAG contrast ratio between two theme colors.
func Contrast(a, b Color) float64 {
	la, oka := luminance(a)
	lb, okb := luminance(b)
	if !oka || !okb {
		return 0
	}
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// ReadableOn picks the candidate with the highest contrast against surface.
// It is used where a derived surface (a footer band, for example) has no
// dedicated foreground token of its own.
func ReadableOn(surface Color, candidates ...Color) Color {
	best, bestRatio := Color(""), 0.0
	for _, c := range candidates {
		if ratio := Contrast(surface, c); ratio > bestRatio {
			best, bestRatio = c, ratio
		}
	}
	return best
}

// AdjustForSurface raises or lowers a color's lightness (keeping hue and
// saturation) until it reaches min contrast against surface. It is applied
// at render time so a single rule works for both light and dark palettes.
func AdjustForSurface(c, surface Color, min float64) Color {
	rgb, ok := colorRGB(c)
	surfaceLum, ok2 := luminance(surface)
	if !ok || !ok2 || min <= 0 {
		return c
	}
	h, l, s := rgbToHLS(rgb)
	for i := 0; i <= 100; i++ {
		frac := float64(i) / 100
		var next float64
		if surfaceLum < 0.5 {
			next = l + (0.95-l)*frac
		} else {
			next = l - (l-0.05)*frac
		}
		candidate := hlsToRGB(h, next, s)
		if Contrast(surface, candidate) >= min {
			return candidate
		}
	}
	return c
}

// AdjustSyntax returns a theme's syntax palette tuned for its executable
// surface, where code text actually sits.
func AdjustSyntax(th Theme) Syntax {
	surface := th.ExecPanel
	return Syntax{
		Comment:     AdjustForSurface(th.Syntax.Comment, surface, 4.5),
		Keyword:     AdjustForSurface(th.Syntax.Keyword, surface, 4.5),
		String:      AdjustForSurface(th.Syntax.String, surface, 4.5),
		Function:    AdjustForSurface(th.Syntax.Function, surface, 4.5),
		Number:      AdjustForSurface(th.Syntax.Number, surface, 4.5),
		Type:        AdjustForSurface(th.Syntax.Type, surface, 4.5),
		Operator:    AdjustForSurface(th.Syntax.Operator, surface, 4.5),
		Punctuation: AdjustForSurface(th.Syntax.Punctuation, surface, 4.5),
		Constant:    AdjustForSurface(th.Syntax.Constant, surface, 4.5),
	}
}

func colorRGB(c Color) ([3]float64, bool) {
	v := strings.TrimPrefix(string(c), "#")
	if len(v) != 6 {
		return [3]float64{}, false
	}
	var out [3]float64
	for i := range out {
		n, err := strconv.ParseUint(v[i*2:i*2+2], 16, 8)
		if err != nil {
			return [3]float64{}, false
		}
		out[i] = float64(n) / 255
	}
	return out, true
}

func rgbToHLS(rgb [3]float64) (h, l, s float64) {
	r, g, b := rgb[0], rgb[1], rgb[2]
	max, min := r, r
	if g > max {
		max = g
	}
	if b > max {
		max = b
	}
	if g < min {
		min = g
	}
	if b < min {
		min = b
	}
	l = (max + min) / 2
	if max == min {
		return 0, l, 0
	}
	d := max - min
	if l > 0.5 {
		s = d / (2 - max - min)
	} else {
		s = d / (max + min)
	}
	switch max {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h /= 6
	return h, l, s
}

func hlsToRGB(h, l, s float64) Color {
	if s == 0 {
		return rgbHex(l, l, l)
	}
	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q
	return rgbHex(hueToRGB(p, q, h+1.0/3), hueToRGB(p, q, h), hueToRGB(p, q, h-1.0/3))
}

func hueToRGB(p, q, t float64) float64 {
	if t < 0 {
		t++
	}
	if t > 1 {
		t--
	}
	switch {
	case t < 1.0/6:
		return p + (q-p)*6*t
	case t < 1.0/2:
		return q
	case t < 2.0/3:
		return p + (q-p)*(2.0/3-t)*6
	default:
		return p
	}
}

func rgbHex(r, g, b float64) Color {
	return Color(fmt.Sprintf("#%02x%02x%02x",
		byte(math.Round(r*255)), byte(math.Round(g*255)), byte(math.Round(b*255))))
}

func luminance(c Color) (float64, bool) {
	v := strings.TrimPrefix(string(c), "#")
	if len(v) != 6 {
		return 0, false
	}
	channels := make([]float64, 3)
	for i := range channels {
		n, err := strconv.ParseUint(v[i*2:i*2+2], 16, 8)
		if err != nil {
			return 0, false
		}
		s := float64(n) / 255
		if s <= 0.03928 {
			channels[i] = s / 12.92
		} else {
			channels[i] = math.Pow((s+0.055)/1.055, 2.4)
		}
	}
	return 0.2126*channels[0] + 0.7152*channels[1] + 0.0722*channels[2], true
}
