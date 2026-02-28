package game

import (
	"math/rand"
	"sort"
)

// Engine advances game state ticks deterministically for a given RNG seed.
type Engine struct {
	rng *rand.Rand
}

// NewEngine creates a tick engine with a seeded random source.
func NewEngine(seed int64) *Engine {
	return &Engine{
		rng: rand.New(rand.NewSource(seed)),
	}
}

// BufferInput stores the next direction to be applied on the next tick.
// Reverse direction inputs are ignored.
func (e *Engine) BufferInput(state *GameState, playerID string, direction Direction) {
	if state == nil || !direction.valid() {
		return
	}
	e.syncSnakesWithPlayers(state)
	snake, ok := state.Snakes[playerID]
	if !ok || snake == nil || !snake.Alive || len(snake.Body) == 0 {
		return
	}
	if direction.opposite(snake.Direction) {
		return
	}
	if state.BufferedInputs == nil {
		state.BufferedInputs = make(map[string]Direction)
	}
	state.BufferedInputs[playerID] = direction
}

// AdvanceTick applies buffered inputs, moves snakes, resolves collisions,
// handles food consumption, and increments the global tick counter.
func (e *Engine) AdvanceTick(state *GameState) {
	if state == nil {
		return
	}
	if state.Snakes == nil {
		state.Snakes = make(map[string]*Snake)
	}
	if state.Players == nil {
		state.Players = make(map[string]*Player)
	}
	if state.BufferedInputs == nil {
		state.BufferedInputs = make(map[string]Direction)
	}
	e.syncSnakesWithPlayers(state)

	ids := sortedSnakeIDs(state.Snakes)

	for _, id := range ids {
		snake := state.Snakes[id]
		if snake == nil || !snake.Alive || len(snake.Body) == 0 {
			continue
		}
		nextDirection, ok := state.BufferedInputs[id]
		if !ok {
			continue
		}
		if nextDirection.valid() && !nextDirection.opposite(snake.Direction) {
			snake.Direction = nextDirection
		}
	}
	clear(state.BufferedInputs)

	nextHeads := make(map[string]Position, len(ids))
	nextBodies := make(map[string][]Position, len(ids))
	grows := make(map[string]bool, len(ids))
	dead := make(map[string]bool, len(ids))

	for _, id := range ids {
		snake := state.Snakes[id]
		if snake == nil || !snake.Alive || len(snake.Body) == 0 {
			continue
		}

		head := moveOne(snake.Body[0], snake.Direction)
		nextHeads[id] = head
		grow := state.Food != nil && head == state.Food.Position
		grows[id] = grow

		nextBody := make([]Position, 0, len(snake.Body)+1)
		nextBody = append(nextBody, head)
		nextBody = append(nextBody, snake.Body...)
		if !grow {
			nextBody = nextBody[:len(nextBody)-1]
		}
		nextBodies[id] = nextBody

		if !inBounds(head, state.Width, state.Height) {
			dead[id] = true
		}
	}

	for _, id := range ids {
		if dead[id] {
			continue
		}
		body, ok := nextBodies[id]
		if !ok || len(body) == 0 {
			continue
		}
		head := body[0]
		for i := 1; i < len(body); i++ {
			if body[i] == head {
				dead[id] = true
				break
			}
		}
	}

	for _, id := range ids {
		if dead[id] {
			continue
		}
		head, ok := nextHeads[id]
		if !ok {
			continue
		}

		for _, otherID := range ids {
			if otherID == id {
				continue
			}
			otherBody, ok := nextBodies[otherID]
			if !ok {
				continue
			}
			for _, segment := range otherBody {
				if head == segment {
					dead[id] = true
					break
				}
			}
			if dead[id] {
				break
			}
		}
	}

	ateFood := false

	for _, id := range ids {
		snake := state.Snakes[id]
		if snake == nil || !snake.Alive || len(snake.Body) == 0 {
			continue
		}

		if dead[id] {
			snake.Alive = false
			snake.Body = nil
			continue
		}

		snake.Body = nextBodies[id]
		if grows[id] {
			ateFood = true
			if player := state.Players[id]; player != nil {
				player.Score++
			}
		}
	}

	if ateFood {
		state.Food = e.spawnFood(state)
	}

	state.Tick++
}

func (e *Engine) spawnFood(state *GameState) *Food {
	if state == nil || state.Width <= 0 || state.Height <= 0 {
		return nil
	}
	e.syncSnakesWithPlayers(state)

	occupied := make(map[Position]struct{})
	for _, id := range sortedSnakeIDs(state.Snakes) {
		snake := state.Snakes[id]
		if snake == nil || !snake.Alive {
			continue
		}
		for _, segment := range snake.Body {
			occupied[segment] = struct{}{}
		}
	}

	empty := make([]Position, 0, state.Width*state.Height-len(occupied))
	for y := 0; y < state.Height; y++ {
		for x := 0; x < state.Width; x++ {
			pos := Position{X: x, Y: y}
			if _, exists := occupied[pos]; exists {
				continue
			}
			empty = append(empty, pos)
		}
	}

	if len(empty) == 0 {
		return nil
	}
	return &Food{Position: empty[e.rng.Intn(len(empty))]}
}

func (e *Engine) syncSnakesWithPlayers(state *GameState) {
	if state == nil {
		return
	}
	if state.Snakes == nil {
		state.Snakes = make(map[string]*Snake)
	}
	if state.Players == nil {
		return
	}

	for playerID, player := range state.Players {
		if player == nil {
			continue
		}

		snake := state.Snakes[playerID]
		if snake == nil && player.Snake != nil {
			snake = player.Snake
			state.Snakes[playerID] = snake
		}
		if snake == nil {
			continue
		}
		if snake.PlayerID == "" {
			snake.PlayerID = playerID
		}
		if player.Snake == nil {
			player.Snake = snake
		}
	}
}

func sortedSnakeIDs(snakes map[string]*Snake) []string {
	ids := make([]string, 0, len(snakes))
	for id := range snakes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func inBounds(pos Position, width, height int) bool {
	return pos.X >= 0 && pos.X < width && pos.Y >= 0 && pos.Y < height
}

func moveOne(pos Position, direction Direction) Position {
	switch direction {
	case DirectionUp:
		return Position{X: pos.X, Y: pos.Y - 1}
	case DirectionDown:
		return Position{X: pos.X, Y: pos.Y + 1}
	case DirectionLeft:
		return Position{X: pos.X - 1, Y: pos.Y}
	case DirectionRight:
		return Position{X: pos.X + 1, Y: pos.Y}
	default:
		return pos
	}
}
