package main

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"strings"
	"unicode"

	"github.com/a-h/gemini"
	"github.com/gdamore/tcell"
	"github.com/mattn/go-runewidth"
)

func main() {
	// Create a screen.
	tcell.SetEncodingFallback(tcell.EncodingFallbackASCII)
	s, err := tcell.NewScreen()
	if err != nil {
		fmt.Println("Error creating screen:", err)
		os.Exit(1)
	}
	err = s.Init()
	if err != nil {
		fmt.Println("Error initializing screen:", err)
		os.Exit(1)
	}
	defer s.Fini()

	// Set default colours.
	s.SetStyle(tcell.StyleDefault.
		Foreground(tcell.ColorWhite).
		Background(tcell.ColorBlack))
	s.Clear()
	s.Show()

	// Parse the input.
	urlString := strings.Join(os.Args[1:], "")
	if urlString == "" {
		//TODO: Load up a home page.
		urlString = "gemini://localhost"
	}
	for {
		// Grab the URL input.
		urlString, ok := NewInput(s, 0, 0, tcell.StyleDefault, "Location:", urlString).Focus()
		if !ok {
			break
		}

		// Check the URL.
		u, err := url.Parse(urlString)
		if err != nil {
			NewOptions(s, 0, 0, tcell.StyleDefault, fmt.Sprintf("Failed to parse address: %q: %v", urlString, err), "OK").Focus()
			continue
		}

		// Connect.
		client := gemini.NewClient()
		var resp *gemini.Response
		var certificates []string
		var redirectCount int
	out:
		for {
			//TODO: Add cert store etc. to the client.
			resp, certificates, _, ok, err = client.RequestURL(u)
			if err != nil {
				switch NewOptions(s, 0, 0, tcell.StyleDefault, fmt.Sprintf("Request error: %v", err), "Retry", "Cancel").Focus() {
				case "Retry":
					continue
				case "Cancel":
					break out
				}

			}
			if !ok {
				//TOFU check required.
				switch NewOptions(s, 0, 0, tcell.StyleDefault, fmt.Sprintf("Accept client certificate?\n\n  %v", certificates[0]), "Accept", "Reject").Focus() {
				case "Accept":
					client.AddAlllowedCertificateForHost(u.Host, certificates[0])
					continue
				case "Reject":
					break out
				}
			}
			break
		}
		if !ok || resp == nil {
			continue
		}
		if resp.Header.Code == gemini.CodeClientCertificateRequired {
			switch NewOptions(s, 0, 0, tcell.StyleDefault, fmt.Sprintf("The server is requested a certificate."), "Create temporary", "Cancel").Focus() {
			case "Create temporary":
				//TODO: Add a certificate to the store.
				break
			case "Cancel":
				break
			}
		}
		//if resp.Header.Code == gemini.CodeInput {
		//text, ok := NewInput(s, 0, 0, tcell.StyleDefault, resp.Header.Meta, "").Focus()
		////TODO: Post the input back.
		//continue
		//}
		if strings.HasPrefix(string(resp.Header.Code), "3") {
			//TODO: Handle redirect.
			redirectCount++
		}
		if strings.HasPrefix(string(resp.Header.Code), "2") {
			NewBrowser(s, u, resp).Focus()
			continue
		}
		NewOptions(s, 0, 0, tcell.StyleDefault, fmt.Sprintf("Unknown code: %v %s", resp.Header.Code, resp.Header.Meta), "OK").Focus()
	}
}

// flow breaks up text to its maximum width.
func flow(s string, maxWidth int) []string {
	var ss []string
	flowProcessor(s, maxWidth, func(line string) {
		ss = append(ss, line)
	})
	return ss
}

func flowProcessor(s string, maxWidth int, out func(string)) {
	var buf strings.Builder
	var col int
	var lastSpace int
	for _, r := range s {
		if r == '\r' {
			continue
		}
		if r == '\n' {
			out(buf.String())
			buf.Reset()
			col = 0
			lastSpace = 0
			continue
		}
		buf.WriteRune(r)
		if unicode.IsSpace(r) {
			lastSpace = col
		}
		if col == maxWidth {
			// If the word is greater than the width, then break the word down.
			end := lastSpace
			if end == 0 {
				end = col
			}
			out(strings.TrimSpace(buf.String()[:end]))
			prefix := strings.TrimSpace(buf.String()[end:])
			buf.Reset()
			lastSpace = 0
			buf.WriteString(prefix)
			col = len(prefix)
			continue
		}
		col++
	}
	out(buf.String())
}

func NewText(s tcell.Screen, x, y int, st tcell.Style, text string) Text {
	return Text{
		Screen: s,
		X:      x,
		Y:      y,
		Style:  st,
		Text:   text,
	}
}

type Text struct {
	Screen tcell.Screen
	X      int
	Y      int
	Style  tcell.Style
	Text   string
}

func (t Text) Draw() (x, y int) {
	maxX, maxY := t.Screen.Size()
	flowed := flow(t.Text, maxX)
	for lineIndex := 0; lineIndex < len(flowed); lineIndex++ {
		y := t.Y + lineIndex
		if y > maxY {
			break
		}
		x = t.X
		for _, c := range flowed[lineIndex] {
			var comb []rune
			w := runewidth.RuneWidth(c)
			if w == 0 {
				comb = []rune{c}
				c = ' '
				w = 1
			}
			t.Screen.SetContent(x, y, c, comb, t.Style)
			x += w
		}
	}
	return x, y
}

func NewOptions(s tcell.Screen, x, y int, st tcell.Style, msg string, opts ...string) *Options {
	return &Options{
		Screen:  s,
		X:       x,
		Y:       y,
		Style:   st,
		Message: msg,
		Options: opts,
	}
}

type Options struct {
	Screen      tcell.Screen
	X           int
	Y           int
	Style       tcell.Style
	Message     string
	Options     []string
	ActiveIndex int
}

func (o *Options) Draw() {
	o.Screen.Clear()
	t := NewText(o.Screen, 0, 0, tcell.StyleDefault, o.Message)
	_, y := t.Draw()
	for i, oo := range o.Options {
		style := tcell.StyleDefault
		if i == o.ActiveIndex {
			style = tcell.StyleDefault.Background(tcell.ColorLightGray)
		}
		t := NewText(o.Screen, 1, i+y+2, style, fmt.Sprintf("[ %s ]", oo))
		t.Draw()
	}
}

func (o *Options) Up() {
	if o.ActiveIndex == 0 {
		return
	}
	o.ActiveIndex--
}

func (o *Options) Down() {
	if o.ActiveIndex == len(o.Options)-1 {
		return
	}
	o.ActiveIndex++
}

func (o *Options) Focus() string {
	o.Draw()
	o.Screen.Show()
	for {
		switch ev := o.Screen.PollEvent().(type) {
		case *tcell.EventResize:
			o.Screen.Sync()
		case *tcell.EventKey:
			switch ev.Key() {
			case tcell.KeyUp:
				o.Up()
			case tcell.KeyDown:
				o.Down()
			case tcell.KeyEnter:
				return o.Options[o.ActiveIndex]
			}
		}
		o.Draw()
		o.Screen.Show()
	}
}

func NewBrowser(s tcell.Screen, u *url.URL, resp *gemini.Response) *Browser {
	return &Browser{
		Screen:   s,
		URL:      u,
		Response: resp,
	}
}

type Browser struct {
	Screen   tcell.Screen
	URL      *url.URL
	Response *gemini.Response
}

func (b Browser) Draw() {
	b.Screen.Clear()
	//TODO: Handle error reading.
	body, _ := ioutil.ReadAll(b.Response.Body)
	//TODO: Render the lines properly.
	NewText(b.Screen, 0, 0, tcell.StyleDefault, string(body)).Draw()
}

func (b Browser) Focus() {
	b.Draw()
	b.Screen.Show()
	for {
		switch ev := b.Screen.PollEvent().(type) {
		case *tcell.EventResize:
			b.Screen.Sync()
			b.Draw()
			b.Screen.Show()
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyEscape {
				return
			}
		}
	}
}

func NewInput(s tcell.Screen, x, y int, st tcell.Style, msg, text string) *Input {
	return &Input{
		Screen:      s,
		X:           x,
		Y:           y,
		Style:       st,
		Message:     msg,
		Text:        text,
		CursorIndex: len(text),
	}
}

type Input struct {
	Screen      tcell.Screen
	X           int
	Y           int
	Style       tcell.Style
	Message     string
	Text        string
	CursorIndex int
	ActiveIndex int
}

func (o *Input) Draw() {
	o.Screen.Clear()
	t := NewText(o.Screen, o.X, o.Y, o.Style, o.Message)
	_, y := t.Draw()

	defaultStyle := tcell.StyleDefault
	activeStyle := tcell.StyleDefault.Background(tcell.ColorLightGray)

	textStyle := defaultStyle
	if o.ActiveIndex == 0 {
		textStyle = defaultStyle.Underline(true)
		// Show that the input is active.
		NewText(o.Screen, o.X, o.Y+y+2, defaultStyle, ">").Draw()
	}
	NewText(o.Screen, o.X+2, o.Y+y+2, textStyle, o.Text).Draw()
	if o.ActiveIndex == 0 {
		o.Screen.ShowCursor(o.X+2+o.CursorIndex, o.Y+y+2)
	} else {
		o.Screen.HideCursor()
	}

	okStyle := defaultStyle
	if o.ActiveIndex == 1 {
		okStyle = activeStyle
	}
	NewText(o.Screen, 1, o.Y+y+4, okStyle, "[ OK ]").Draw()
	cancelStyle := defaultStyle
	if o.ActiveIndex == 2 {
		cancelStyle = activeStyle
	}
	NewText(o.Screen, 1, o.Y+y+5, cancelStyle, "[ Cancel ]").Draw()
}

func (o *Input) Up() {
	if o.ActiveIndex == 0 {
		return
	}
	o.ActiveIndex--
}

func (o *Input) Down() {
	if o.ActiveIndex == 2 {
		return
	}
	o.ActiveIndex++
}

type InputResult string

func (o *Input) Focus() (text string, ok bool) {
	o.Draw()
	o.Screen.Show()
	for {
		if o.ActiveIndex == 0 {
			// Handle textbox keys.
			switch ev := o.Screen.PollEvent().(type) {
			case *tcell.EventResize:
				o.Screen.Sync()
			case *tcell.EventKey:
				switch ev.Key() {
				case tcell.KeyBackspace:
					if tl := len(o.Text); tl > 0 {
						o.CursorIndex--
						o.Text = o.Text[0 : tl-1]
					}
				case tcell.KeyLeft:
					if o.CursorIndex > 0 {
						o.CursorIndex--
					}
				case tcell.KeyRight:
					if o.CursorIndex < len(o.Text) {
						o.CursorIndex++
					}
				case tcell.KeyDelete:
					o.Text = cut(o.Text, o.CursorIndex)
				case tcell.KeyHome:
					o.CursorIndex = 0
				case tcell.KeyEnd:
					o.CursorIndex = len(o.Text)
				case tcell.KeyRune:
					o.Text = insert(o.Text, o.CursorIndex, ev.Rune())
					o.CursorIndex++
				case tcell.KeyDown:
					o.Down()
				case tcell.KeyEnter:
					o.Down()
				}
			}
			o.Draw()
			o.Screen.Show()
			continue
		}
		switch ev := o.Screen.PollEvent().(type) {
		case *tcell.EventResize:
			o.Screen.Sync()
		case *tcell.EventKey:
			switch ev.Key() {
			case tcell.KeyUp:
				o.Up()
			case tcell.KeyDown:
				o.Down()
			case tcell.KeyEnter:
				switch o.ActiveIndex {
				case 0:
					o.ActiveIndex = 1
					break
				case 1:
					return o.Text, true
				case 2:
					return o.Text, false
				}
			}
		}
		o.Draw()
		o.Screen.Show()
	}
}

func cut(s string, at int) string {
	prefix, suffix := split(s, at)
	if len(suffix) > 0 {
		suffix = suffix[1:]
	}
	return prefix + suffix
}

func split(s string, at int) (prefix, suffix string) {
	if at > len(s) {
		prefix = s
		return
	}
	prefix = string([]rune(s)[:at])
	suffix = string([]rune(s)[at:])
	return
}

func insert(s string, at int, r rune) string {
	prefix, suffix := split(s, at)
	return prefix + string(r) + suffix
}
