package game

import (
	"errors"
	"math/rand"
	"reflect"
	"testing"
)

func TestTickAdvancementStateTransitions(t *testing.T) {
	t.Parallel()

	type expectation struct {
		snake     []Point
		head      Point
		food      Point
		score     int
		tick      int
		gameOver  bool
		direction Direction
	}

	tests := []struct {
		name   string
		setup  func(t *testing.T) *Engine
		steps  []Direction
		expect expectation
		verify func(t *testing.T, engine *Engine, snap Snapshot)
	}{
		{
			name: "movement trims tail and applies queued input",
			setup: func(t *testing.T) *Engine {
				engine := mustEngine(t, 4, 4, 101)
				engine.snake = []Point{{X: 2, Y: 2}, {X: 2, Y: 3}}
				engine.occupied = map[Point]struct{}{
					{X: 2, Y: 2}: {},
					{X: 2, Y: 3}: {},
					{X: 0, Y: 0}: {},
				}
				engine.direction = DirectionUp
				engine.food = Point{X: 3, Y: 3}
				return engine
			},
			steps: []Direction{DirectionLeft, DirectionLeft},
			expect: expectation{
				snake: []Point{
					{X: 0, Y: 2},
					{X: 1, Y: 2},
				},
				head:      Point{X: 0, Y: 2},
				food:      Point{X: 3, Y: 3},
				score:     0,
				tick:      2,
				gameOver:  false,
				direction: DirectionLeft,
			},
			verify: func(t *testing.T, engine *Engine, _ Snapshot) {
				expectedOccupied := map[Point]struct{}{
					{X: 0, Y: 0}: {},
					{X: 0, Y: 2}: {},
					{X: 1, Y: 2}: {},
				}
				if !reflect.DeepEqual(engine.occupied, expectedOccupied) {
					t.Fatalf("expected occupied cells %+v, got %+v", expectedOccupied, engine.occupied)
				}
				if engine.hasInput {
					t.Fatalf("hasInput should clear after processing queued moves")
				}
			},
		},
		{
			name: "eating updates score then continues moving",
			setup: func(t *testing.T) *Engine {
				const seed = int64(2024)

				engine := mustEngine(t, 4, 4, seed)
				engine.food = Point{X: engine.snake[0].X + 1, Y: engine.snake[0].Y}
				engine.rng = rand.New(rand.NewSource(seed))

				return engine
			},
			steps: []Direction{DirectionRight, DirectionDown},
			expect: expectation{
				snake: []Point{
					{X: 3, Y: 3},
					{X: 3, Y: 2},
				},
				head:      Point{X: 3, Y: 3},
				food:      deterministicEmptyCell(4, 4, map[Point]struct{}{{X: 3, Y: 2}: {}, {X: 2, Y: 2}: {}}, 2024),
				score:     1,
				tick:      2,
				gameOver:  false,
				direction: DirectionDown,
			},
			verify: func(t *testing.T, engine *Engine, _ Snapshot) {
				expectedOccupied := map[Point]struct{}{
					{X: 3, Y: 2}: {},
					{X: 3, Y: 3}: {},
				}
				if !reflect.DeepEqual(engine.occupied, expectedOccupied) {
					t.Fatalf("expected occupied cells %+v, got %+v", expectedOccupied, engine.occupied)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			engine := tt.setup(t)

			for _, direction := range tt.steps {
				if ok := engine.QueueDirection(direction); !ok {
					t.Fatalf("failed to queue direction %v", direction)
				}
				engine.AdvanceTick()
			}

			snap := engine.Snapshot()

			if snap.GameOver != tt.expect.gameOver {
				t.Fatalf("game over mismatch: expected %v, got %v", tt.expect.gameOver, snap.GameOver)
			}
			if snap.Tick != tt.expect.tick {
				t.Fatalf("tick mismatch: expected %d, got %d", tt.expect.tick, snap.Tick)
			}
			if snap.Score != tt.expect.score {
				t.Fatalf("score mismatch: expected %d, got %d", tt.expect.score, snap.Score)
			}
			if snap.Direction != tt.expect.direction {
				t.Fatalf("direction mismatch: expected %v, got %v", tt.expect.direction, snap.Direction)
			}
			if len(snap.Snake) != len(tt.expect.snake) {
				t.Fatalf("snake length mismatch: expected %d, got %d", len(tt.expect.snake), len(snap.Snake))
			}
			if !reflect.DeepEqual(snap.Snake, tt.expect.snake) {
				t.Fatalf("snake body mismatch: expected %+v, got %+v", tt.expect.snake, snap.Snake)
			}
			if snap.Snake[0] != tt.expect.head {
				t.Fatalf("head mismatch: expected %+v, got %+v", tt.expect.head, snap.Snake[0])
			}
			if snap.Food != tt.expect.food {
				t.Fatalf("food mismatch: expected %+v, got %+v", tt.expect.food, snap.Food)
			}

			if tt.verify != nil {
				tt.verify(t, engine, snap)
			}
		})
	}
}

func TestAdvanceTickStateTransitionsBoundarySequence(t *testing.T) {
	const (
		width  = 3
		height = 2
		seed   = int64(4242)
	)

	engine := mustEngine(t, width, height, seed)

	start := engine.Snapshot()
	engine.food = Point{X: start.Snake[0].X + 1, Y: start.Snake[0].Y}
	engine.rng = rand.New(rand.NewSource(seed))

	if ok := engine.QueueDirection(DirectionRight); !ok {
		t.Fatalf("expected to queue initial move toward food")
	}
	engine.AdvanceTick()
	afterGrowth := engine.Snapshot()

	occupiedAfterGrowth := map[Point]struct{}{
		afterGrowth.Snake[0]: {},
		afterGrowth.Snake[1]: {},
	}
	expectedFood := deterministicEmptyCell(width, height, occupiedAfterGrowth, seed)

	if afterGrowth.Tick != 1 {
		t.Fatalf("tick mismatch after growth: expected 1, got %d", afterGrowth.Tick)
	}
	if afterGrowth.Score != 1 {
		t.Fatalf("expected score to increment after eating, got %d", afterGrowth.Score)
	}
	if len(afterGrowth.Snake) != 2 {
		t.Fatalf("expected snake to grow to length 2, got %d", len(afterGrowth.Snake))
	}
	if afterGrowth.Snake[0] != (Point{X: start.Snake[0].X + 1, Y: start.Snake[0].Y}) {
		t.Fatalf("head mismatch after first move, got %+v", afterGrowth.Snake[0])
	}
	if afterGrowth.Food != expectedFood {
		t.Fatalf("expected deterministic respawn at %+v, got %+v", expectedFood, afterGrowth.Food)
	}
	if afterGrowth.Direction != DirectionRight {
		t.Fatalf("direction mismatch after growth: expected %v, got %v", DirectionRight, afterGrowth.Direction)
	}
	if engine.hasInput {
		t.Fatalf("queued input should clear after processing growth")
	}

	if ok := engine.QueueDirection(DirectionUp); !ok {
		t.Fatalf("expected to queue turn upward")
	}
	engine.AdvanceTick()
	afterTurn := engine.Snapshot()

	expectedHeadAfterTurn := Point{X: afterGrowth.Snake[0].X, Y: afterGrowth.Snake[0].Y - 1}
	if afterTurn.GameOver {
		t.Fatalf("did not expect game over while inside bounds")
	}
	if afterTurn.Tick != 2 {
		t.Fatalf("tick mismatch after second move: expected 2, got %d", afterTurn.Tick)
	}
	if afterTurn.Score != 1 {
		t.Fatalf("score should remain after moving without food, got %d", afterTurn.Score)
	}
	if afterTurn.Snake[0] != expectedHeadAfterTurn {
		t.Fatalf("head mismatch after second move: expected %+v, got %+v", expectedHeadAfterTurn, afterTurn.Snake[0])
	}
	if afterTurn.Food != expectedFood {
		t.Fatalf("food should remain until eaten, expected %+v got %+v", expectedFood, afterTurn.Food)
	}
	if afterTurn.Direction != DirectionUp {
		t.Fatalf("direction mismatch after turning up: expected %v, got %v", DirectionUp, afterTurn.Direction)
	}

	if ok := engine.QueueDirection(DirectionUp); !ok {
		t.Fatalf("expected to queue final move into boundary")
	}
	engine.AdvanceTick()
	afterCollision := engine.Snapshot()

	if !afterCollision.GameOver {
		t.Fatalf("expected boundary collision to end the game")
	}
	if afterCollision.Tick != afterTurn.Tick {
		t.Fatalf("tick should not advance after boundary collision, expected %d got %d", afterTurn.Tick, afterCollision.Tick)
	}
	if afterCollision.Score != afterTurn.Score {
		t.Fatalf("score should remain unchanged after collision, expected %d got %d", afterTurn.Score, afterCollision.Score)
	}
	if afterCollision.Direction != DirectionUp {
		t.Fatalf("direction should remain last processed input, expected %v got %v", DirectionUp, afterCollision.Direction)
	}
	if !reflect.DeepEqual(afterCollision.Snake, afterTurn.Snake) {
		t.Fatalf("snake should remain unchanged after collision, before %+v after %+v", afterTurn.Snake, afterCollision.Snake)
	}
	if engine.hasInput {
		t.Fatalf("queued input should clear even when collision occurs")
	}
	if engine.QueueDirection(DirectionLeft) {
		t.Fatalf("should reject input after game over")
	}
}

func TestEdgeCaseConstraintsAdditional(t *testing.T) {
	t.Parallel()

	t.Run("invalid dimensions and full grid are rejected", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name   string
			width  int
			height int
			err    error
		}{
			{name: "zero width", width: 0, height: 2, err: ErrInvalidDimensions},
			{name: "zero height", width: 2, height: 0, err: ErrInvalidDimensions},
			{name: "no free cells", width: 1, height: 1, err: ErrNoFreeCells},
		}

		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				engine, err := NewEngine(tt.width, tt.height, 5)
				if !errors.Is(err, tt.err) {
					t.Fatalf("expected error %v, got %v", tt.err, err)
				}
				if engine != nil {
					t.Fatalf("expected engine to be nil when initialization fails")
				}
			})
		}
	})

	t.Run("single column boundary collision halts progression", func(t *testing.T) {
		engine := mustEngine(t, 1, 2, 71)
		engine.food = Point{X: 0, Y: 0}

		engine.AdvanceTick()
		snap := engine.Snapshot()

		if !snap.GameOver {
			t.Fatalf("expected collision with right boundary to end game")
		}
		if snap.Tick != 0 {
			t.Fatalf("tick should not increment after immediate boundary collision, got %d", snap.Tick)
		}
		if snap.Score != 0 {
			t.Fatalf("score should remain unchanged on boundary collision, got %d", snap.Score)
		}
		if snap.Snake[0] != (Point{X: 0, Y: 1}) {
			t.Fatalf("expected head to remain at start, got %+v", snap.Snake[0])
		}
		if snap.Food != (Point{X: 0, Y: 0}) {
			t.Fatalf("food should remain unchanged after collision, got %+v", snap.Food)
		}
		if snap.Direction != DirectionRight {
			t.Fatalf("direction should remain unchanged after collision, got %v", snap.Direction)
		}
	})

	t.Run("multiple players occupancy preserved when respawning food", func(t *testing.T) {
		const seed = int64(999)

		engine := mustEngine(t, 3, 3, seed)
		engine.snake = []Point{{X: 0, Y: 1}}
		engine.occupied = map[Point]struct{}{
			{X: 0, Y: 1}: {},
			{X: 2, Y: 2}: {},
			{X: 1, Y: 0}: {},
		}
		engine.direction = DirectionRight
		engine.food = Point{X: 1, Y: 1}
		engine.rng = rand.New(rand.NewSource(seed))

		if ok := engine.QueueDirection(DirectionRight); !ok {
			t.Fatalf("expected to queue movement toward food")
		}

		engine.AdvanceTick()
		snap := engine.Snapshot()

		occupiedAfterGrowth := map[Point]struct{}{
			{X: 1, Y: 1}: {},
			{X: 0, Y: 1}: {},
			{X: 2, Y: 2}: {},
			{X: 1, Y: 0}: {},
		}
		expectedFood := deterministicEmptyCell(engine.width, engine.height, occupiedAfterGrowth, seed)

		if snap.GameOver {
			t.Fatalf("did not expect game over when growing near other players")
		}
		if snap.Tick != 1 {
			t.Fatalf("tick mismatch: expected 1, got %d", snap.Tick)
		}
		if snap.Score != 1 {
			t.Fatalf("score mismatch: expected 1 after eating, got %d", snap.Score)
		}
		if len(snap.Snake) != 2 {
			t.Fatalf("snake length mismatch: expected 2 after growth, got %d", len(snap.Snake))
		}
		if snap.Snake[0] != (Point{X: 1, Y: 1}) {
			t.Fatalf("head mismatch: expected %+v, got %+v", Point{X: 1, Y: 1}, snap.Snake[0])
		}
		if snap.Food != expectedFood {
			t.Fatalf("food respawn mismatch: expected %+v, got %+v", expectedFood, snap.Food)
		}
		if len(engine.occupied) != len(occupiedAfterGrowth) {
			t.Fatalf("occupied count mismatch: expected %d, got %d", len(occupiedAfterGrowth), len(engine.occupied))
		}
		for _, point := range []Point{{X: 2, Y: 2}, {X: 1, Y: 0}} {
			if _, ok := engine.occupied[point]; !ok {
				t.Fatalf("expected external occupancy at %+v to persist", point)
			}
		}
	})
}

func TestEdgeCaseConstraintsRespawnAndBoundaries(t *testing.T) {
	t.Parallel()

	t.Run("rejects invalid dimensions", func(t *testing.T) {
		t.Parallel()

		engine, err := NewEngine(0, 3, 9)
		if !errors.Is(err, ErrInvalidDimensions) {
			t.Fatalf("expected ErrInvalidDimensions, got %v", err)
		}
		if engine != nil {
			t.Fatalf("expected engine to be nil when initialization fails")
		}
	})

	t.Run("single column boundary collision halts immediately", func(t *testing.T) {
		engine := mustEngine(t, 1, 2, 303)
		engine.food = Point{X: 0, Y: 0}

		engine.AdvanceTick()
		snap := engine.Snapshot()

		if !snap.GameOver {
			t.Fatalf("expected collision on first move to end game")
		}
		if snap.Tick != 0 {
			t.Fatalf("tick should remain zero on immediate collision, got %d", snap.Tick)
		}
		if snap.Score != 0 {
			t.Fatalf("score should remain zero after boundary collision, got %d", snap.Score)
		}
		if len(snap.Snake) != 1 {
			t.Fatalf("snake length should remain one, got %d", len(snap.Snake))
		}
		if snap.Snake[0] != (Point{X: 0, Y: 1}) {
			t.Fatalf("head should stay at starting cell, got %+v", snap.Snake[0])
		}
		if snap.Direction != DirectionRight {
			t.Fatalf("direction should remain initial right, got %v", snap.Direction)
		}
		if snap.Food != (Point{X: 0, Y: 0}) {
			t.Fatalf("food should remain unchanged after collision, got %+v", snap.Food)
		}
	})

	t.Run("food respawn respects other players", func(t *testing.T) {
		const seed = int64(881)

		engine := mustEngine(t, 3, 3, seed)
		engine.snake = []Point{{X: 1, Y: 1}}
		engine.occupied = map[Point]struct{}{
			{X: 1, Y: 1}: {},
			{X: 0, Y: 0}: {},
			{X: 2, Y: 2}: {},
		}
		engine.direction = DirectionRight
		engine.food = Point{X: 2, Y: 1}
		engine.rng = rand.New(rand.NewSource(seed))

		if ok := engine.QueueDirection(DirectionRight); !ok {
			t.Fatalf("expected to queue move toward food")
		}

		engine.AdvanceTick()
		snap := engine.Snapshot()

		if snap.GameOver {
			t.Fatalf("did not expect collision when growing near other snakes")
		}
		if snap.Tick != 1 {
			t.Fatalf("tick mismatch: expected 1, got %d", snap.Tick)
		}
		if snap.Score != 1 {
			t.Fatalf("score should increment after eating, got %d", snap.Score)
		}
		if len(snap.Snake) != 2 {
			t.Fatalf("expected snake to grow to length 2, got %d", len(snap.Snake))
		}
		if snap.Snake[0] != (Point{X: 2, Y: 1}) {
			t.Fatalf("head mismatch after eating, got %+v", snap.Snake[0])
		}

		occupiedAfterGrowth := map[Point]struct{}{
			{X: 2, Y: 1}: {},
			{X: 1, Y: 1}: {},
			{X: 0, Y: 0}: {},
			{X: 2, Y: 2}: {},
		}
		expectedFood := deterministicEmptyCell(engine.width, engine.height, occupiedAfterGrowth, seed)

		if snap.Food != expectedFood {
			t.Fatalf("food respawn mismatch: expected %+v, got %+v", expectedFood, snap.Food)
		}
		for _, point := range []Point{{X: 0, Y: 0}, {X: 2, Y: 2}} {
			if _, ok := engine.occupied[point]; !ok {
				t.Fatalf("expected external occupancy at %+v to persist", point)
			}
		}
	})
}
