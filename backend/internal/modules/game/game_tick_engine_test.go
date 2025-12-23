package game

import (
	"reflect"
	"testing"
)

type stubRandom struct {
	values []int
	index  int
}

func (s *stubRandom) Intn(n int) int {
	if n <= 0 {
		return 0
	}
	if len(s.values) == 0 {
		return 0
	}
	value := s.values[s.index%len(s.values)]
	s.index++
	return value % n
}

func newTestSnake(id string, body []Position, direction Direction) *Snake {
	return &Snake{
		ID:        id,
		Body:      body,
		Direction: direction,
		Alive:     true,
	}
}

func cloneState(state *GameState) *GameState {
	snakes := make(map[string]*Snake, len(state.Snakes))
	for id, snake := range state.Snakes {
		if snake == nil {
			continue
		}
		bodyCopy := append([]Position(nil), snake.Body...)
		snakes[id] = &Snake{
			ID:        snake.ID,
			Body:      bodyCopy,
			Direction: snake.Direction,
			Alive:     snake.Alive,
		}
	}

	return &GameState{
		Grid:   state.Grid,
		Snakes: snakes,
		Food:   append([]Position(nil), state.Food...),
	}
}

type snakeSnapshot struct {
	Alive     bool
	Body      []Position
	Direction Direction
}

type gameSnapshot struct {
	Food   []Position
	Snakes map[string]snakeSnapshot
}

func snapshotState(state *GameState) gameSnapshot {
	snakes := make(map[string]snakeSnapshot, len(state.Snakes))
	for id, snake := range state.Snakes {
		if snake == nil {
			continue
		}
		snakes[id] = snakeSnapshot{
			Alive:     snake.Alive,
			Body:      append([]Position(nil), snake.Body...),
			Direction: snake.Direction,
		}
	}

	return gameSnapshot{
		Food:   append([]Position(nil), state.Food...),
		Snakes: snakes,
	}
}

func TestTickConsumesFoodAndSpawnsNew(t *testing.T) {
	engine := NewTickEngineWithRandom(&stubRandom{values: []int{2}})
	state := &GameState{
		Grid: Grid{Width: 3, Height: 3},
		Snakes: map[string]*Snake{
			"s1": newTestSnake("s1", []Position{{0, 0}}, DirectionRight),
		},
		Food: []Position{{1, 0}},
	}

	result := engine.Tick(state)

	expectedBody := []Position{{1, 0}, {0, 0}}
	if got := state.Snakes["s1"].Body; !reflect.DeepEqual(expectedBody, got) {
		t.Fatalf("expected body %v, got %v", expectedBody, got)
	}

	if len(result.ConsumedFood) != 1 || !result.ConsumedFood[0].Equal(Position{X: 1, Y: 0}) {
		t.Fatalf("expected consumed food at (1,0), got %v", result.ConsumedFood)
	}

	expectedSpawn := Position{X: 1, Y: 1}
	if len(result.SpawnedFood) != 1 || !result.SpawnedFood[0].Equal(expectedSpawn) {
		t.Fatalf("expected spawned food at %v, got %v", expectedSpawn, result.SpawnedFood)
	}

	if len(state.Food) != 1 || !state.Food[0].Equal(expectedSpawn) {
		t.Fatalf("expected food list to contain %v, got %v", expectedSpawn, state.Food)
	}
}

func TestWallCollisionKillsSnake(t *testing.T) {
	engine := NewTickEngine(1)
	state := &GameState{
		Grid: Grid{Width: 2, Height: 2},
		Snakes: map[string]*Snake{
			"s1": newTestSnake("s1", []Position{{1, 0}}, DirectionRight),
		},
	}

	result := engine.Tick(state)

	if state.Snakes["s1"].Alive {
		t.Fatalf("expected snake to be dead after wall collision")
	}

	if len(state.Snakes["s1"].Body) != 0 {
		t.Fatalf("expected body to be cleared, got %v", state.Snakes["s1"].Body)
	}

	if len(result.DeadSnakes) != 1 || result.DeadSnakes[0] != "s1" {
		t.Fatalf("expected dead snakes [s1], got %v", result.DeadSnakes)
	}
}

func TestSelfCollisionKillsSnake(t *testing.T) {
	engine := NewTickEngine(1)
	state := &GameState{
		Grid: Grid{Width: 4, Height: 4},
		Snakes: map[string]*Snake{
			"s1": newTestSnake("s1", []Position{{1, 1}, {1, 2}, {1, 3}}, DirectionDown),
		},
	}

	result := engine.Tick(state)

	if state.Snakes["s1"].Alive {
		t.Fatalf("expected snake to die on self collision")
	}

	if len(result.DeadSnakes) != 1 || result.DeadSnakes[0] != "s1" {
		t.Fatalf("expected dead snakes [s1], got %v", result.DeadSnakes)
	}
}

func TestHeadOnCollisionKillsBoth(t *testing.T) {
	engine := NewTickEngine(1)
	state := &GameState{
		Grid: Grid{Width: 3, Height: 3},
		Snakes: map[string]*Snake{
			"a": newTestSnake("a", []Position{{0, 1}}, DirectionRight),
			"b": newTestSnake("b", []Position{{2, 1}}, DirectionLeft),
		},
	}

	result := engine.Tick(state)

	if state.Snakes["a"].Alive || state.Snakes["b"].Alive {
		t.Fatalf("expected both snakes to be dead after head-on collision")
	}

	if len(result.DeadSnakes) != 2 || !reflect.DeepEqual(result.DeadSnakes, []string{"a", "b"}) {
		t.Fatalf("expected dead snakes [a b], got %v", result.DeadSnakes)
	}
}

func TestCollisionWithOtherSnakeBody(t *testing.T) {
	engine := NewTickEngine(1)
	state := &GameState{
		Grid: Grid{Width: 3, Height: 3},
		Snakes: map[string]*Snake{
			"a": newTestSnake("a", []Position{{1, 1}, {1, 2}}, DirectionUp),
			"b": newTestSnake("b", []Position{{0, 1}}, DirectionRight),
		},
	}

	result := engine.Tick(state)

	if !state.Snakes["a"].Alive {
		t.Fatalf("expected snake a to survive")
	}

	expectedBody := []Position{{1, 0}, {1, 1}}
	if got := state.Snakes["a"].Body; !reflect.DeepEqual(expectedBody, got) {
		t.Fatalf("expected body %v, got %v", expectedBody, got)
	}

	if state.Snakes["b"].Alive {
		t.Fatalf("expected snake b to die after hitting snake a body")
	}

	if len(result.DeadSnakes) != 1 || result.DeadSnakes[0] != "b" {
		t.Fatalf("expected dead snakes [b], got %v", result.DeadSnakes)
	}
}

func TestDeterministicTicks(t *testing.T) {
	base := &GameState{
		Grid: Grid{Width: 4, Height: 4},
		Snakes: map[string]*Snake{
			"s1": newTestSnake("s1", []Position{{0, 0}}, DirectionRight),
			"s2": newTestSnake("s2", []Position{{3, 3}}, DirectionLeft),
		},
		Food: []Position{{1, 0}},
	}

	stateOne := cloneState(base)
	stateTwo := cloneState(base)

	engineOne := NewTickEngine(99)
	engineTwo := NewTickEngine(99)

	for tick := 0; tick < 3; tick++ {
		if tick == 1 {
			stateOne.Snakes["s1"].Direction = DirectionDown
			stateTwo.Snakes["s1"].Direction = DirectionDown
		}
		engineOne.Tick(stateOne)
		engineTwo.Tick(stateTwo)
	}

	if snapshotOne, snapshotTwo := snapshotState(stateOne), snapshotState(stateTwo); !reflect.DeepEqual(snapshotOne, snapshotTwo) {
		t.Fatalf("expected deterministic states, got %v and %v", snapshotOne, snapshotTwo)
	}
}
