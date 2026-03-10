package assets

// StatKey identifies an ability score.
type StatKey string

// StatKey constants for the six core ability scores.
const (
	StatSTR StatKey = "str"
	StatDEX StatKey = "dex"
	StatCON StatKey = "con"
	StatINT StatKey = "int"
	StatWIS StatKey = "wis"
	StatCHA StatKey = "cha"
)

// AllStatKeys defines the canonical display order for ability scores.
var AllStatKeys = []StatKey{StatSTR, StatDEX, StatCON, StatINT, StatWIS, StatCHA}
