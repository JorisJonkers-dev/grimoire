package domain

// Action kinds of legendary creatures.
const (
	ActionLegendaryAction     = "legendary_action"
	ActionLairAction          = "lair_action"
	ActionLegendaryResistance = "legendary_resistance"
)

// Legend is what makes a creature legendary in play, and what it has left: legendary actions it takes
// once another creature's turn ends (Ready), Uses a round; Legendary Resistance a day; lair actions on
// initiative count 20, once a round; mythic phases it enters at 0 hit points; and a damage threshold.
type Legend struct {
	Uses       int            `json:"uses"`
	Left       int            `json:"left"`
	Ready      bool           `json:"ready"`
	Actions    []LegendAction `json:"actions"`
	Resistance int            `json:"resistance"`
	ResistLeft int            `json:"resistLeft"`
	Lair       []LegendAction `json:"lair"`
	LairRound  int            `json:"lairRound"`
	Phases     []Phase        `json:"phases"`
	Phase      int            `json:"phase"`
	Threshold  int            `json:"threshold"`
}

// LegendAction is a legendary or lair action: what it is called, what it costs and what it does.
type LegendAction struct {
	Name string `json:"name"`
	Cost int    `json:"cost"`
	Text string `json:"text"`
}

// Phase is a mythic form a creature takes when it drops to 0 hit points.
type Phase struct {
	Name string `json:"name"`
	HP   int    `json:"hp"`
	Text string `json:"text"`
}
