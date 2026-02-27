package game

import "testing"

func TestAdvanceTickMovesSnakeOneCell(t *testing.T) {
	engine := NewEngine(1)
	state := NewGameState(8, 8)
	state.Players["p1"] = &Player{ID: "p1", Name: "P1"}
	state.Snakes["p1"] = &Snake{
		PlayerID:  "p1",
		Body:      []Position{{X: 2, Y: 2}},
		Direction: DirectionRight,
		Alive:     true,
	}

	engine.AdvanceTick(state)

	if got, want := state.Tick, uint64(1); got != want {
		t.Fatalf("tick mismatch: got %d want %d", got, want)
	}
	head := state.Snakes["p1"].Body[0]
	if got, want := head, (Position{X: 3, Y: 2}); got != want {
		t.Fatalf("head mismatch: got %+v want %+v", got, want)
	}
}

func TestBufferedInputAppliesOnNextTickAndNoReverse(t *testing.T) {
	engine := NewEngine(2)
	state := NewGameState(8, 8)
	state.Players["p1"] = &Player{ID: "p1", Name: "P1"}
	state.Snakes["p1"] = &Snake{
		PlayerID:  "p1",
		Body:      []Position{{X: 4, Y: 4}, {X: 3, Y: 4}},
		Direction: DirectionRight,
		Alive:     true,
	}

	engine.BufferInput(state, "p1", DirectionUp)
	engine.AdvanceTick(state)

	snake := state.Snakes["p1"]
	if got, want := snake.Direction, DirectionUp; got != want {
		t.Fatalf("direction mismatch after first tick: got %s want %s", got, want)
	}
	if got, want := snake.Body[0], (Position{X: 4, Y: 3}); got != want {
		t.Fatalf("head mismatch after first tick: got %+v want %+v", got, want)
	}

	engine.BufferInput(state, "p1", DirectionDown)
	engine.AdvanceTick(state)

	snake = state.Snakes["p1"]
	if got, want := snake.Direction, DirectionUp; got != want {
		t.Fatalf("reverse direction should be ignored: got %s want %s", got, want)
	}
	if got, want := snake.Body[0], (Position{X: 4, Y: 2}); got != want {
		t.Fatalf("head mismatch after reverse attempt: got %+v want %+v", got, want)
	}
}

func TestWallCollisionKillsSnakeAtBoundaryLengthOne(t *testing.T) {
	engine := NewEngine(3)
	state := NewGameState(5, 5)
	state.Players["p1"] = &Player{ID: "p1", Name: "P1"}
	state.Snakes["p1"] = &Snake{
		PlayerID:  "p1",
		Body:      []Position{{X: 0, Y: 0}},
		Direction: DirectionLeft,
		Alive:     true,
	}

	engine.AdvanceTick(state)

	snake := state.Snakes["p1"]
	if snake.Alive {
		t.Fatalf("snake should be dead after wall collision")
	}
	if snake.Body != nil {
		t.Fatalf("dead snake body should be cleared, got %+v", snake.Body)
	}
}

func TestSelfCollisionKillsSnake(t *testing.T) {
	engine := NewEngine(4)
	state := NewGameState(8, 8)
	state.Players["p1"] = &Player{ID: "p1", Name: "P1"}
	state.Snakes["p1"] = &Snake{
		PlayerID: "p1",
		Body: []Position{
			{X: 2, Y: 2},
			{X: 2, Y: 3},
			{X: 1, Y: 3},
			{X: 1, Y: 2},
		},
		Direction: DirectionDown,
		Alive:     true,
	}

	engine.AdvanceTick(state)

	if state.Snakes["p1"].Alive {
		t.Fatalf("snake should be dead after self collision")
	}
}

func TestOtherSnakeCollisionKillsCollidingSnake(t *testing.T) {
	engine := NewEngine(5)
	state := NewGameState(8, 8)
	state.Players["a"] = &Player{ID: "a", Name: "A"}
	state.Players["b"] = &Player{ID: "b", Name: "B"}
	state.Snakes["a"] = &Snake{
		PlayerID:  "a",
		Body:      []Position{{X: 2, Y: 2}},
		Direction: DirectionRight,
		Alive:     true,
	}
	state.Snakes["b"] = &Snake{
		PlayerID: "b",
		Body: []Position{
			{X: 4, Y: 2},
			{X: 3, Y: 2},
			{X: 3, Y: 3},
		},
		Direction: DirectionDown,
		Alive:     true,
	}

	engine.AdvanceTick(state)

	if state.Snakes["a"].Alive {
		t.Fatalf("snake a should be dead after colliding with snake b body")
	}
	if !state.Snakes["b"].Alive {
		t.Fatalf("snake b should remain alive")
	}
}

func TestFoodConsumptionGrowsSnakeIncrementsScoreAndRespawnsFood(t *testing.T) {
	engine := NewEngine(6)
	state := NewGameState(4, 4)
	state.Players["p1"] = &Player{ID: "p1", Name: "P1"}
	state.Snakes["p1"] = &Snake{
		PlayerID:  "p1",
		Body:      []Position{{X: 1, Y: 1}, {X: 0, Y: 1}},
		Direction: DirectionRight,
		Alive:     true,
	}
	state.Food = &Food{Position: Position{X: 2, Y: 1}}

	engine.AdvanceTick(state)

	snake := state.Snakes["p1"]
	if got, want := len(snake.Body), 3; got != want {
		t.Fatalf("snake length mismatch: got %d want %d", got, want)
	}
	if got, want := state.Players["p1"].Score, 1; got != want {
		t.Fatalf("score mismatch: got %d want %d", got, want)
	}
	if state.Food == nil {
		t.Fatalf("food should respawn after being eaten")
	}
	for _, segment := range snake.Body {
		if state.Food.Position == segment {
			t.Fatalf("respawned food should not overlap snake body")
		}
	}
}
