package game

import (
	"errors"
	"testing"
)

// setSnake rewrites the engine snake and occupied map for custom scenarios.
func setSnake(e *Engine, points []Point) {
	e.snake = append([]Point(nil), points...)
	e.occupied = make(map[Point]struct{}, len(points))
	for _, p := range points {
		e.occupied[p] = struct{}{}
	}
}

func TestNewEngineValidation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		width   int
		height  int
		wantErr error
	}{
		{name: "negative width", width: -1, height: 5, wantErr: ErrInvalidDimensions},
		{name: "zero height", width: 4, height: 0, wantErr: ErrInvalidDimensions},
		{name: "no space for food", width: 1, height: 1, wantErr: ErrNoFreeCells},
		{name: "valid grid", width: 2, height: 2, wantErr: nil},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			engine, err := NewEngine(tc.width, tc.height, 99)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if engine == nil {
				t.Fatalf("engine should not be nil")
			}
		})
	}
}

func TestAdvanceTickUpdatesDirectionAndTick(t *testing.T) {
	engine, err := NewEngine(5, 5, 7)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	start := engine.Snapshot()
	if ok := engine.QueueDirection(DirectionDown); !ok {
		t.Fatalf("expected direction to queue")
	}

	engine.AdvanceTick()
	snap := engine.Snapshot()

	expectedHead := Point{X: start.Snake[0].X, Y: start.Snake[0].Y + 1}
	if snap.Snake[0] != expectedHead {
		t.Fatalf("expected head to move to %+v, got %+v", expectedHead, snap.Snake[0])
	}
	if snap.Direction != DirectionDown {
		t.Fatalf("expected direction to update to down")
	}
	if snap.Tick != start.Tick+1 {
		t.Fatalf("expected tick to increment from %d to %d", start.Tick, snap.Tick)
	}
}

func TestAdvanceTickIgnoresInputAfterGameOver(t *testing.T) {
	engine, err := NewEngine(4, 4, 5)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	engine.gameOver = true
	engine.tick = 2

	if ok := engine.QueueDirection(DirectionLeft); ok {
		t.Fatalf("queueing direction should fail when game is over")
	}

	engine.AdvanceTick()
	snap := engine.Snapshot()

	if snap.Tick != 2 {
		t.Fatalf("expected tick to remain unchanged after game over, got %d", snap.Tick)
	}
	if snap.Direction != DirectionRight {
		t.Fatalf("direction should remain unchanged after game over")
	}
}

func TestWallCollisionEndsGameFromAllSides(t *testing.T) {
	cases := []struct {
		name      string
		head      Point
		direction Direction
	}{
		{name: "left wall", head: Point{X: 0, Y: 1}, direction: DirectionLeft},
		{name: "right wall", head: Point{X: 2, Y: 1}, direction: DirectionRight},
		{name: "top wall", head: Point{X: 1, Y: 0}, direction: DirectionUp},
		{name: "bottom wall", head: Point{X: 1, Y: 2}, direction: DirectionDown},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			engine, err := NewEngine(3, 3, 11)
			if err != nil {
				t.Fatalf("unexpected error creating engine: %v", err)
			}

			setSnake(engine, []Point{tc.head})
			engine.direction = tc.direction
			engine.tick = 5

			engine.AdvanceTick()
			snap := engine.Snapshot()

			if !snap.GameOver {
				t.Fatalf("expected game over when moving %v from %+v", tc.direction, tc.head)
			}
			if snap.Tick != 5 {
				t.Fatalf("tick should not advance after wall collision, got %d", snap.Tick)
			}
			if snap.Snake[0] != tc.head {
				t.Fatalf("snake should not move after collision; expected head %+v, got %+v", tc.head, snap.Snake[0])
			}
		})
	}
}

func TestSelfCollisionEndsGameImmediately(t *testing.T) {
	engine, err := NewEngine(5, 5, 13)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	body := []Point{{2, 2}, {2, 1}, {1, 1}, {1, 2}}
	setSnake(engine, body)
	engine.direction = DirectionLeft

	engine.AdvanceTick()
	snap := engine.Snapshot()

	if !snap.GameOver {
		t.Fatalf("expected game over on self-collision")
	}
	if snap.Tick != 0 {
		t.Fatalf("tick should not advance after self-collision, got %d", snap.Tick)
	}
	if len(snap.Snake) != len(body) {
		t.Fatalf("snake length should remain %d on collision, got %d", len(body), len(snap.Snake))
	}
}

func TestSnakeToSnakeCollisionEndsGame(t *testing.T) {
	engine, err := NewEngine(4, 4, 21)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	head := Point{X: 1, Y: 1}
	setSnake(engine, []Point{head})
	engine.direction = DirectionRight

	otherSnake := Point{X: 2, Y: 1}
	engine.occupied[otherSnake] = struct{}{}
	engine.food = Point{X: 0, Y: 0}

	engine.AdvanceTick()
	snap := engine.Snapshot()

	if !snap.GameOver {
		t.Fatalf("expected game over when colliding with another snake at %+v", otherSnake)
	}
	if snap.Tick != 0 {
		t.Fatalf("tick should not advance after snake-to-snake collision, got %d", snap.Tick)
	}
}

func TestFoodConsumptionGrowsSnakeAndRespawns(t *testing.T) {
	engine, err := NewEngine(6, 6, 3)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	head := engine.Snapshot().Snake[0]
	engine.food = Point{X: head.X + 1, Y: head.Y}

	engine.AdvanceTick()
	snap := engine.Snapshot()

	if len(snap.Snake) != 2 {
		t.Fatalf("expected snake length 2 after eating, got %d", len(snap.Snake))
	}
	if snap.Score != 1 {
		t.Fatalf("expected score to increment to 1, got %d", snap.Score)
	}
	if snap.GameOver {
		t.Fatalf("game should continue after successful food consumption")
	}
	for _, segment := range snap.Snake {
		if segment == snap.Food {
			t.Fatalf("respawned food should not overlap snake at %+v", snap.Food)
		}
	}
}

func TestFoodRespawnsInOnlyFreeCell(t *testing.T) {
	engine, err := NewEngine(3, 2, 8)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	snake := []Point{{0, 0}, {0, 1}, {1, 1}, {2, 1}}
	setSnake(engine, snake)
	engine.direction = DirectionRight
	engine.food = Point{X: 1, Y: 0}

	engine.AdvanceTick()
	snap := engine.Snapshot()

	expectedFood := Point{X: 2, Y: 0}
	if snap.Food != expectedFood {
		t.Fatalf("expected food to respawn at the only free cell %+v, got %+v", expectedFood, snap.Food)
	}
	if snap.Score != 1 {
		t.Fatalf("expected score 1 after eating, got %d", snap.Score)
	}
	if len(snap.Snake) != 5 {
		t.Fatalf("expected snake length 5 after growth, got %d", len(snap.Snake))
	}
}

func TestEatingLastFreeCellEndsGame(t *testing.T) {
	engine, err := NewEngine(2, 2, 2)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	snake := []Point{{0, 0}, {0, 1}, {1, 1}}
	setSnake(engine, snake)
	engine.direction = DirectionRight
	engine.food = Point{X: 1, Y: 0}

	engine.AdvanceTick()
	snap := engine.Snapshot()

	if !snap.GameOver {
		t.Fatalf("expected game over when no free cells remain after eating")
	}
	if snap.Score != 0 {
		t.Fatalf("score should not increment when food cannot respawn, got %d", snap.Score)
	}
	if snap.Tick != 1 {
		t.Fatalf("expected tick to increment to 1 after move, got %d", snap.Tick)
	}
	if len(snap.Snake) != 4 {
		t.Fatalf("snake should grow before detecting no free cells, expected length 4 got %d", len(snap.Snake))
	}
}

func TestScoreAccumulatesAcrossMultipleFoods(t *testing.T) {
	engine, err := NewEngine(6, 6, 4)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	head := engine.Snapshot().Snake[0]

	// First food directly to the right.
	engine.food = Point{X: head.X + 1, Y: head.Y}
	engine.AdvanceTick()

	// Second food directly below the new head.
	newHead := engine.Snapshot().Snake[0]
	engine.QueueDirection(DirectionDown)
	engine.food = Point{X: newHead.X, Y: newHead.Y + 1}
	engine.AdvanceTick()

	snap := engine.Snapshot()
	if snap.Score != 2 {
		t.Fatalf("expected score 2 after two foods, got %d", snap.Score)
	}
	if len(snap.Snake) != 3 {
		t.Fatalf("expected snake length 3 after two growths, got %d", len(snap.Snake))
	}
	if snap.GameOver {
		t.Fatalf("game should remain active after consecutive food consumption")
	}
}
