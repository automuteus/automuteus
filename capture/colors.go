package capture

type SpacemanColor int

const (
	RED    SpacemanColor = iota //DONE
	BLUE   SpacemanColor = iota //DONE
	GREEN  SpacemanColor = iota //NEED DIM
	PINK   SpacemanColor = iota //NEED BOTH
	ORANGE SpacemanColor = iota //NEED BRIGHT
	YELLOW SpacemanColor = iota //NEED BRIGHT
	BLACK  SpacemanColor = iota //NEED BRIGHT
	WHITE  SpacemanColor = iota //NEED BRIGHT
	PURPLE SpacemanColor = iota //NEED BRIGHT
	BROWN  SpacemanColor = iota //NEED BRIGHT
	CYAN   SpacemanColor = iota //NEED BRIGHT
	LIME   SpacemanColor = iota //NEED BOTH
)

type RGBColor struct {
	r uint32
	g uint32
	b uint32
}

type ColorPair struct {
	dim    RGBColor
	bright RGBColor
}

//Map a single spaceman color to an array with 2 values within; the "dim" color, and the "bright" variant
//(dim is for discussion phase and dead players, and bright is for voting phase)
type SpacemanColors map[SpacemanColor]ColorPair

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
			r: 0,
			g: 0,
			b: 0,
		},
		RGBColor{
			r: 60,
			g: 149,
			b: 94,
		},
	},
	PINK: ColorPair{
		RGBColor{
			r: 0,
			g: 0,
			b: 0,
		},
		RGBColor{
			r: 0,
			g: 0,
			b: 0,
		},
	},
	ORANGE: ColorPair{
		RGBColor{
			r: 147,
			g: 105,
			b: 65,
		},
		RGBColor{
			r: 0,
			g: 0,
			b: 0,
		},
	},
	YELLOW: ColorPair{
		RGBColor{
			r: 148,
			g: 154,
			b: 95,
		},
		RGBColor{
			r: 0,
			g: 0,
			b: 0,
		},
	},
	BLACK: ColorPair{
		RGBColor{
			r: 73,
			g: 82,
			b: 92,
		},
		RGBColor{
			r: 0,
			g: 0,
			b: 0,
		},
	},
	WHITE: ColorPair{
		RGBColor{
			r: 131,
			g: 142,
			b: 156,
		},
		RGBColor{
			r: 0,
			g: 0,
			b: 0,
		},
	},
	PURPLE: ColorPair{
		RGBColor{
			r: 90,
			g: 71,
			b: 137,
		},
		RGBColor{
			r: 0,
			g: 0,
			b: 0,
		},
	},
	BROWN: ColorPair{
		RGBColor{
			r: 93,
			g: 82,
			b: 72,
		},
		RGBColor{
			r: 0,
			g: 0,
			b: 0,
		},
	},
	CYAN: ColorPair{
		RGBColor{
			r: 66,
			g: 155,
			b: 149,
		},
		RGBColor{
			r: 0,
			g: 0,
			b: 0,
		},
	},
	LIME: ColorPair{
		RGBColor{
			r: 0,
			g: 0,
			b: 0,
		},
		RGBColor{
			r: 0,
			g: 0,
			b: 0,
		},
	},
}
