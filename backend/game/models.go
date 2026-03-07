package game

// Direction represents a snake movement direction.
type Direction string

const (
	DirectionUp    Direction = "up"
	DirectionDown  Direction = "down"
	DirectionLeft  Direction = "left"
	DirectionRight Direction = "right"
)

// Position is a grid coordinate.
type Position struct {
	X int
	Y int
}

// Snake is a player-controlled snake.
type Snake struct {
	PlayerID  string
	Body      []Position
	Direction Direction
	Alive     bool
}

// Food is a consumable item on the grid.
type Food struct {
	Position Position
}

// Player holds player identity and score.
type Player struct {
	ID    string
	Name  string
	Snake *Snake
	Score int
}

// Room is a game room container holding a single deterministic game state.
type Room struct {
	ID    string
	State *GameState
}

// GameState is the full room state for deterministic simulation.
type GameState struct {
	Width          int
	Height         int
	Snakes         map[string]*Snake
	Players        map[string]*Player
	Food           *Food
	Tick           uint64
	BufferedInputs map[string]Direction
}

// NewGameState creates an empty game state with initialized maps.
func NewGameState(width, height int) *GameState {
	return &GameState{
		Width:          width,
		Height:         height,
		Snakes:         make(map[string]*Snake),
		Players:        make(map[string]*Player),
		BufferedInputs: make(map[string]Direction),
	}
}

func (d Direction) valid() bool {
	switch d {
	case DirectionUp, DirectionDown, DirectionLeft, DirectionRight:
		return true
	default:
		return false
	}
}

func (d Direction) opposite(other Direction) bool {
	return (d == DirectionUp && other == DirectionDown) ||
		(d == DirectionDown && other == DirectionUp) ||
		(d == DirectionLeft && other == DirectionRight) ||
		(d == DirectionRight && other == DirectionLeft)
}
