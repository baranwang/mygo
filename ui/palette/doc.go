// Package palette has ready-made colors: Tailwind CSS's default palette, 26
// hues in the shades 50, 100 to 900 and 950, as functions such as Blue500.
//
// The colors are those of Tailwind CSS v4.3.3, defined in oklch, which MyGo
// draws in the wide gamut of a display that has one (see [ui.Oklch]); a
// function builds its color on each call, so keep the color of one you use
// every frame:
//
//	t := ui.LightTheme()
//	t.Accent = palette.Blue600()
//
// Tailwind CSS is copyright (c) Tailwind Labs, Inc., under the MIT license.
package palette
