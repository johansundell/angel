package main

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// TestStylesheetColoursOnlyInTokens keeps every colour in the shipped
// stylesheet inside the :root token block(s), so a theme only has to redefine
// the tokens and no colour can be left behind.
func TestStylesheetColoursOnlyInTokens(t *testing.T) {
	css, err := embeddedAssets.ReadFile("assets/css/main.css")
	if err != nil {
		t.Fatalf("read stylesheet: %v", err)
	}
	for _, v := range colourViolations(string(css)) {
		t.Errorf("assets/css/main.css:%s", v)
	}
}

func TestColourViolations(t *testing.T) {
	css := `:root {
    --bg: #ffffff;
    --shadow: 0 1px 2px rgba(0, 0, 0, 0.1);
}

@media (prefers-color-scheme: dark) {
    :root:not([data-theme="light"]) {
        --bg: black;
    }
}

:root[data-theme="dark"] {
    --bg: hsl(0, 0%, 10%);
}

/* A comment may say #fff or white. */
.ok {
    color: var(--fg);
    border-color: var(--important-red);
    font-family: "Helvetica Neue", sans-serif;
    background: currentColor;
}

.bad-hex { color: #FFF; }
.bad-rgb {
    box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.15);
}
.bad-name { background: white; }
#add { color: var(--fg); }
:root { --a: #000; } .after-root { color: #111; }
.before-root { color: #222; } :root { --b: #333; }
:root, .mixed { color: #444; }
.a,
:root { --c: #555; }
`
	got := colourViolations(css)
	want := []string{
		`24: colour "#FFF" outside token block: .bad-hex { color: #FFF; }`,
		`26: colour "rgba(" outside token block: box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.15);`,
		`28: colour "white" outside token block: .bad-name { background: white; }`,
		`30: colour "#111" outside token block: :root { --a: #000; } .after-root { color: #111; }`,
		`31: colour "#222" outside token block: .before-root { color: #222; } :root { --b: #333; }`,
		`32: colour "#444" outside token block: :root, .mixed { color: #444; }`,
		`34: colour "#555" outside token block: :root { --c: #555; }`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("violations:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

var (
	cssComment     = regexp.MustCompile(`(?s)/\*.*?\*/`)
	cssCustomProp  = regexp.MustCompile(`--[\w-]+`)
	cssColourValue = regexp.MustCompile(`(?i)#[0-9a-f]{3,8}\b|\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch|color)\(`)
	cssNamedColour = regexp.MustCompile(`(?i)(?:^|[^\w-])(` + strings.Join(cssColourNames, "|") + `)(?:[^\w-]|$)`)
)

// cssColourNames are the CSS named colours plus transparent. currentColor is
// left out: it follows the text colour, which already comes from a token.
var cssColourNames = strings.Fields(`
	aliceblue antiquewhite aqua aquamarine azure beige bisque black
	blanchedalmond blue blueviolet brown burlywood cadetblue chartreuse
	chocolate coral cornflowerblue cornsilk crimson cyan darkblue darkcyan
	darkgoldenrod darkgray darkgreen darkgrey darkkhaki darkmagenta
	darkolivegreen darkorange darkorchid darkred darksalmon darkseagreen
	darkslateblue darkslategray darkslategrey darkturquoise darkviolet
	deeppink deepskyblue dimgray dimgrey dodgerblue firebrick floralwhite
	forestgreen fuchsia gainsboro ghostwhite gold goldenrod gray green
	greenyellow grey honeydew hotpink indianred indigo ivory khaki lavender
	lavenderblush lawngreen lemonchiffon lightblue lightcoral lightcyan
	lightgoldenrodyellow lightgray lightgreen lightgrey lightpink lightsalmon
	lightseagreen lightskyblue lightslategray lightslategrey lightsteelblue
	lightyellow lime limegreen linen magenta maroon mediumaquamarine
	mediumblue mediumorchid mediumpurple mediumseagreen mediumslateblue
	mediumspringgreen mediumturquoise mediumvioletred midnightblue mintcream
	mistyrose moccasin navajowhite navy oldlace olive olivedrab orange
	orangered orchid palegoldenrod palegreen paleturquoise palevioletred
	papayawhip peachpuff peru pink plum powderblue purple rebeccapurple red
	rosybrown royalblue saddlebrown salmon sandybrown seagreen seashell
	sienna silver skyblue slateblue slategray slategrey snow springgreen
	steelblue tan teal thistle tomato turquoise violet wheat white whitesmoke
	yellow yellowgreen transparent
`)

// colourViolations returns "line: message" for every colour value in css that
// sits outside a token block. A token block is a rule whose selectors are all
// :root, at any nesting depth (so :root inside @media counts too). Selectors
// are never checked, so an ID like #add is not mistaken for a colour.
func colourViolations(css string) []string {
	// Blank out comments but keep their newlines so line numbers hold.
	css = cssComment.ReplaceAllStringFunc(css, func(c string) string {
		return strings.Repeat("\n", strings.Count(c, "\n"))
	})
	lines := strings.Split(css, "\n")

	var out []string
	depth := 0
	tokenDepth := -1 // depth of the open :root block, or -1 when outside one
	pending := ""    // text since the last {, } or ;
	line := 1        // line of the current character
	startLine := 1   // line where pending starts
	check := func() {
		if tokenDepth >= 0 {
			return
		}
		c, offset := firstColour(pending)
		if c == "" {
			return
		}
		n := startLine + strings.Count(pending[:offset], "\n")
		out = append(out, fmt.Sprintf("%d: colour %q outside token block: %s", n, c, strings.TrimSpace(lines[n-1])))
	}
	for _, ch := range css {
		switch ch {
		case '{':
			if tokenDepth < 0 && isRootSelector(pending) {
				tokenDepth = depth
			}
			depth++
			pending = ""
		case '}':
			check()
			depth--
			if depth == tokenDepth {
				tokenDepth = -1
			}
			pending = ""
		case ';':
			check()
			pending = ""
		default:
			if pending == "" {
				startLine = line
			}
			pending += string(ch)
		}
		if ch == '\n' {
			line++
		}
	}
	return out
}

func isRootSelector(selectors string) bool {
	for _, sel := range strings.Split(selectors, ",") {
		if !strings.HasPrefix(strings.TrimSpace(sel), ":root") {
			return false
		}
	}
	return true
}

// firstColour returns the first colour value in decl and its byte offset, or
// "" when there is none.
func firstColour(decl string) (string, int) {
	// Blank custom property names (keeping offsets) so --important-red is
	// not read as red.
	decl = cssCustomProp.ReplaceAllStringFunc(decl, func(p string) string {
		return strings.Repeat(" ", len(p))
	})
	if loc := cssColourValue.FindStringIndex(decl); loc != nil {
		return decl[loc[0]:loc[1]], loc[0]
	}
	if m := cssNamedColour.FindStringSubmatchIndex(decl); m != nil {
		return decl[m[2]:m[3]], m[2]
	}
	return "", 0
}
