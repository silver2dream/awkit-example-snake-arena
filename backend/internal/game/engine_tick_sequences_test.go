package game

import (
	"math/rand"
	"testing"
)

func TestAdvanceTickSequentialScenarios(t *testing.T) {
	t.Parallel()

	type expectation struct {
		head      Point
		length    int
		score     int
		tick      int
		gameOver  bool
		food      Point
		direction Direction
	}

	tests := []struct {
		name              string
		setup             func(t *testing.T) *Engine
		first             Direction
		second            *Direction
		expectAfterFirst  expectation
		expectAfterSecond expectation
		verify            func(t *testing.T, engine *Engine)
	}{
		{
			name: "single row growth then wall collision",
			setup: func(t *testing.T) *Engine {
				engine := mustEngine(t, 3, 1, 909)
				engine.snake = []Point{{X: 1, Y: 0}}
				engine.occupied = map[Point]struct{}{
					{X: 1, Y: 0}: {},
				}
				engine.direction = DirectionRight
				engine.food = Point{X: 0, Y: 0}
				engine.rng = rand.New(rand.NewSource(909))
				return engine
			},
			first:  DirectionLeft,
			second: ptr(DirectionLeft),
			expectAfterFirst: expectation{
				head:   Point{X: 0, Y: 0},
				length: 2,
				score:  1,
				tick:   1,
				food: deterministicEmptyCell(3, 1, map[Point]struct{}{
					{X: 0, Y: 0}: {},
					{X: 1, Y: 0}: {},
				}, 909),
				direction: DirectionLeft,
			},
			expectAfterSecond: expectation{
				head:   Point{X: 0, Y: 0},
				length: 2,
				score:  1,
				tick:   1,
				food: deterministicEmptyCell(3, 1, map[Point]struct{}{
					{X: 0, Y: 0}: {},
					{X: 1, Y: 0}: {},
				}, 909),
				gameOver:  true,
				direction: DirectionLeft,
			},
			verify: func(t *testing.T, engine *Engine) {
				if engine.hasInput {
					t.Fatalf("queued input flag should clear after wall collision")
				}
			},
		},
		{
			name: "growth followed by collision with other snake",
			setup: func(t *testing.T) *Engine {
				engine := mustEngine(t, 4, 3, 515)
				engine.snake = []Point{{X: 1, Y: 1}}
				engine.occupied = map[Point]struct{}{
					{X: 1, Y: 1}: {},
					{X: 3, Y: 1}: {}, // other player segment
				}
				engine.direction = DirectionRight
				engine.food = Point{X: 2, Y: 1}
				engine.rng = rand.New(rand.NewSource(515))
				return engine
			},
			first:  DirectionRight,
			second: ptr(DirectionRight),
			expectAfterFirst: expectation{
				head:   Point{X: 2, Y: 1},
				length: 2,
				score:  1,
				tick:   1,
				food: deterministicEmptyCell(4, 3, map[Point]struct{}{
					{X: 2, Y: 1}: {},
					{X: 1, Y: 1}: {},
					{X: 3, Y: 1}: {},
				}, 515),
				direction: DirectionRight,
			},
			expectAfterSecond: expectation{
				head:   Point{X: 2, Y: 1},
				length: 2,
				score:  1,
				tick:   1,
				food: deterministicEmptyCell(4, 3, map[Point]struct{}{
					{X: 2, Y: 1}: {},
					{X: 1, Y: 1}: {},
					{X: 3, Y: 1}: {},
				}, 515),
				gameOver:  true,
				direction: DirectionRight,
			},
			verify: func(t *testing.T, engine *Engine) {
				if _, ok := engine.occupied[Point{X: 3, Y: 1}]; !ok {
					t.Fatalf("expected other snake occupancy to persist after collision")
				}
				if engine.hasInput {
					t.Fatalf("queued input should clear after collision with other snake")
				}
			},
		},
	}

	assertSnapshot := func(t *testing.T, snap Snapshot, expect expectation) {
		t.Helper()

		if snap.GameOver != expect.gameOver {
			t.Fatalf("game over mismatch: expected %v, got %v", expect.gameOver, snap.GameOver)
		}
		if snap.Tick != expect.tick {
			t.Fatalf("tick mismatch: expected %d, got %d", expect.tick, snap.Tick)
		}
		if snap.Score != expect.score {
			t.Fatalf("score mismatch: expected %d, got %d", expect.score, snap.Score)
		}
		if len(snap.Snake) != expect.length {
			t.Fatalf("snake length mismatch: expected %d, got %d", expect.length, len(snap.Snake))
		}
		if snap.Snake[0] != expect.head {
			t.Fatalf("head position mismatch: expected %+v, got %+v", expect.head, snap.Snake[0])
		}
		if snap.Food != expect.food {
			t.Fatalf("food position mismatch: expected %+v, got %+v", expect.food, snap.Food)
		}
		if snap.Direction != expect.direction {
			t.Fatalf("direction mismatch: expected %v, got %v", expect.direction, snap.Direction)
		}
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			engine := tt.setup(t)

			if ok := engine.QueueDirection(tt.first); !ok {
				t.Fatalf("failed to queue first direction %v", tt.first)
			}

			engine.AdvanceTick()
			assertSnapshot(t, engine.Snapshot(), tt.expectAfterFirst)

			if tt.second != nil {
				if ok := engine.QueueDirection(*tt.second); !ok {
					t.Fatalf("failed to queue second direction %v", *tt.second)
				}
			}

			engine.AdvanceTick()
			assertSnapshot(t, engine.Snapshot(), tt.expectAfterSecond)

			if tt.verify != nil {
				tt.verify(t, engine)
			}
		})
	}
}
