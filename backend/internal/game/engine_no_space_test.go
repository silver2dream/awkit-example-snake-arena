package game

import (
	"reflect"
	"testing"
)

func TestAdvanceTickNoSpaceClearsQueuedInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		setup            func(t *testing.T) *Engine
		queuedDirection  Direction
		expectedHead     Point
		expectedSnake    []Point
		expectedOccupied map[Point]struct{}
	}{
		{
			name: "snake fills final cell and ends game",
			setup: func(t *testing.T) *Engine {
				engine := mustEngine(t, 2, 2, 101)
				engine.snake = []Point{{X: 0, Y: 1}, {X: 0, Y: 0}, {X: 1, Y: 0}}
				engine.occupied = map[Point]struct{}{
					{X: 0, Y: 1}: {},
					{X: 0, Y: 0}: {},
					{X: 1, Y: 0}: {},
				}
				engine.direction = DirectionRight
				engine.food = Point{X: 1, Y: 1}
				return engine
			},
			queuedDirection: DirectionRight,
			expectedHead:    Point{X: 1, Y: 1},
			expectedSnake: []Point{
				{X: 1, Y: 1},
				{X: 0, Y: 1},
				{X: 0, Y: 0},
				{X: 1, Y: 0},
			},
			expectedOccupied: map[Point]struct{}{
				{X: 0, Y: 1}: {},
				{X: 0, Y: 0}: {},
				{X: 1, Y: 0}: {},
				{X: 1, Y: 1}: {},
			},
		},
		{
			name: "other snakes occupy remaining cells",
			setup: func(t *testing.T) *Engine {
				engine := mustEngine(t, 2, 2, 2024)
				engine.snake = []Point{{X: 0, Y: 0}}
				engine.occupied = map[Point]struct{}{
					{X: 0, Y: 0}: {},
					{X: 1, Y: 0}: {}, // other player
					{X: 1, Y: 1}: {}, // other player
				}
				engine.direction = DirectionDown
				engine.food = Point{X: 0, Y: 1}
				return engine
			},
			queuedDirection: DirectionDown,
			expectedHead:    Point{X: 0, Y: 1},
			expectedSnake: []Point{
				{X: 0, Y: 1},
				{X: 0, Y: 0},
			},
			expectedOccupied: map[Point]struct{}{
				{X: 0, Y: 1}: {},
				{X: 0, Y: 0}: {},
				{X: 1, Y: 0}: {},
				{X: 1, Y: 1}: {},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			engine := tt.setup(t)

			if ok := engine.QueueDirection(tt.queuedDirection); !ok {
				t.Fatalf("expected to queue direction %v", tt.queuedDirection)
			}

			engine.AdvanceTick()
			snap := engine.Snapshot()

			if !snap.GameOver {
				t.Fatalf("expected game over when no free cells remain for respawn")
			}
			if snap.Tick != 1 {
				t.Fatalf("tick should advance once before detecting no space, got %d", snap.Tick)
			}
			if snap.Score != 0 {
				t.Fatalf("score should remain unchanged when food cannot respawn, got %d", snap.Score)
			}
			if snap.Direction != tt.queuedDirection {
				t.Fatalf("direction mismatch: expected %v, got %v", tt.queuedDirection, snap.Direction)
			}
			if engine.hasInput {
				t.Fatalf("queued input flag should clear after processing movement")
			}
			if len(snap.Snake) != len(tt.expectedSnake) {
				t.Fatalf("snake length mismatch: expected %d, got %d", len(tt.expectedSnake), len(snap.Snake))
			}
			if snap.Snake[0] != tt.expectedHead {
				t.Fatalf("head mismatch: expected %+v, got %+v", tt.expectedHead, snap.Snake[0])
			}
			if !reflect.DeepEqual(snap.Snake, tt.expectedSnake) {
				t.Fatalf("snake body mismatch: expected %+v, got %+v", tt.expectedSnake, snap.Snake)
			}
			if len(engine.occupied) != len(tt.expectedOccupied) {
				t.Fatalf("occupied cell count mismatch: expected %d, got %d", len(tt.expectedOccupied), len(engine.occupied))
			}
			for point := range tt.expectedOccupied {
				if _, ok := engine.occupied[point]; !ok {
					t.Fatalf("expected occupancy at %+v to persist after no-space game over", point)
				}
			}
		})
	}
}
