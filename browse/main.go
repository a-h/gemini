package main

import (
	"bufio"
	"fmt"
	"io"
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
	var askForURL, ok bool
	askForURL = true
	for {
		// Grab the URL input.
		if askForURL {
			urlString, ok = NewInput(s, 0, 0, tcell.StyleDefault, "Location:", urlString).Focus()
			if !ok {
				break
			}
		}
		askForURL = !askForURL

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
				switch NewOptions(s, 0, 0, tcell.StyleDefault, fmt.Sprintf("Accept client certificate?\n  %v", certificates[0]), "Accept", "Reject").Focus() {
				case "Accept":
					//TODO: Save this in a persistent store.
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
		if resp.Header.Code == gemini.CodeInput {
			text, ok := NewInput(s, 0, 0, tcell.StyleDefault, resp.Header.Meta, "").Focus()
			if !ok {
				continue
			}
			// Post the input back.
			askForURL = false
			u.RawQuery = url.QueryEscape(text)
			urlString = u.String()
			continue
		}
		if strings.HasPrefix(string(resp.Header.Code), "3") {
			//TODO: Handle redirect.
			redirectCount++
		}
		if strings.HasPrefix(string(resp.Header.Code), "2") {
			b, err := NewBrowser(s, u, resp)
			if err != nil {
				NewOptions(s, 0, 0, tcell.StyleDefault, fmt.Sprintf("Error displaying server response:\n\n%v", err), "OK").Focus()
				continue
			}
			next, err := b.Focus()
			if err != nil {
				//TODO: The link was garbage, show the error.
				continue
			}
			if next != nil {
				askForURL = false
				urlString = next.String()
				continue
			}
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
	maxWidth := maxX - t.X
	if maxWidth < 0 {
		// It's off screen, so there's nothing to display.
		return
	}
	lines := flow(t.Text, maxWidth)
	var requiredMaxWidth int
	for lineIndex := 0; lineIndex < len(lines); lineIndex++ {
		y = t.Y + lineIndex
		if y > maxY {
			break
		}
		x = t.X
		for _, c := range lines[lineIndex] {
			var comb []rune
			w := runewidth.RuneWidth(c)
			if w == 0 {
				comb = []rune{c}
				c = ' '
				w = 1
			}
			t.Screen.SetContent(x, y, c, comb, t.Style)
			x += w
			if x > requiredMaxWidth {
				requiredMaxWidth = x
			}
		}
	}
	return requiredMaxWidth, y
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

func NewLineConverter(resp *gemini.Response) *LineConverter {
	return &LineConverter{
		Response: resp,
	}
}

type LineConverter struct {
	Response     *gemini.Response
	preFormatted bool
}

func (lc *LineConverter) process(s string) (l Line, isVisualLine bool) {
	if strings.HasPrefix(s, "```") {
		lc.preFormatted = !lc.preFormatted
		return l, false
	}
	if lc.preFormatted {
		return PreformattedTextLine{Text: s}, true
	}
	if strings.HasPrefix(s, "=>") {
		return LinkLine{Text: s}, true
	}
	if strings.HasPrefix(s, "#") {
		return HeadingLine{Text: s}, true
	}
	if strings.HasPrefix(s, "* ") {
		return UnorderedListItemLine{Text: s}, true
	}
	if strings.HasPrefix(s, ">") {
		return QuoteLine{Text: s}, true
	}
	return TextLine{Text: s}, true
}

func (lc *LineConverter) Lines() (lines []Line, err error) {
	reader := bufio.NewReader(lc.Response.Body)
	var s string
	for {
		s, err = reader.ReadString('\n')
		line, isVisual := lc.process(strings.TrimRight(s, "\n"))
		if isVisual {
			lines = append(lines, line)
		}
		if err != nil {
			break
		}
	}
	if err == io.EOF {
		err = nil
	}
	return
}

type Line interface {
	Draw(to tcell.Screen, atX, atY int, highlighted bool) (x, y int)
}

type TextLine struct {
	Text string
}

func (l TextLine) Draw(to tcell.Screen, atX, atY int, highlighted bool) (x, y int) {
	return NewText(to, atX, atY, tcell.StyleDefault, l.Text).Draw()
}

type PreformattedTextLine struct {
	Text string
}

func (l PreformattedTextLine) Draw(to tcell.Screen, atX, atY int, highlighted bool) (x, y int) {
	return NewText(to, atX, atY, tcell.StyleDefault, l.Text).Draw()
}

type LinkLine struct {
	Text string
}

func (l LinkLine) URL(relativeTo *url.URL) (u *url.URL, err error) {
	urlString := strings.TrimPrefix(l.Text, "=>")
	urlString = strings.TrimSpace(urlString)
	urlString = strings.SplitN(urlString, " ", 2)[0]
	u, err = url.Parse(urlString)
	if err != nil {
		return
	}
	if u.IsAbs() || relativeTo == nil {
		return
	}
	return relativeTo.ResolveReference(u), nil
}

func (l LinkLine) Draw(to tcell.Screen, atX, atY int, highlighted bool) (x, y int) {
	ls := tcell.StyleDefault.Foreground(tcell.ColorBlue)
	if highlighted {
		ls = ls.Underline(true)
	}
	return NewText(to, atX+2, atY, ls, l.Text).Draw()
}

type HeadingLine struct {
	Text string
}

func (l HeadingLine) Draw(to tcell.Screen, atX, atY int, highlighted bool) (x, y int) {
	return NewText(to, atX, atY, tcell.StyleDefault.Foreground(tcell.ColorGreen), l.Text).Draw()
}

type UnorderedListItemLine struct {
	Text string
}

func (l UnorderedListItemLine) Draw(to tcell.Screen, atX, atY int, highlighted bool) (x, y int) {
	return NewText(to, atX+2, atY, tcell.StyleDefault, l.Text).Draw()
}

type QuoteLine struct {
	Text string
}

func (l QuoteLine) Draw(to tcell.Screen, atX, atY int, highlighted bool) (x, y int) {
	return NewText(to, atX+2, atY, tcell.StyleDefault.Foreground(tcell.ColorLightGrey), l.Text).Draw()
}

func NewBrowser(s tcell.Screen, u *url.URL, resp *gemini.Response) (b *Browser, err error) {
	b = &Browser{
		Screen:          s,
		URL:             u,
		ResponseHeader:  resp.Header,
		ActiveLineIndex: -1,
	}
	b.Lines, err = NewLineConverter(resp).Lines()
	return
}

type Browser struct {
	Screen          tcell.Screen
	URL             *url.URL
	ResponseHeader  *gemini.Header
	Lines           []Line
	ActiveLineIndex int
}

func (b *Browser) Links() (indices []int) {
	for i := 0; i < len(b.Lines); i++ {
		if _, ok := b.Lines[i].(LinkLine); ok {
			indices = append(indices, i)
		}
	}
	return indices
}

func (b *Browser) CurrentLink() (u *url.URL, err error) {
	for i := 0; i < len(b.Lines); i++ {
		if i == b.ActiveLineIndex {
			if ll, ok := b.Lines[b.ActiveLineIndex].(LinkLine); ok {
				return ll.URL(b.URL)
			}
		}
	}
	return nil, nil
}

func (b *Browser) PreviousLink() {
	ll := b.Links()
	if len(ll) == 0 {
		return
	}
	if b.ActiveLineIndex < 0 {
		b.ActiveLineIndex = ll[len(ll)-1]
		return
	}
	var curIndex, li int
	for curIndex, li = range ll {
		if li == b.ActiveLineIndex {
			break
		}
	}
	if curIndex == 0 {
		b.ActiveLineIndex = ll[len(ll)-1]
		return
	}
	b.ActiveLineIndex = ll[curIndex-1]
}

func (b *Browser) NextLink() {
	ll := b.Links()
	if len(ll) == 0 {
		return
	}
	if b.ActiveLineIndex < 0 {
		b.ActiveLineIndex = ll[0]
		return
	}
	var curIndex, li int
	for curIndex, li = range ll {
		if li == b.ActiveLineIndex {
			break
		}
	}
	if curIndex == len(ll)-1 {
		b.ActiveLineIndex = ll[0]
		return
	}
	b.ActiveLineIndex = ll[curIndex+1]
}

func (b Browser) Draw() {
	b.Screen.Clear()
	//TODO: Handle scrolling.
	var y int
	for lineIndex, line := range b.Lines {
		highlighted := lineIndex == b.ActiveLineIndex
		_, yy := line.Draw(b.Screen, 0, y, highlighted)
		y = yy + 1
	}
}

func (b Browser) Focus() (next *url.URL, err error) {
	b.Draw()
	b.Screen.Show()
	for {
		switch ev := b.Screen.PollEvent().(type) {
		case *tcell.EventResize:
			b.Screen.Sync()
		case *tcell.EventKey:
			switch ev.Key() {
			case tcell.KeyEscape:
				return
			case tcell.KeyBacktab:
				b.PreviousLink()
			case tcell.KeyTAB:
				b.NextLink()
			case tcell.KeyCtrlO:
				b.PreviousLink()
			case tcell.KeyEnter:
				return b.CurrentLink()
			case tcell.KeyRune:
				switch ev.Rune() {
				case 'j':
					//TODO: Scroll down.
				case 'k':
					//TODO: Scroll up.
				case 'n':
					b.NextLink()
				}
			}
		}
		b.Draw()
		b.Screen.Show()
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
				case tcell.KeyBacktab:
					o.Up()
				case tcell.KeyTab:
					o.Down()
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