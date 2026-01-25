package game

import (
	"errors"
	"testing"
)

func TestNewEngineEmptyGridFails(t *testing.T) {
	t.Parallel()

	if _, err := NewEngine(0, 0, 7); !errors.Is(err, ErrInvalidDimensions) {
		t.Fatalf("expected ErrInvalidDimensions for empty grid, got %v", err)
	}
}

func TestQueueDirectionValidation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		setup        func(*Engine)
		direction    Direction
		wantQueued   bool
		wantHasInput bool
	}{
		{
			name:         "rejects invalid direction",
			direction:    Direction(-1),
			wantQueued:   false,
			wantHasInput: false,
		},
		{
			name: "rejects when game already over",
			setup: func(e *Engine) {
				e.gameOver = true
			},
			direction:    DirectionLeft,
			wantQueued:   false,
			wantHasInput: false,
		},
		{
			name:         "queues valid input",
			direction:    DirectionUp,
			wantQueued:   true,
			wantHasInput: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			engine, err := NewEngine(3, 3, 11)
			if err != nil {
				t.Fatalf("unexpected error creating engine: %v", err)
			}

			originalPending := engine.pending
			originalDirection := engine.direction

			if tc.setup != nil {
				tc.setup(engine)
			}

			queued := engine.QueueDirection(tc.direction)
			if queued != tc.wantQueued {
				t.Fatalf("expected queued=%v, got %v", tc.wantQueued, queued)
			}
			if engine.hasInput != tc.wantHasInput {
				t.Fatalf("expected hasInput=%v, got %v", tc.wantHasInput, engine.hasInput)
			}
			if !tc.wantHasInput && engine.pending != originalPending {
				t.Fatalf("pending direction changed on rejection: before %v after %v", originalPending, engine.pending)
			}
			if tc.wantHasInput && engine.pending != tc.direction {
				t.Fatalf("expected pending direction %v, got %v", tc.direction, engine.pending)
			}
			if engine.direction != originalDirection {
				t.Fatalf("direction should not change until tick advance; before %v after %v", originalDirection, engine.direction)
			}
		})
	}
}

func TestAdvanceTickRemovesTailWhenNotGrowing(t *testing.T) {
	engine, err := NewEngine(4, 2, 19)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	startHead := engine.snake[0]
	engine.food = Point{X: 0, Y: 0} // keep food away to avoid growth

	engine.AdvanceTick()
	snap := engine.Snapshot()

	if snap.GameOver {
		t.Fatalf("game should remain active after normal movement")
	}
	if snap.Score != 0 {
		t.Fatalf("score should remain unchanged without eating, got %d", snap.Score)
	}
	if snap.Tick != 1 {
		t.Fatalf("expected tick to increment to 1, got %d", snap.Tick)
	}
	if len(snap.Snake) != 1 {
		t.Fatalf("snake length should remain 1, got %d", len(snap.Snake))
	}
	if snap.Snake[0] == startHead {
		t.Fatalf("head should move away from start position %+v", startHead)
	}
	if _, stillOccupied := engine.occupied[startHead]; stillOccupied {
		t.Fatalf("tail should be cleared from occupied map after movement")
	}
}

func TestAdvanceTickCollisionOutcomes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		setup      func(*Engine) int
		wantOver   bool
		wantTick   int
		wantScore  int
		wantLength int
	}{
		{
			name: "wall collision stops tick",
			setup: func(e *Engine) int {
				setSnake(e, []Point{{0, 0}})
				e.direction = DirectionLeft
				return 1
			},
			wantOver:   true,
			wantTick:   0,
			wantScore:  0,
			wantLength: 1,
		},
		{
			name: "self collision ends game",
			setup: func(e *Engine) int {
				body := []Point{{1, 1}, {1, 2}, {2, 2}, {2, 1}}
				setSnake(e, body)
				e.direction = DirectionRight
				return len(body)
			},
			wantOver:   true,
			wantTick:   0,
			wantScore:  0,
			wantLength: 4,
		},
		{
			name: "collision with other snake",
			setup: func(e *Engine) int {
				setSnake(e, []Point{{1, 1}})
				e.occupied[Point{X: 2, Y: 1}] = struct{}{}
				e.direction = DirectionRight
				return 1
			},
			wantOver:   true,
			wantTick:   0,
			wantScore:  0,
			wantLength: 1,
		},
		{
			name: "food collision grows snake and continues",
			setup: func(e *Engine) int {
				setSnake(e, []Point{{1, 1}})
				e.direction = DirectionRight
				e.food = Point{X: 2, Y: 1}
				return 2
			},
			wantOver:   false,
			wantTick:   1,
			wantScore:  1,
			wantLength: 2,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			engine, err := NewEngine(4, 4, 23)
			if err != nil {
				t.Fatalf("unexpected error creating engine: %v", err)
			}

			expectedLength := tc.setup(engine)

			engine.AdvanceTick()
			snap := engine.Snapshot()

			if snap.GameOver != tc.wantOver {
				t.Fatalf("expected gameOver=%v, got %v", tc.wantOver, snap.GameOver)
			}
			if snap.Tick != tc.wantTick {
				t.Fatalf("expected tick %d, got %d", tc.wantTick, snap.Tick)
			}
			if snap.Score != tc.wantScore {
				t.Fatalf("expected score %d, got %d", tc.wantScore, snap.Score)
			}
			if len(snap.Snake) != expectedLength || len(snap.Snake) != tc.wantLength {
				t.Fatalf("expected snake length %d, got %d", tc.wantLength, len(snap.Snake))
			}
		})
	}
}

func TestFoodRespawnRespectsOccupiedCells(t *testing.T) {
	engine, err := NewEngine(3, 3, 31)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	primarySnake := []Point{{1, 1}, {1, 0}}
	setSnake(engine, primarySnake)
	engine.direction = DirectionRight
	engine.food = Point{X: 2, Y: 1}

	otherSnake := []Point{{0, 0}, {0, 1}, {0, 2}, {1, 2}, {2, 2}}
	for _, p := range otherSnake {
		engine.occupied[p] = struct{}{}
	}

	engine.AdvanceTick()
	snap := engine.Snapshot()

	expectedFood := Point{X: 2, Y: 0} // only remaining free cell

	if snap.GameOver {
		t.Fatalf("game should continue after eating when a free cell exists")
	}
	if snap.Score != 1 {
		t.Fatalf("expected score 1 after eating, got %d", snap.Score)
	}
	if len(snap.Snake) != len(primarySnake)+1 {
		t.Fatalf("expected snake to grow to length %d, got %d", len(primarySnake)+1, len(snap.Snake))
	}
	if snap.Food != expectedFood {
		t.Fatalf("expected food to respawn at %+v, got %+v", expectedFood, snap.Food)
	}
	if _, taken := engine.occupied[snap.Food]; taken {
		t.Fatalf("respawned food should not land on occupied cells, found at %+v", snap.Food)
	}
}

func TestAdvanceTickAtBoundaryStaysAlive(t *testing.T) {
	engine, err := NewEngine(3, 3, 41)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	setSnake(engine, []Point{{0, 0}})
	engine.direction = DirectionRight
	engine.food = Point{X: 2, Y: 2} // keep food away from the path

	engine.AdvanceTick()

	if ok := engine.QueueDirection(DirectionDown); !ok {
		t.Fatalf("expected to queue direction while game is active")
	}
	engine.AdvanceTick()
	snap := engine.Snapshot()

	expectedHead := Point{X: 1, Y: 1}
	if snap.GameOver {
		t.Fatalf("game should remain active when moving along boundary")
	}
	if snap.Tick != 2 {
		t.Fatalf("expected tick to be 2, got %d", snap.Tick)
	}
	if snap.Snake[0] != expectedHead {
		t.Fatalf("expected head at %+v, got %+v", expectedHead, snap.Snake[0])
	}
	if snap.Score != 0 {
		t.Fatalf("score should remain 0 without eating, got %d", snap.Score)
	}
}
