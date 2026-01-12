package game

import (
	"reflect"
	"testing"
)

func TestNewEngineDeterministicInitialization(t *testing.T) {
	const (
		width  = 7
		height = 6
		seed   = int64(404)
	)

	start := Point{X: width / 2, Y: height / 2}
	expectedFood := deterministicEmptyCell(width, height, map[Point]struct{}{start: {}}, seed)

	first, err := NewEngine(width, height, seed)
	if err != nil {
		t.Fatalf("unexpected error creating first engine: %v", err)
	}
	second, err := NewEngine(width, height, seed)
	if err != nil {
		t.Fatalf("unexpected error creating second engine: %v", err)
	}

	firstSnap := first.Snapshot()
	secondSnap := second.Snapshot()

	if !reflect.DeepEqual(firstSnap, secondSnap) {
		t.Fatalf("expected deterministic initial state with identical seeds, got %+v and %+v", firstSnap, secondSnap)
	}
	if firstSnap.Snake[0] != start {
		t.Fatalf("expected snake to start at %+v, got %+v", start, firstSnap.Snake[0])
	}
	if firstSnap.Food != expectedFood {
		t.Fatalf("expected deterministic food spawn at %+v, got %+v", expectedFood, firstSnap.Food)
	}
	if firstSnap.Tick != 0 || firstSnap.Score != 0 || firstSnap.GameOver {
		t.Fatalf("expected fresh game state, got tick=%d score=%d gameOver=%v", firstSnap.Tick, firstSnap.Score, firstSnap.GameOver)
	}
}

func TestAdvanceTickCollisionsPreserveOccupancy(t *testing.T) {
	t.Parallel()

	clone := func(src map[Point]struct{}) map[Point]struct{} {
		dst := make(map[Point]struct{}, len(src))
		for point := range src {
			dst[point] = struct{}{}
		}
		return dst
	}

	tests := []struct {
		name   string
		setup  func(t *testing.T) *Engine
		queued Direction
	}{
		{
			name: "wall collision leaves occupancy intact",
			setup: func(t *testing.T) *Engine {
				engine := mustEngine(t, 2, 2, 1)
				engine.food = Point{X: 0, Y: 0}
				return engine
			},
			queued: DirectionRight,
		},
		{
			name: "self collision keeps body unchanged",
			setup: func(t *testing.T) *Engine {
				engine := mustEngine(t, 3, 3, 2)
				engine.snake = []Point{{X: 1, Y: 1}, {X: 1, Y: 2}, {X: 1, Y: 0}}
				engine.occupied = map[Point]struct{}{
					{X: 1, Y: 1}: {},
					{X: 1, Y: 2}: {},
					{X: 1, Y: 0}: {},
				}
				engine.direction = DirectionUp
				engine.food = Point{X: 2, Y: 2}
				return engine
			},
			queued: DirectionUp,
		},
		{
			name: "collision with other snake retains external segments",
			setup: func(t *testing.T) *Engine {
				engine := mustEngine(t, 3, 3, 3)
				engine.snake = []Point{{X: 1, Y: 1}}
				engine.occupied = map[Point]struct{}{
					{X: 1, Y: 1}: {},
					{X: 1, Y: 0}: {}, // other player segment
				}
				engine.direction = DirectionUp
				engine.food = Point{X: 2, Y: 2}
				return engine
			},
			queued: DirectionUp,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			engine := tt.setup(t)
			beforeSnapshot := engine.Snapshot()
			beforeOccupied := clone(engine.occupied)

			if ok := engine.QueueDirection(tt.queued); !ok {
				t.Fatalf("expected to queue direction %v", tt.queued)
			}

			engine.AdvanceTick()
			after := engine.Snapshot()

			if !after.GameOver {
				t.Fatalf("expected collision to end the game")
			}
			if after.Tick != beforeSnapshot.Tick {
				t.Fatalf("expected tick to remain %d after collision, got %d", beforeSnapshot.Tick, after.Tick)
			}
			if after.Score != beforeSnapshot.Score {
				t.Fatalf("score should not change on collision, expected %d got %d", beforeSnapshot.Score, after.Score)
			}
			if !reflect.DeepEqual(after.Snake, beforeSnapshot.Snake) {
				t.Fatalf("expected snake body to remain unchanged after collision, before %+v after %+v", beforeSnapshot.Snake, after.Snake)
			}
			if !reflect.DeepEqual(engine.occupied, beforeOccupied) {
				t.Fatalf("expected occupancy map to remain unchanged, before %+v after %+v", beforeOccupied, engine.occupied)
			}
			if engine.hasInput {
				t.Fatalf("expected queued input to clear after collision")
			}
			if after.Direction != tt.queued {
				t.Fatalf("expected direction to reflect queued input %v, got %v", tt.queued, after.Direction)
			}
		})
	}
}
