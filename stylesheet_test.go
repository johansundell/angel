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
`
	got := colourViolations(css)
	want := []string{
		`24: colour "#FFF" outside token block: .bad-hex { color: #FFF; }`,
		`26: colour "rgba(" outside token block: box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.15);`,
		`28: colour "white" outside token block: .bad-name { background: white; }`,
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
// sits outside a token block. A token block is a rule whose selector starts
// with :root, at any nesting depth (so :root inside @media counts too).
func colourViolations(css string) []string {
	// Blank out comments but keep their newlines so line numbers hold.
	css = cssComment.ReplaceAllStringFunc(css, func(c string) string {
		return strings.Repeat("\n", strings.Count(c, "\n"))
	})

	var out []string
	depth := 0
	tokenDepth := -1 // depth of the open :root block, or -1 when outside one
	selector := ""
	for i, line := range strings.Split(css, "\n") {
		inTokens := tokenDepth >= 0
		for _, ch := range line {
			switch ch {
			case '{':
				if tokenDepth < 0 && strings.HasPrefix(strings.TrimSpace(selector), ":root") {
					tokenDepth = depth
					inTokens = true
				}
				depth++
				selector = ""
			case '}':
				depth--
				if depth == tokenDepth {
					tokenDepth = -1
				}
				selector = ""
			case ';':
				selector = ""
			default:
				selector += string(ch)
			}
		}
		if inTokens {
			continue
		}
		if c := firstColour(line); c != "" {
			out = append(out, fmt.Sprintf("%d: colour %q outside token block: %s", i+1, c, strings.TrimSpace(line)))
		}
	}
	return out
}

func firstColour(line string) string {
	line = cssCustomProp.ReplaceAllString(line, "")
	if m := cssColourValue.FindString(line); m != "" {
		return m
	}
	if m := cssNamedColour.FindStringSubmatch(line); m != nil {
		return m[1]
	}
	return ""
}
