package game

import (
	"math/rand"
	"reflect"
	"testing"
)

func TestEngineInitializationAndGrowthDeterministic(t *testing.T) {
	t.Parallel()

	const (
		width  = 6
		height = 5
		seed   = int64(2025)
	)

	start := Point{X: width / 2, Y: height / 2}
	initialOccupied := map[Point]struct{}{
		start: {},
	}
	expectedInitialFood := deterministicEmptyCell(width, height, initialOccupied, seed)

	first := mustEngine(t, width, height, seed)
	second := mustEngine(t, width, height, seed)

	initialSnapA := first.Snapshot()
	initialSnapB := second.Snapshot()

	if initialSnapA.Food != expectedInitialFood {
		t.Fatalf("expected deterministic initial food at %+v, got %+v", expectedInitialFood, initialSnapA.Food)
	}
	if !reflect.DeepEqual(initialSnapA, initialSnapB) {
		t.Fatalf("expected identical initial snapshots for same seed:\nfirst: %+v\nsecond: %+v", initialSnapA, initialSnapB)
	}

	targetFood := Point{X: start.X + 1, Y: start.Y}
	first.food = targetFood
	second.food = targetFood

	first.rng = rand.New(rand.NewSource(seed + 1))
	second.rng = rand.New(rand.NewSource(seed + 1))

	occupiedAfterGrowth := map[Point]struct{}{
		targetFood: {},
		start:      {},
	}
	expectedRespawn := deterministicEmptyCell(width, height, occupiedAfterGrowth, seed+1)

	first.AdvanceTick()
	second.AdvanceTick()

	afterA := first.Snapshot()
	afterB := second.Snapshot()

	if !reflect.DeepEqual(afterA, afterB) {
		t.Fatalf("expected deterministic snapshots after identical inputs:\nfirst: %+v\nsecond: %+v", afterA, afterB)
	}
	if afterA.Tick != 1 || afterA.Score != 1 {
		t.Fatalf("expected tick 1 and score 1 after eating, got tick %d score %d", afterA.Tick, afterA.Score)
	}
	if afterA.GameOver {
		t.Fatalf("did not expect game over after deterministic growth")
	}
	if len(afterA.Snake) != 2 {
		t.Fatalf("expected snake to grow to length 2, got %d", len(afterA.Snake))
	}
	if afterA.Snake[0] != targetFood {
		t.Fatalf("expected head to move onto food at %+v, got %+v", targetFood, afterA.Snake[0])
	}
	if afterA.Food != expectedRespawn {
		t.Fatalf("expected respawned food at %+v, got %+v", expectedRespawn, afterA.Food)
	}
	if afterA.Direction != DirectionRight {
		t.Fatalf("expected direction to remain right after growth, got %v", afterA.Direction)
	}
	if len(first.occupied) != len(occupiedAfterGrowth) {
		t.Fatalf("expected %d occupied cells after growth, got %d", len(occupiedAfterGrowth), len(first.occupied))
	}
	for point := range occupiedAfterGrowth {
		if _, ok := first.occupied[point]; !ok {
			t.Fatalf("expected occupancy at %+v to persist after growth", point)
		}
	}
}

func TestAdvanceTickCollisionStopsProgress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		setup             func(t *testing.T) *Engine
		queue             *Direction
		expectedHead      Point
		expectedLength    int
		expectedTick      int
		expectedDirection Direction
		verify            func(t *testing.T, engine *Engine)
	}{
		{
			name: "wall collision halts subsequent ticks",
			setup: func(t *testing.T) *Engine {
				engine := mustEngine(t, 2, 2, 41)
				engine.snake = []Point{{X: 0, Y: 0}}
				engine.occupied = map[Point]struct{}{
					{X: 0, Y: 0}: {},
				}
				engine.direction = DirectionRight
				engine.food = Point{X: 0, Y: 0}
				return engine
			},
			queue:             ptr(DirectionLeft),
			expectedHead:      Point{X: 0, Y: 0},
			expectedLength:    1,
			expectedTick:      0,
			expectedDirection: DirectionLeft,
		},
		{
			name: "self collision preserves body and direction",
			setup: func(t *testing.T) *Engine {
				engine := mustEngine(t, 4, 4, 73)
				engine.snake = []Point{{X: 1, Y: 1}, {X: 1, Y: 2}, {X: 1, Y: 3}}
				engine.occupied = map[Point]struct{}{
					{X: 1, Y: 1}: {},
					{X: 1, Y: 2}: {},
					{X: 1, Y: 3}: {},
				}
				engine.direction = DirectionUp
				engine.food = Point{X: 0, Y: 0}
				return engine
			},
			queue:             ptr(DirectionDown),
			expectedHead:      Point{X: 1, Y: 1},
			expectedLength:    3,
			expectedTick:      0,
			expectedDirection: DirectionDown,
			verify: func(t *testing.T, engine *Engine) {
				if len(engine.occupied) != 3 {
					t.Fatalf("expected body occupancy to remain intact after collision, got %d cells", len(engine.occupied))
				}
			},
		},
		{
			name: "collision with other snake retains external occupancy",
			setup: func(t *testing.T) *Engine {
				engine := mustEngine(t, 3, 3, 11)
				engine.snake = []Point{{X: 1, Y: 1}}
				engine.occupied = map[Point]struct{}{
					{X: 1, Y: 1}: {},
					{X: 2, Y: 1}: {}, // other player
				}
				engine.direction = DirectionRight
				engine.food = Point{X: 0, Y: 0}
				return engine
			},
			queue:             ptr(DirectionRight),
			expectedHead:      Point{X: 1, Y: 1},
			expectedLength:    1,
			expectedTick:      0,
			expectedDirection: DirectionRight,
			verify: func(t *testing.T, engine *Engine) {
				if _, ok := engine.occupied[Point{X: 2, Y: 1}]; !ok {
					t.Fatalf("expected other snake occupancy to persist after collision")
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			engine := tt.setup(t)
			before := engine.Snapshot()

			if tt.queue != nil {
				if ok := engine.QueueDirection(*tt.queue); !ok {
					t.Fatalf("expected to queue direction %v", *tt.queue)
				}
			}

			engine.AdvanceTick()
			afterCollision := engine.Snapshot()

			engine.AdvanceTick()
			afterNoOp := engine.Snapshot()

			if !afterCollision.GameOver {
				t.Fatalf("expected collision to end the game")
			}
			if afterCollision.Tick != tt.expectedTick {
				t.Fatalf("tick mismatch after collision: expected %d, got %d", tt.expectedTick, afterCollision.Tick)
			}
			if len(afterCollision.Snake) != tt.expectedLength {
				t.Fatalf("length mismatch: expected %d, got %d", tt.expectedLength, len(afterCollision.Snake))
			}
			if afterCollision.Snake[0] != tt.expectedHead {
				t.Fatalf("head mismatch: expected %+v, got %+v", tt.expectedHead, afterCollision.Snake[0])
			}
			if afterCollision.Score != 0 {
				t.Fatalf("expected score to remain zero on collision, got %d", afterCollision.Score)
			}
			if afterCollision.Direction != tt.expectedDirection {
				t.Fatalf("direction mismatch: expected %v, got %v", tt.expectedDirection, afterCollision.Direction)
			}
			if !reflect.DeepEqual(afterCollision, afterNoOp) {
				t.Fatalf("state should remain frozen after game over, before %+v after %+v", afterCollision, afterNoOp)
			}
			if engine.hasInput {
				t.Fatalf("queued input should clear after processing collision")
			}
			if before.GameOver {
				t.Fatalf("expected collision to be detected during first advancement, initial state was already game over")
			}

			if tt.verify != nil {
				tt.verify(t, engine)
			}
		})
	}
}
