package e2e

import (
	"sync/atomic"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const webViewPage = `<!doctype html><html><body style="margin:0;height:100vh;background:#fff"
	onmousedown="window.clicks = (window.clicks || 0) + 1">
<script>
mygo.call("Greeter.WindowID").then((id) => { window.windowId = id; window.ready = true; });
</script></body></html>`

// TestContentWindowWebView shows a web page in a window of native UI,
// under a popover the view opens over it: the page calls Go as a window's
// does, with the window as caller, clicks on the page reach it, those on
// the popover over it do not, and a click on the page closes the popover.
func TestContentWindowWebView(t *testing.T) {
	var view *mygo.WebView
	var open, frames, items atomic.Int32
	w := newWindow(t, mygo.WindowOptions{Title: "Web view", Width: 400, Height: 300, Content: ui.View(func(c *ui.Context) {
		frames.Add(1)
		ui.Column(c).Fill().AlignItems(ui.Stretch).Children(func() {
			menu := ui.Button(c, "Menu").Height(40).Width(100)
			if menu.Clicked() {
				open.Store(1)
			}
			shown := open.Load() == 1
			ui.Popover(c, menu, &shown, func() {
				if ui.Box(c).Size(150, 100).Background(ui.RGB(255, 0, 0)).Clicked() {
					items.Add(1)
				}
			})
			if !shown {
				open.Store(0)
			}
			if view != nil {
				ui.WebView(c, view).Grow(1)
			}
		})
	})})
	eventually(t, "a frame", func() bool { return frames.Load() > 0 })
	v, err := w.NewWebView(mygo.WebViewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if v.Window() != w || v.Page().WebView() != v {
		t.Error("the web view's window or page")
	}
	w.Update(func() { view = v })
	v.Page().LoadHTML(webViewPage, "")
	waitForPage(t, v.Page(), "window.ready")
	if id, err := mygo.EvalAs[float64](v.Page(), "window.windowId"); err != nil || int(id) != w.ID() {
		t.Errorf("the page called as window %v (%v), want %d", id, err, w.ID())
	}

	w.Focus()
	clicks := func() float64 {
		n, _ := mygo.EvalAs[float64](v.Page(), "window.clicks || 0")
		return n
	}
	if !click(w, 300, 200) {
		t.Skip("click automation not available on this platform")
	}
	eventually(t, "the click on the page", func() bool { return clicks() == 1 })

	click(w, 50, 20)
	eventually(t, "the popover", func() bool { return open.Load() == 1 })
	click(w, 50, 80) // on the popover, over the page
	eventually(t, "the click on the popover", func() bool { return items.Load() == 1 })
	if n := clicks(); n != 1 {
		t.Errorf("the page got %v clicks under the popover", n)
	}
	click(w, 300, 200)
	eventually(t, "the second click on the page", func() bool { return clicks() == 2 })
	eventually(t, "the popover closing", func() bool { return open.Load() == 0 })

	v.Destroy()
	if !v.IsDestroyed() {
		t.Error("Destroy left the web view")
	}
}
