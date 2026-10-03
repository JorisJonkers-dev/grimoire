package domain

// Camera modes of the Table Display.
const (
	CameraFollowTurn = "follow_turn"
	CameraShowParty  = "show_party"
	CameraFree       = "free"
)

// Scenes the Table Display can show.
const (
	SceneLocal   = "local"
	SceneWorld   = "world"
	SceneHandout = "handout"
	SceneTitle   = "title"
)

// TableDisplay is what the DM has the shared screen show. Q, R and ZoomPct steer the free camera; the
// world scene shows MapID; the handout and title scenes show Title and Body.
type TableDisplay struct {
	Camera   string
	Q        int
	R        int
	ZoomPct  int
	Scene    string
	Title    string
	Body     string
	MapID    *MapID
	Blackout bool
	// Caption is a line the DM puts on the Table Display.
	Caption string
}

// DefaultTable follows the turn on the local map at normal zoom.
func DefaultTable() TableDisplay {
	return TableDisplay{Camera: CameraFollowTurn, ZoomPct: 100, Scene: SceneLocal}
}

// ActionTableSet is the Action Log kind for a change to the Table Display.
const ActionTableSet = "table_set"
