package game

import (
	"errors"
	"math/rand"
	"strings"
)

var (
	ErrInvalidDimensions = errors.New("grid dimensions must be positive")
	ErrNoFreeCells       = errors.New("no free cells available for food")
)

// Direction represents the current heading of the snake.
type Direction int

const (
	DirectionUp Direction = iota
	DirectionDown
	DirectionLeft
	DirectionRight
)

type Point struct {
	X int
	Y int
}

// Snapshot is an immutable view of the current game state.
type Snapshot struct {
	Width     int
	Height    int
	Snake     []Point
	Direction Direction
	Food      Point
	Tick      int
	GameOver  bool
	Score     int
}

type Engine struct {
	width  int
	height int

	snake    []Point // head-first
	occupied map[Point]struct{}

	direction Direction
	pending   Direction
	hasInput  bool

	food Point
	rng  *rand.Rand

	tick     int
	score    int
	gameOver bool
}

func NewEngine(width, height int, seed int64) (*Engine, error) {
	if width <= 0 || height <= 0 {
		return nil, ErrInvalidDimensions
	}

	start := Point{X: width / 2, Y: height / 2}
	engine := &Engine{
		width:     width,
		height:    height,
		snake:     []Point{start},
		occupied:  map[Point]struct{}{start: {}},
		direction: DirectionRight,
		rng:       rand.New(rand.NewSource(seed)),
	}

	food, err := engine.randomEmptyCell()
	if err != nil {
		return nil, err
	}
	engine.food = food

	return engine, nil
}

func (e *Engine) QueueDirection(direction Direction) bool {
	if e.gameOver || !isValidDirection(direction) {
		return false
	}

	e.pending = direction
	e.hasInput = true
	return true
}

func (e *Engine) AdvanceTick() {
	if e.gameOver {
		return
	}

	if e.hasInput {
		e.direction = e.pending
		e.hasInput = false
	}

	next := e.nextHeadPosition()

	if next.X < 0 || next.X >= e.width || next.Y < 0 || next.Y >= e.height {
		e.gameOver = true
		return
	}

	if _, occupied := e.occupied[next]; occupied {
		e.gameOver = true
		return
	}

	ateFood := next == e.food
	e.moveSnake(next, ateFood)

	if ateFood {
		food, err := e.randomEmptyCell()
		if err != nil {
			e.gameOver = true
		} else {
			e.food = food
			e.score++
		}
	}

	e.tick++
}

func (e *Engine) Snapshot() Snapshot {
	body := make([]Point, len(e.snake))
	copy(body, e.snake)

	return Snapshot{
		Width:     e.width,
		Height:    e.height,
		Snake:     body,
		Direction: e.direction,
		Food:      e.food,
		Tick:      e.tick,
		GameOver:  e.gameOver,
		Score:     e.score,
	}
}

func (e *Engine) moveSnake(next Point, grew bool) {
	e.snake = append([]Point{next}, e.snake...)
	e.occupied[next] = struct{}{}

	if !grew {
		tail := e.snake[len(e.snake)-1]
		e.snake = e.snake[:len(e.snake)-1]
		delete(e.occupied, tail)
	}
}

func (e *Engine) nextHeadPosition() Point {
	head := e.snake[0]
	switch e.direction {
	case DirectionUp:
		return Point{X: head.X, Y: head.Y - 1}
	case DirectionDown:
		return Point{X: head.X, Y: head.Y + 1}
	case DirectionLeft:
		return Point{X: head.X - 1, Y: head.Y}
	default:
		return Point{X: head.X + 1, Y: head.Y}
	}
}

func (e *Engine) randomEmptyCell() (Point, error) {
	free := make([]Point, 0, e.width*e.height-len(e.snake))

	for y := 0; y < e.height; y++ {
		for x := 0; x < e.width; x++ {
			point := Point{X: x, Y: y}
			if _, taken := e.occupied[point]; !taken {
				free = append(free, point)
			}
		}
	}

	if len(free) == 0 {
		return Point{}, ErrNoFreeCells
	}

	return free[e.rng.Intn(len(free))], nil
}

func isValidDirection(direction Direction) bool {
	switch direction {
	case DirectionUp, DirectionDown, DirectionLeft, DirectionRight:
		return true
	default:
		return false
	}
}

func isOpposite(current, candidate Direction) bool {
	return current == DirectionUp && candidate == DirectionDown ||
		current == DirectionDown && candidate == DirectionUp ||
		current == DirectionLeft && candidate == DirectionRight ||
		current == DirectionRight && candidate == DirectionLeft
}

// ParseDirection converts a user-friendly direction string into a Direction enum.
// It accepts case-insensitive values: up, down, left, right.
func ParseDirection(value string) (Direction, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "up":
		return DirectionUp, true
	case "down":
		return DirectionDown, true
	case "left":
		return DirectionLeft, true
	case "right":
		return DirectionRight, true
	default:
		return Direction(0), false
	}
}
