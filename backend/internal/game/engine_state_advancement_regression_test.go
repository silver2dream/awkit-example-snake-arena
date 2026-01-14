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

func TestDeterministicSelfCollisionAcrossRuns(t *testing.T) {
	t.Parallel()

	build := func(t *testing.T) *Engine {
		t.Helper()

		engine := mustEngine(t, 4, 4, 888)
		engine.snake = []Point{{X: 2, Y: 2}, {X: 1, Y: 2}, {X: 1, Y: 1}}
		engine.occupied = map[Point]struct{}{
			{X: 2, Y: 2}: {},
			{X: 1, Y: 2}: {},
			{X: 1, Y: 1}: {},
		}
		engine.direction = DirectionRight
		engine.food = Point{X: 3, Y: 3}

		return engine
	}

	expected := Snapshot{
		Width:     4,
		Height:    4,
		Snake:     []Point{{X: 2, Y: 2}, {X: 1, Y: 2}, {X: 1, Y: 1}},
		Direction: DirectionLeft,
		Food:      Point{X: 3, Y: 3},
		Tick:      0,
		GameOver:  true,
		Score:     0,
	}

	first := build(t)
	second := build(t)

	for _, engine := range []*Engine{first, second} {
		if ok := engine.QueueDirection(DirectionLeft); !ok {
			t.Fatalf("expected to queue reversal to trigger self collision")
		}
		engine.AdvanceTick()
	}

	snapA := first.Snapshot()
	snapB := second.Snapshot()

	if !reflect.DeepEqual(snapA, expected) {
		t.Fatalf("unexpected snapshot after self collision: got %+v, want %+v", snapA, expected)
	}
	if !reflect.DeepEqual(snapA, snapB) {
		t.Fatalf("expected deterministic self-collision snapshots, got %+v and %+v", snapA, snapB)
	}

	for _, engine := range []*Engine{first, second} {
		if engine.hasInput {
			t.Fatalf("queued input flag should clear after processing collision")
		}
		if len(engine.occupied) != len(expected.Snake) {
			t.Fatalf("expected %d occupied cells to remain, got %d", len(expected.Snake), len(engine.occupied))
		}
		for _, point := range expected.Snake {
			if _, ok := engine.occupied[point]; !ok {
				t.Fatalf("expected occupancy at %+v to persist after collision", point)
			}
		}
	}
}

func TestDeterministicNoSpaceGameOverWithExternalOccupancy(t *testing.T) {
	t.Parallel()

	const (
		width  = 3
		height = 2
		seed   = int64(5150)
	)

	build := func(t *testing.T) *Engine {
		t.Helper()

		engine := mustEngine(t, width, height, seed)
		engine.snake = []Point{{X: 0, Y: 1}, {X: 0, Y: 0}}
		engine.occupied = map[Point]struct{}{
			{X: 0, Y: 1}: {},
			{X: 0, Y: 0}: {},
			{X: 1, Y: 0}: {}, // other snake
			{X: 2, Y: 0}: {}, // other snake
			{X: 2, Y: 1}: {}, // other snake
		}
		engine.direction = DirectionRight
		engine.food = Point{X: 1, Y: 1}
		engine.rng = rand.New(rand.NewSource(seed))

		return engine
	}

	expectedSnake := []Point{
		{X: 1, Y: 1},
		{X: 0, Y: 1},
		{X: 0, Y: 0},
	}
	expectedOccupied := map[Point]struct{}{
		{X: 1, Y: 1}: {},
		{X: 0, Y: 1}: {},
		{X: 0, Y: 0}: {},
		{X: 1, Y: 0}: {},
		{X: 2, Y: 0}: {},
		{X: 2, Y: 1}: {},
	}
	expectedSnap := Snapshot{
		Width:     width,
		Height:    height,
		Snake:     expectedSnake,
		Direction: DirectionRight,
		Food:      Point{X: 1, Y: 1},
		Tick:      1,
		GameOver:  true,
		Score:     0,
	}

	first := build(t)
	second := build(t)

	first.AdvanceTick()
	second.AdvanceTick()

	snapA := first.Snapshot()
	snapB := second.Snapshot()

	if !reflect.DeepEqual(snapA, expectedSnap) {
		t.Fatalf("unexpected snapshot after no-space game over: got %+v, want %+v", snapA, expectedSnap)
	}
	if !reflect.DeepEqual(snapA, snapB) {
		t.Fatalf("expected deterministic no-space snapshots, got %+v and %+v", snapA, snapB)
	}

	for _, engine := range []*Engine{first, second} {
		if len(engine.occupied) != len(expectedOccupied) {
			t.Fatalf("expected %d occupied cells, got %d", len(expectedOccupied), len(engine.occupied))
		}
		for point := range expectedOccupied {
			if _, ok := engine.occupied[point]; !ok {
				t.Fatalf("expected occupancy at %+v to persist", point)
			}
		}
		if engine.hasInput {
			t.Fatalf("queued input flag should clear after processing tick")
		}
		if engine.QueueDirection(DirectionLeft) {
			t.Fatalf("should not accept new input after game over")
		}
	}
}

func TestDeterministicCollisionOutcomesAcrossRuns(t *testing.T) {
	t.Parallel()

	type expectation struct {
		head      Point
		length    int
		direction Direction
	}

	tests := []struct {
		name   string
		build  func(t *testing.T) *Engine
		queue  Direction
		expect expectation
		verify func(t *testing.T, engine *Engine)
	}{
		{
			name: "boundary collision remains deterministic",
			build: func(t *testing.T) *Engine {
				engine := mustEngine(t, 1, 2, 9001)
				engine.food = Point{X: 0, Y: 0}
				return engine
			},
			queue: DirectionRight,
			expect: expectation{
				head:      Point{X: 0, Y: 1},
				length:    1,
				direction: DirectionRight,
			},
		},
		{
			name: "collision with other snake stays consistent",
			build: func(t *testing.T) *Engine {
				engine := mustEngine(t, 4, 3, 42424)
				engine.snake = []Point{{X: 1, Y: 1}}
				engine.occupied = map[Point]struct{}{
					{X: 1, Y: 1}: {},
					{X: 2, Y: 1}: {}, // other player segment
				}
				engine.direction = DirectionRight
				engine.food = Point{X: 0, Y: 0}
				return engine
			},
			queue: DirectionRight,
			expect: expectation{
				head:      Point{X: 1, Y: 1},
				length:    1,
				direction: DirectionRight,
			},
			verify: func(t *testing.T, engine *Engine) {
				if _, ok := engine.occupied[Point{X: 2, Y: 1}]; !ok {
					t.Fatalf("expected external occupancy to persist after collision")
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			snapshots := make([]Snapshot, 0, 2)

			for i := 0; i < 2; i++ {
				engine := tt.build(t)
				startTick := engine.tick

				if ok := engine.QueueDirection(tt.queue); !ok {
					t.Fatalf("expected to queue direction %v", tt.queue)
				}

				engine.AdvanceTick()
				snap := engine.Snapshot()

				if !snap.GameOver {
					t.Fatalf("expected collision to end the game")
				}
				if snap.Tick != startTick {
					t.Fatalf("tick should remain %d after immediate collision, got %d", startTick, snap.Tick)
				}
				if snap.Score != 0 {
					t.Fatalf("score should remain zero after collision, got %d", snap.Score)
				}
				if len(snap.Snake) != tt.expect.length {
					t.Fatalf("length mismatch: expected %d, got %d", tt.expect.length, len(snap.Snake))
				}
				if snap.Snake[0] != tt.expect.head {
					t.Fatalf("head mismatch: expected %+v, got %+v", tt.expect.head, snap.Snake[0])
				}
				if snap.Direction != tt.expect.direction {
					t.Fatalf("direction mismatch: expected %v, got %v", tt.expect.direction, snap.Direction)
				}
				if engine.hasInput {
					t.Fatalf("queued input should clear after collision")
				}

				if tt.verify != nil {
					tt.verify(t, engine)
				}

				snapshots = append(snapshots, snap)
			}

			if len(snapshots) != 2 {
				t.Fatalf("expected two deterministic runs, got %d", len(snapshots))
			}
			if !reflect.DeepEqual(snapshots[0], snapshots[1]) {
				t.Fatalf("expected deterministic snapshots across runs, got %+v and %+v", snapshots[0], snapshots[1])
			}
		})
	}
}
