package capture

// SpacemanColor type
type SpacemanColor int

// Colors const
const (
	RED    SpacemanColor = iota //0
	BLUE   SpacemanColor = iota //1
	GREEN  SpacemanColor = iota //2
	PINK   SpacemanColor = iota //3
	ORANGE SpacemanColor = iota //4
	YELLOW SpacemanColor = iota //5
	BLACK  SpacemanColor = iota //6
	WHITE  SpacemanColor = iota //7
	PURPLE SpacemanColor = iota //8
	BROWN  SpacemanColor = iota //9
	CYAN   SpacemanColor = iota //10
	LIME   SpacemanColor = iota //11
	NULL   SpacemanColor = iota //12
)

// RGBColor struct
type RGBColor struct {
	r float64
	g float64
	b float64
}

// WithinAcceptableRange displays are darker or lighter, this approach (probably) WILL NOT WORK. Needs proper sorting by distance
func WithinAcceptableRange(testColor, baseColor RGBColor, percentDiff float64) bool {
	redInRange := testColor.r > baseColor.r-(baseColor.r*percentDiff)
	blueInRange := testColor.b > baseColor.b-(baseColor.b*percentDiff)
	greenInRange := testColor.g > baseColor.g-(baseColor.g*percentDiff)
	return redInRange && greenInRange && blueInRange
}

// PercentDiff const
const PercentDiff = 0.05

// BestColorMatch returns the best color match kekw
func BestColorMatch(color RGBColor) (SpacemanColor, bool) {
	for sc, v := range AllSpacemanColors {
		if WithinAcceptableRange(color, v.bright, PercentDiff) {
			return sc, true
		} else if WithinAcceptableRange(color, v.dim, PercentDiff) {
			return sc, false
		}
	}
	return NULL, false
}

// ColorPair struct
type ColorPair struct {
	dim    RGBColor
	bright RGBColor
}

// SpacemanColors map a single spaceman color to an array with 2 values within; the "dim" color, and the "bright" variant
//(dim is for discussion phase and dead players, and bright is for voting phase)
type SpacemanColors map[SpacemanColor]ColorPair

// AllSpacemanColors variable with all the juice
var AllSpacemanColors = SpacemanColors{
	RED: ColorPair{
		RGBColor{
			r: 127,
			g: 59,
			b: 66,
		},
		RGBColor{
			r: 193,
			g: 65,
			b: 72,
		},
	},
	BLUE: ColorPair{
		RGBColor{
			r: 51,
			g: 70,
			b: 144,
		},
		RGBColor{
			r: 57,
			g: 85,
			b: 214,
		},
	},
	GREEN: ColorPair{
		RGBColor{
			r: 50,
			g: 104,
			b: 77,
		},
		RGBColor{
			r: 60,
			g: 149,
			b: 94,
		},
	},
	PINK: ColorPair{
		RGBColor{
			r: 141,
			g: 85,
			b: 135,
		},
		RGBColor{
			r: 219,
			g: 114,
			b: 196,
		},
	},
	ORANGE: ColorPair{
		RGBColor{
			r: 147,
			g: 105,
			b: 65,
		},
		RGBColor{
			r: 227,
			g: 147,
			b: 70,
		},
	},
	YELLOW: ColorPair{
		RGBColor{
			r: 148,
			g: 154,
			b: 95,
		},
		RGBColor{
			r: 231,
			g: 237,
			b: 125,
		},
	},
	BLACK: ColorPair{
		RGBColor{
			r: 73,
			g: 82,
			b: 92,
		},
		RGBColor{
			r: 92,
			g: 105,
			b: 117,
		},
	},
	WHITE: ColorPair{
		RGBColor{
			r: 131,
			g: 142,
			b: 156,
		},
		RGBColor{
			r: 208,
			g: 222,
			b: 241,
		},
	},
	PURPLE: ColorPair{
		RGBColor{
			r: 90,
			g: 71,
			b: 137,
		},
		RGBColor{
			r: 127,
			g: 88,
			b: 201,
		},
	},
	BROWN: ColorPair{
		RGBColor{
			r: 93,
			g: 82,
			b: 72,
		},
		RGBColor{
			r: 130,
			g: 107,
			b: 81,
		},
	},
	CYAN: ColorPair{
		RGBColor{
			r: 66,
			g: 155,
			b: 149,
		},
		RGBColor{
			r: 89,
			g: 244,
			b: 226,
		},
	},
	LIME: ColorPair{
		RGBColor{
			r: 79,
			g: 152,
			b: 83,
		},
		RGBColor{
			r: 105,
			g: 232,
			b: 102,
		},
	},
}
