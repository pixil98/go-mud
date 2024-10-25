package zones

type Room struct {
	Id          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Zone struct {
	Id   int     `json:"id"`
	Name string  `json:"name"`
	Room []*Room `json:"rooms"`
}
