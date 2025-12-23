package game

import (
	"math/rand"
	"sort"
)

// RandomSource allows injecting deterministic randomness for testing and replay.
type RandomSource interface {
	Intn(n int) int
}

// TickEngine advances the game state by one tick in a deterministic way.
type TickEngine struct {
	random RandomSource
}

// NewTickEngine creates a tick engine using the provided seed.
func NewTickEngine(seed int64) *TickEngine {
	return &TickEngine{
		random: rand.New(rand.NewSource(seed)),
	}
}

// NewTickEngineWithRandom allows supplying a custom deterministic random source.
func NewTickEngineWithRandom(random RandomSource) *TickEngine {
	if random == nil {
		random = rand.New(rand.NewSource(0))
	}
	return &TickEngine{
		random: random,
	}
}

// TickResult captures the outcomes of a tick.
type TickResult struct {
	DeadSnakes   []string
	ConsumedFood []Position
	SpawnedFood  []Position
}

type movePlan struct {
	snake    *Snake
	nextHead Position
	willGrow bool
	dead     bool
	nextBody []Position
}

// Tick advances the game state by one step and returns the resulting events.
func (e *TickEngine) Tick(state *GameState) TickResult {
	if state == nil || e == nil || e.random == nil {
		return TickResult{}
	}

	result := TickResult{}
	foodSet := buildPositionSet(state.Food)
	moves := make(map[string]*movePlan, len(state.Snakes))
	ids := sortedSnakeIDs(state.Snakes)

	for _, id := range ids {
		snake := state.Snakes[id]
		if snake == nil || !snake.Alive || len(snake.Body) == 0 {
			continue
		}

		nextHead := addPosition(snake.Body[0], snake.Direction.Delta())
		willGrow := foodSet[nextHead]
		plan := &movePlan{
			snake:    snake,
			nextHead: nextHead,
			willGrow: willGrow,
		}

		if !state.Grid.InBounds(nextHead) || collidesWithBody(snake.Body, nextHead, !willGrow) {
			plan.dead = true
		}

		moves[id] = plan
	}

	applyHeadToHeadCollisions(moves, ids)
	prepareFutureBodies(moves, ids)
	applyBodyCollisions(moves, ids)

	consumedFood := collectConsumedFood(moves, ids)
	if len(consumedFood) > 0 {
		state.Food = filterFood(state.Food, consumedFood)
		result.ConsumedFood = append(result.ConsumedFood, consumedFood...)
	}

	for _, id := range ids {
		plan, ok := moves[id]
		if !ok {
			continue
		}

		if plan.dead {
			if plan.snake.Alive {
				result.DeadSnakes = append(result.DeadSnakes, id)
			}
			plan.snake.Alive = false
			plan.snake.Body = nil
			continue
		}

		plan.snake.Body = plan.nextBody
	}

	for range result.ConsumedFood {
		if spawned, ok := e.spawnFood(state); ok {
			result.SpawnedFood = append(result.SpawnedFood, spawned)
			state.Food = append(state.Food, spawned)
		}
	}

	return result
}

func applyHeadToHeadCollisions(moves map[string]*movePlan, ids []string) {
	headCounts := map[Position]int{}
	for _, id := range ids {
		if plan, ok := moves[id]; ok && !plan.dead {
			headCounts[plan.nextHead]++
		}
	}

	for _, id := range ids {
		if plan, ok := moves[id]; ok && !plan.dead && headCounts[plan.nextHead] > 1 {
			plan.dead = true
		}
	}
}

func prepareFutureBodies(moves map[string]*movePlan, ids []string) {
	for _, id := range ids {
		plan, ok := moves[id]
		if !ok || plan.dead {
			continue
		}

		plan.nextBody = buildNextBody(plan.snake.Body, plan.nextHead, plan.willGrow)
	}
}

func applyBodyCollisions(moves map[string]*movePlan, ids []string) {
	occupiedAfterMove := map[string]map[Position]bool{}
	for _, id := range ids {
		if plan, ok := moves[id]; ok && !plan.dead {
			occupiedAfterMove[id] = buildPositionSet(plan.nextBody)
		}
	}

	for _, id := range ids {
		plan, ok := moves[id]
		if !ok || plan.dead {
			continue
		}

		for otherID, bodySet := range occupiedAfterMove {
			if otherID == id {
				continue
			}
			if bodySet[plan.nextHead] {
				plan.dead = true
				break
			}
		}
	}
}

func collectConsumedFood(moves map[string]*movePlan, ids []string) []Position {
	consumed := make([]Position, 0)
	for _, id := range ids {
		if plan, ok := moves[id]; ok && !plan.dead && plan.willGrow {
			consumed = append(consumed, plan.nextHead)
		}
	}
	return consumed
}

func filterFood(current []Position, consumed []Position) []Position {
	if len(consumed) == 0 {
		return current
	}

	consumedSet := buildPositionSet(consumed)
	remaining := make([]Position, 0, len(current))

	for _, food := range current {
		if !consumedSet[food] {
			remaining = append(remaining, food)
		}
	}

	return remaining
}

func (e *TickEngine) spawnFood(state *GameState) (Position, bool) {
	occupied := buildPositionSet(state.Food)
	ids := sortedSnakeIDs(state.Snakes)
	for _, id := range ids {
		snake := state.Snakes[id]
		if snake == nil || !snake.Alive {
			continue
		}
		for _, segment := range snake.Body {
			occupied[segment] = true
		}
	}

	available := make([]Position, 0, state.Grid.Width*state.Grid.Height-len(occupied))
	for y := 0; y < state.Grid.Height; y++ {
		for x := 0; x < state.Grid.Width; x++ {
			pos := Position{X: x, Y: y}
			if !occupied[pos] {
				available = append(available, pos)
			}
		}
	}

	if len(available) == 0 {
		return Position{}, false
	}

	index := e.random.Intn(len(available))
	return available[index], true
}

func collidesWithBody(body []Position, target Position, ignoreTail bool) bool {
	limit := len(body)
	if ignoreTail && limit > 0 {
		limit--
	}

	for i := 0; i < limit; i++ {
		if body[i].Equal(target) {
			return true
		}
	}

	return false
}

func buildNextBody(body []Position, nextHead Position, grow bool) []Position {
	next := make([]Position, 0, len(body)+1)
	next = append(next, nextHead)
	next = append(next, body...)
	if !grow && len(next) > 0 {
		next = next[:len(next)-1]
	}
	return next
}

func buildPositionSet(positions []Position) map[Position]bool {
	result := make(map[Position]bool, len(positions))
	for _, p := range positions {
		result[p] = true
	}
	return result
}

func addPosition(a, b Position) Position {
	return Position{X: a.X + b.X, Y: a.Y + b.Y}
}

func sortedSnakeIDs(snakes map[string]*Snake) []string {
	ids := make([]string, 0, len(snakes))
	for id := range snakes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
