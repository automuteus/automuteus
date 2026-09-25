package storage

// Int16ModeCount is one row of a grouped count keyed by a small integer, such as a color.
type Int16ModeCount struct {
	Count int64 `db:"count"`
	Mode  int16 `db:"mode"`
}

// StringModeCount is one row of a grouped count keyed by a string, such as an in-game name.
type StringModeCount struct {
	Count int64  `db:"count"`
	Mode  string `db:"mode"`
}
