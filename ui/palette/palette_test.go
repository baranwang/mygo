package palette

import (
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func near(a, b ui.Color) bool {
	d := func(x, y uint8) int { return max(int(x)-int(y), int(y)-int(x)) }
	return d(a.R, b.R) <= 2 && d(a.G, b.G) <= 2 && d(a.B, b.B) <= 2 && a.A == b.A
}

func TestShades(t *testing.T) {
	src, err := os.ReadFile("colors.go")
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(src), "\nfunc "); n != 26*11 {
		t.Errorf("%d colors, want %d", n, 26*11)
	}
}

// The sRGB values Tailwind's documentation gives for these colors.
func TestSRGBFallbacks(t *testing.T) {
	for _, c := range []struct {
		name string
		got  ui.Color
		want ui.Color
	}{
		{"blue-600", Blue600(), ui.Hex("#155dfc")},
		{"red-500", Red500(), ui.Hex("#fb2c36")},
		{"slate-900", Slate900(), ui.Hex("#0f172b")},
		{"neutral-50", Neutral50(), ui.Hex("#fafafa")},
		{"white", White(), ui.Hex("#fff")},
	} {
		if !near(c.got, c.want) {
			t.Errorf("%s = %+v, want about %+v", c.name, c.got, c.want)
		}
	}
}
