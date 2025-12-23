package game

// Direction represents the movement direction of a snake.
type Direction int

const (
	DirectionUp Direction = iota
	DirectionRight
	DirectionDown
	DirectionLeft
)

// Position is a coordinate on the game grid.
type Position struct {
	X int
	Y int
}

// Equal reports whether two positions are identical.
func (p Position) Equal(other Position) bool {
	return p.X == other.X && p.Y == other.Y
}

// Delta returns the positional delta for the direction.
func (d Direction) Delta() Position {
	switch d {
	case DirectionUp:
		return Position{X: 0, Y: -1}
	case DirectionRight:
		return Position{X: 1, Y: 0}
	case DirectionDown:
		return Position{X: 0, Y: 1}
	case DirectionLeft:
		return Position{X: -1, Y: 0}
	default:
		return Position{}
	}
}

// Grid defines the playfield dimensions.
type Grid struct {
	Width  int
	Height int
}

// InBounds reports whether a position is inside the grid.
func (g Grid) InBounds(p Position) bool {
	return p.X >= 0 && p.X < g.Width && p.Y >= 0 && p.Y < g.Height
}

// Snake represents a player's snake.
// Body is ordered head (index 0) to tail (last index).
type Snake struct {
	ID        string
	Body      []Position
	Direction Direction
	Alive     bool
}

// GameState captures the entire game board for a tick.
type GameState struct {
	Grid   Grid
	Snakes map[string]*Snake
	Food   []Position
}
