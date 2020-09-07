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

	// Parse the command-line URL, if provided.
	urlString := strings.Join(os.Args[1:], "")
	if urlString == "" {
		//TODO: Load up a home page based on configuration.
		urlString = "gemini://localhost"
	}

	// Create required top-level variables.
	client := gemini.NewClient()
	var redirectCount int

	var askForURL, ok bool
	askForURL = true
	for {
		// Grab the URL input.
		if askForURL {
			urlString, ok = NewInput(s, "Location:", urlString).Focus()
			if !ok {
				break
			}
		}

		// Check the URL.
		u, err := url.Parse(urlString)
		if err != nil {
			NewOptions(s, fmt.Sprintf("Failed to parse address: %q: %v", urlString, err), "OK").Focus()
			askForURL = true
			continue
		}

		// Connect.
		var resp *gemini.Response
		var certificates []string
	out:
		for {
			//TODO: Add cert store etc. to the client.
			resp, certificates, _, ok, err = client.RequestURL(u)
			if err != nil {
				switch NewOptions(s, fmt.Sprintf("Request error: %v", err), "Retry", "Cancel").Focus() {
				case "Retry":
					askForURL = false
					continue
				case "Cancel":
					break out
				}
			}
			if !ok {
				//TOFU check required.
				switch NewOptions(s, fmt.Sprintf("Accept client certificate?\n  %v", certificates[0]), "Accept (Permanent)", "Accept (Temporary)", "Reject").Focus() {
				case "Accept (Permanent)":
					//TODO: Save this in a persistent store.
					client.AddAlllowedCertificateForHost(u.Host, certificates[0])
					askForURL = false
					continue
				case "Accept (Temporary)":
					client.AddAlllowedCertificateForHost(u.Host, certificates[0])
					askForURL = false
					continue
				case "Reject":
					break out
				}
			}
			break
		}
		if !ok || resp == nil {
			askForURL = true
			continue
		}
		if strings.HasPrefix(string(resp.Header.Code), "3") {
			redirectCount++
			if redirectCount >= 5 {
				NewOptions(s, fmt.Sprintf("Server issued 5 or more redirects, cancelling request."), "OK").Focus()
				askForURL = true
				continue
			}
			redirectTo, err := url.Parse(resp.Header.Meta)
			if err != nil {
				//TODO: Add the ability to go back, once history has been added.
				NewOptions(s, fmt.Sprintf("Server returned invalid redirect: code %s, meta: %q", resp.Header.Code, resp.Header.Meta), "OK").Focus()
				askForURL = true
				continue
			}
			//TODO: Check with the user if the redirect is to another domain or protocol.
			urlString = u.ResolveReference(redirectTo).String()
			askForURL = false
			continue
		}
		redirectCount = 0
		if strings.HasPrefix(string(resp.Header.Code), "6") {
			msg := fmt.Sprintf("The server has requested a certificate: code %s, meta: %q", resp.Header.Code, resp.Header.Meta)
			switch NewOptions(s, msg, "Create (Permanent)", "Create (Temporary)", "Cancel").Focus() {
			case "Create (Permanent)":
				//TODO: Add a certificate to the permanent store.
				askForURL = false
				continue
			case "Create (Temporary)":
				//TODO: Add a certificate to the client.
				askForURL = false
				continue
			case "Cancel":
				askForURL = true
				continue
			}
		}
		if strings.HasPrefix(string(resp.Header.Code), "1") {
			text, ok := NewInput(s, resp.Header.Meta, "").Focus()
			if !ok {
				continue
			}
			// Post the input back.
			u.RawQuery = url.QueryEscape(text)
			urlString = u.String()
			askForURL = false
			continue
		}
		if strings.HasPrefix(string(resp.Header.Code), "2") {
			b, err := NewBrowser(s, u, resp)
			if err != nil {
				NewOptions(s, fmt.Sprintf("Error displaying server response:\n\n%v", err), "OK").Focus()
				askForURL = true
				continue
			}
			next, err := b.Focus()
			if err != nil {
				//TODO: The link was garbage, show the error.
				NewOptions(s, fmt.Sprintf("Invalid link: %v\n", err), "OK").Focus()
				askForURL = true
				continue
			}
			if next != nil {
				//TODO: Ask the user whether they want to follow it, if it's a non-Gemini link, or goes to a different domain.
				urlString = next.String()
				askForURL = false
				continue
			}
			askForURL = true
			continue
		}
		NewOptions(s, fmt.Sprintf("Unknown code: %v %s", resp.Header.Code, resp.Header.Meta), "OK").Focus()
		askForURL = true
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

func NewText(s tcell.Screen, text string) *Text {
	return &Text{
		Screen: s,
		X:      0,
		Y:      0,
		Style:  tcell.StyleDefault,
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

func (t *Text) WithOffset(x, y int) *Text {
	t.X = x
	t.Y = y
	return t
}

func (t *Text) WithStyle(st tcell.Style) *Text {
	t.Style = st
	return t
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

func NewOptions(s tcell.Screen, msg string, opts ...string) *Options {
	cancelIndex := -1
	for i, o := range opts {
		if o == "Cancel" {
			cancelIndex = i
			break
		}
	}
	return &Options{
		Screen:      s,
		X:           0,
		Y:           0,
		Style:       tcell.StyleDefault,
		Message:     msg,
		Options:     opts,
		CancelIndex: cancelIndex,
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
	CancelIndex int
}

func (o *Options) Draw() {
	o.Screen.Clear()
	t := NewText(o.Screen, o.Message)
	_, y := t.Draw()
	for i, oo := range o.Options {
		style := tcell.StyleDefault
		if i == o.ActiveIndex {
			style = tcell.StyleDefault.Background(tcell.ColorLightGray)
		}
		NewText(o.Screen, fmt.Sprintf("[ %s ]", oo)).WithOffset(1, i+y+2).WithStyle(style).Draw()
	}
}

func (o *Options) Up() {
	if o.ActiveIndex == 0 {
		o.ActiveIndex = len(o.Options) - 1
		return
	}
	o.ActiveIndex--
}

func (o *Options) Down() {
	if o.ActiveIndex == len(o.Options)-1 {
		o.ActiveIndex = 0
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
			case tcell.KeyBacktab:
				o.Up()
			case tcell.KeyTab:
				o.Down()
			case tcell.KeyUp:
				o.Up()
			case tcell.KeyDown:
				o.Down()
			case tcell.KeyEscape:
				if o.CancelIndex > -1 {
					return o.Options[o.CancelIndex]
				}
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
	return NewText(to, l.Text).WithOffset(atX, atY).Draw()
}

type PreformattedTextLine struct {
	Text string
}

func (l PreformattedTextLine) Draw(to tcell.Screen, atX, atY int, highlighted bool) (x, y int) {
	return NewText(to, l.Text).WithOffset(atX, atY).Draw()
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
	if relativeTo == nil {
		return
	}
	return relativeTo.ResolveReference(u), nil
}

func (l LinkLine) Draw(to tcell.Screen, atX, atY int, highlighted bool) (x, y int) {
	ls := tcell.StyleDefault.Foreground(tcell.ColorBlue)
	if highlighted {
		ls = ls.Underline(true)
	}
	return NewText(to, l.Text).WithOffset(atX+2, atY).WithStyle(ls).Draw()
}

type HeadingLine struct {
	Text string
}

func (l HeadingLine) Draw(to tcell.Screen, atX, atY int, highlighted bool) (x, y int) {
	return NewText(to, l.Text).WithOffset(atX, atY).WithStyle(tcell.StyleDefault.Foreground(tcell.ColorGreen)).Draw()
}

type UnorderedListItemLine struct {
	Text string
}

func (l UnorderedListItemLine) Draw(to tcell.Screen, atX, atY int, highlighted bool) (x, y int) {
	return NewText(to, l.Text).WithOffset(atX+2, atY).Draw()
}

type QuoteLine struct {
	Text string
}

func (l QuoteLine) Draw(to tcell.Screen, atX, atY int, highlighted bool) (x, y int) {
	return NewText(to, l.Text).WithOffset(atX+2, atY).WithStyle(tcell.StyleDefault.Foreground(tcell.ColorLightGrey)).Draw()
}

func NewBrowser(s tcell.Screen, u *url.URL, resp *gemini.Response) (b *Browser, err error) {
	b = &Browser{
		Screen:          s,
		URL:             u,
		ResponseHeader:  resp.Header,
		ActiveLineIndex: -1,
	}
	b.Lines, err = NewLineConverter(resp).Lines()
	b.calculateLinkIndices()
	return
}

type Browser struct {
	Screen          tcell.Screen
	URL             *url.URL
	ResponseHeader  *gemini.Header
	Lines           []Line
	LinkLineIndices []int
	ActiveLineIndex int
}

func (b *Browser) calculateLinkIndices() {
	for i := 0; i < len(b.Lines); i++ {
		if _, ok := b.Lines[i].(LinkLine); ok {
			b.LinkLineIndices = append(b.LinkLineIndices, i)
		}
	}
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
	if len(b.LinkLineIndices) == 0 {
		return
	}
	if b.ActiveLineIndex < 0 {
		b.ActiveLineIndex = b.LinkLineIndices[len(b.LinkLineIndices)-1]
		return
	}
	var curIndex, li int
	for curIndex, li = range b.LinkLineIndices {
		if li == b.ActiveLineIndex {
			break
		}
	}
	if curIndex == 0 {
		b.ActiveLineIndex = b.LinkLineIndices[len(b.LinkLineIndices)-1]
		return
	}
	b.ActiveLineIndex = b.LinkLineIndices[curIndex-1]
}

func (b *Browser) NextLink() {
	if len(b.LinkLineIndices) == 0 {
		return
	}
	if b.ActiveLineIndex < 0 {
		b.ActiveLineIndex = b.LinkLineIndices[0]
		return
	}
	var curIndex, li int
	for curIndex, li = range b.LinkLineIndices {
		if li == b.ActiveLineIndex {
			break
		}
	}
	if curIndex == len(b.LinkLineIndices)-1 {
		b.ActiveLineIndex = b.LinkLineIndices[0]
		return
	}
	b.ActiveLineIndex = b.LinkLineIndices[curIndex+1]
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

func NewInput(s tcell.Screen, msg, text string) *Input {
	return &Input{
		Screen:      s,
		X:           0,
		Y:           0,
		Style:       tcell.StyleDefault,
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
	_, y := NewText(o.Screen, o.Message).WithOffset(o.X, o.Y).WithStyle(o.Style).Draw()

	defaultStyle := tcell.StyleDefault
	activeStyle := tcell.StyleDefault.Background(tcell.ColorLightGray)

	textStyle := defaultStyle
	if o.ActiveIndex == 0 {
		NewText(o.Screen, ">").WithOffset(o.X, o.Y+y+2).WithStyle(defaultStyle).Draw()
	}
	NewText(o.Screen, o.Text).WithOffset(o.X+2, o.Y+y+2).WithStyle(textStyle).Draw()
	if o.ActiveIndex == 0 {
		o.Screen.ShowCursor(o.X+2+o.CursorIndex, o.Y+y+2)
	} else {
		o.Screen.HideCursor()
	}

	okStyle := defaultStyle
	if o.ActiveIndex == 1 {
		okStyle = activeStyle
	}
	NewText(o.Screen, "[ OK ]").WithOffset(1, o.Y+y+4).WithStyle(okStyle).Draw()
	cancelStyle := defaultStyle
	if o.ActiveIndex == 2 {
		cancelStyle = activeStyle
	}
	NewText(o.Screen, "[ Cancel ]").WithOffset(1, o.Y+y+5).WithStyle(cancelStyle).Draw()
}

func (o *Input) Up() {
	if o.ActiveIndex == 0 {
		o.ActiveIndex = 2
		return
	}
	o.ActiveIndex--
}

func (o *Input) Down() {
	if o.ActiveIndex == 2 {
		o.ActiveIndex = 0
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
			case tcell.KeyBacktab:
				o.Up()
			case tcell.KeyTab:
				o.Down()
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
			case tcell.KeyEscape:
				return o.Text, false
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
