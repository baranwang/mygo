//go:build linux && (amd64 || arm64)

package linux

import (
	"math"
	"slices"

	"github.com/egoist/mygo/internal/platform"
)

// A web view in a window of native UI (platform.Surface.NewWebView) is a
// window of this package without a GtkWindow of its own: win is its
// host's, and its WebKitWebView is a child of a GtkLayout under the
// surface's area, in a GtkOverlay that the first web view puts where the
// area was (the area is then an overlay of it, which GTK realizes again,
// with a new GL context). The area keeps an alpha channel from then on:
// its frames are transparent where the content shows a web view (holes),
// so that what the content paints after it shows over the page, and an
// input shape lets the pointer through to the page there. Its signals
// find it by its id in Backend.webViews, apart from the windows.

var (
	gtkLayoutNew                     func(h, v ptr) ptr
	gtkLayoutPut                     func(l, child ptr, x, y int32)
	gtkLayoutMove                    func(l, child ptr, x, y int32)
	gtkWidgetGetParent               func(w ptr) ptr
	gtkWidgetGetParentWindow         func(w ptr) ptr
	gdkWindowInputShapeCombineRegion func(w, region ptr, x, y int32)
	cairoRegionCreateRectangle       func(r *gdkRectangle) ptr
	cairoRegionSubtractRectangle     func(region ptr, r *gdkRectangle) int32
	cairoRegionUnion                 func(region, other ptr) int32
	cairoRegionDestroy               func(region ptr)
	webViewsLoaded                   bool
)

func loadWebViews() {
	if webViewsLoaded {
		return
	}
	webViewsLoaded = true
	mustBind(libGTK, &gtkLayoutNew, "gtk_layout_new")
	mustBind(libGTK, &gtkLayoutPut, "gtk_layout_put")
	mustBind(libGTK, &gtkLayoutMove, "gtk_layout_move")
	mustBind(libGTK, &gtkWidgetGetParent, "gtk_widget_get_parent")
	mustBind(libGTK, &gtkWidgetGetParentWindow, "gtk_widget_get_parent_window")
	mustBind(libGDK, &gdkWindowInputShapeCombineRegion, "gdk_window_input_shape_combine_region")
	mustBind(libCairo, &cairoRegionCreateRectangle, "cairo_region_create_rectangle")
	mustBind(libCairo, &cairoRegionSubtractRectangle, "cairo_region_subtract_rectangle")
	mustBind(libCairo, &cairoRegionUnion, "cairo_region_union")
	mustBind(libCairo, &cairoRegionDestroy, "cairo_region_destroy")
}

func (s *surface) NewWebView(o *platform.WindowOptions, h platform.WindowHandler) (platform.WebView, error) {
	if err := webKit(); err != nil {
		return nil, err
	}
	loadWebViews()
	host := s.w
	b := host.b
	if s.layers == 0 {
		s.makeLayers()
	}
	b.nextID++
	v := &window{b: b, id: b.nextID, h: h, opts: o, win: host.win, host: host}
	b.webViews[v.id] = v
	v.createWebView()
	gtkWidgetSetNoShowAll(v.web, true) // hidden until placed
	gtkLayoutPut(s.web, v.web, 0, 0)
	b.byWebView[v.web] = v
	host.webViews = append(host.webViews, v)
	return v, nil
}

// makeLayers puts the surface's area over a layout holding the web views,
// in an overlay where the area was.
func (s *surface) makeLayers() {
	area := s.area
	parent := gtkWidgetGetParent(area)
	focused := gtkWidgetHasFocus(area)
	s.layers = gtkOverlayNew()
	s.web = gtkLayoutNew(0, 0)
	gtkContainerAdd(s.layers, s.web)
	gObjectRef(area)
	if parent != 0 {
		gtkContainerRemove(parent, area)
	}
	if s.gl {
		// Holes show the web views through the frames.
		gtkGLAreaSetHasAlpha(area, true)
	}
	gtkOverlayAddOverlay(s.layers, area)
	gObjectUnref(area)
	switch {
	case parent == 0:
	case parent == s.w.box:
		gtkBoxPackStart(parent, s.layers, true, true, 0)
		if s.w.menubar != 0 {
			gtkBoxReorderChild(parent, s.layers, 1) // under the menu bar
		}
	default: // the overlay of a hidden title bar's controls
		gtkContainerAdd(parent, s.layers)
	}
	gtkWidgetShowAll(s.layers)
	if focused {
		gtkWidgetGrabFocus(area)
	}
}

// PlaceWebViews shows the window's web views where the content shows
// them, in its paint order under the surface, and hides the others; the
// area's input shape lets the pointer through to them.
func (s *surface) PlaceWebViews(views []platform.WebViewPlacement) {
	host := s.w
	if host.closed || s.layers == 0 {
		return
	}
	s.placed = append(s.placed[:0], views...)
	shown := make([]*window, 0, len(views))
	for _, p := range views {
		v, ok := p.WebView.(*window)
		if !ok || v.host != host || v.closed {
			continue
		}
		f := p.Frame
		x, y := int32(math.Round(f.X)), int32(math.Round(f.Y))
		gtkLayoutMove(s.web, v.web, x, y)
		gtkWidgetSetSizeRequest(v.web, int32(math.Round(f.X+f.W))-x, int32(math.Round(f.Y+f.H))-y)
		if !gtkWidgetGetVisible(v.web) {
			gtkWidgetShow(v.web)
		}
		shown = append(shown, v)
	}
	for _, v := range host.webViews {
		if !slices.Contains(shown, v) && gtkWidgetGetVisible(v.web) {
			if gtkWidgetHasFocus(v.web) {
				// A hidden view keeps no keyboard: it goes back to the
				// content.
				gtkWidgetGrabFocus(s.area)
			}
			gtkWidgetHide(v.web)
		}
	}
	s.shapeInput()
}

// shapeInput lets the pointer through the area where the web views take
// it: in their clips but where the content painted over them.
func (s *surface) shapeInput() {
	win := gtkWidgetGetParentWindow(s.area) // the overlay's window of the area
	if win == 0 || win == gtkWidgetGetWindow(s.w.win) {
		return
	}
	var all gdkRectangle
	gtkWidgetGetAllocation(s.area, &all)
	region := cairoRegionCreateRectangle(&gdkRectangle{Width: all.Width, Height: all.Height})
	for _, p := range s.placed {
		cairoRegionSubtractRectangle(region, gdkRect(p.Clip))
		// Unless what the content painted over it takes the pointer:
		// put back by a second region, as cairo has no union of a
		// difference.
		for _, c := range p.Covers {
			r := gdkRect(c)
			cover := cairoRegionCreateRectangle(r)
			cairoRegionUnion(region, cover)
			cairoRegionDestroy(cover)
		}
	}
	gdkWindowInputShapeCombineRegion(win, region, 0, 0)
	cairoRegionDestroy(region)
}

// gdkRect returns the pixels of r, in DIPs, that it covers wholly.
func gdkRect(r platform.RectF) *gdkRectangle {
	x, y := int32(math.Ceil(r.X)), int32(math.Ceil(r.Y))
	return &gdkRectangle{X: x, Y: y, Width: max(int32(math.Floor(r.X+r.W))-x, 0), Height: max(int32(math.Floor(r.Y+r.H))-y, 0)}
}

// webViewAt returns the web view that takes the pointer at x, y in the
// surface.
func (s *surface) webViewAt(x, y float64) *window {
	for i := len(s.placed) - 1; i >= 0; i-- {
		if p := &s.placed[i]; p.At(x, y) {
			if v, ok := p.WebView.(*window); ok && !v.closed {
				return v
			}
		}
	}
	return nil
}

// top returns the window that shows w: its host for a web view.
func (w *window) top() *window {
	if w.host != nil {
		return w.host
	}
	return w
}

// webViewPressed tells the content of the window that a button went down
// on the web view w, at x, y in its window, which takes the press.
func (w *window) webViewPressed(x, y float64, button uint32, mods platform.Modifiers) {
	s := w.host.surface
	if s == nil || w.host.closed {
		return
	}
	var a gdkRectangle
	gtkWidgetGetAllocation(w.web, &a)
	b, ok := map[uint32]int{1: 0, 2: 2, 3: 1}[button]
	if !ok {
		return
	}
	s.send(platform.SurfaceEvent{Kind: platform.WebViewPress, X: float64(a.X) + x, Y: float64(a.Y) + y, Button: b, Mods: mods})
}

// focusedWebView returns the web view of w that has the keyboard, if one
// does.
func (w *window) focusedWebView() *window {
	for _, v := range w.webViews {
		if !v.closed && gtkWidgetHasFocus(v.web) {
			return v
		}
	}
	return nil
}

// closeWebView closes a web view: by Close, or as its window closes.
func (w *window) closeWebView() {
	if w.closed {
		return
	}
	w.closed = true
	b := w.b
	delete(b.webViews, w.id)
	delete(b.byWebView, w.web)
	if gtkWidgetHasFocus(w.web) && w.host.surface != nil {
		gtkWidgetGrabFocus(w.host.surface.area)
	}
	webkitUserContentManagerUnregisterHandler(w.ucm, cs("mygo"))
	webkitUserContentManagerRemoveAllScripts(w.ucm)
	gtkWidgetDestroy(w.web)
	gObjectUnref(w.ucm)
	if w.press.event != 0 {
		gdkEventFree(w.press.event)
		w.press.event = 0
	}
	h := w.host
	h.webViews = slices.DeleteFunc(h.webViews, func(v *window) bool { return v == w })
	if s := h.surface; s != nil {
		s.placed = slices.DeleteFunc(s.placed, func(p platform.WebViewPlacement) bool { return p.WebView == platform.WebView(w) })
		if !h.closed {
			s.shapeInput()
		}
	}
}

// closeWebViews closes the web views of a window that closes.
func (w *window) closeWebViews() {
	for len(w.webViews) > 0 {
		w.webViews[len(w.webViews)-1].closeWebView()
	}
}
