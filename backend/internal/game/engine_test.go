package game

import (
	"reflect"
	"testing"
)

func TestNewEngineInitializesState(t *testing.T) {
	engine, err := NewEngine(7, 5, 42)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	snap := engine.Snapshot()

	if snap.Width != 7 || snap.Height != 5 {
		t.Fatalf("expected dimensions 7x5, got %dx%d", snap.Width, snap.Height)
	}
	if snap.Direction != DirectionRight {
		t.Fatalf("expected initial direction to be right")
	}
	if len(snap.Snake) != 1 {
		t.Fatalf("expected single-segment snake, got %d segments", len(snap.Snake))
	}

	head := snap.Snake[0]
	if head.X < 0 || head.X >= snap.Width || head.Y < 0 || head.Y >= snap.Height {
		t.Fatalf("expected head to be inside grid, got %+v", head)
	}

	if snap.Food.X < 0 || snap.Food.X >= snap.Width || snap.Food.Y < 0 || snap.Food.Y >= snap.Height {
		t.Fatalf("expected food to be inside grid, got %+v", snap.Food)
	}
	if reflect.DeepEqual(snap.Food, head) {
		t.Fatalf("food should not spawn on top of snake")
	}
	if snap.GameOver {
		t.Fatalf("game should not be over on initialization")
	}
}

func TestAdvanceTickMovesSnake(t *testing.T) {
	engine, err := NewEngine(5, 5, 1)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	start := engine.Snapshot().Snake[0]
	engine.AdvanceTick()
	afterRight := engine.Snapshot().Snake[0]

	if afterRight.X != start.X+1 || afterRight.Y != start.Y {
		t.Fatalf("expected head to move right from %+v, got %+v", start, afterRight)
	}

	engine.QueueDirection(DirectionDown)
	engine.AdvanceTick()
	afterDown := engine.Snapshot().Snake[0]

	if afterDown.X != afterRight.X || afterDown.Y != afterRight.Y+1 {
		t.Fatalf("expected head to move down from %+v, got %+v", afterRight, afterDown)
	}
}

func TestFoodConsumptionGrowsSnakeAndSpawnsNewFood(t *testing.T) {
	engine, err := NewEngine(6, 6, 3)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	head := engine.Snapshot().Snake[0]
	engine.food = Point{X: head.X + 1, Y: head.Y}

	engine.AdvanceTick()
	snap := engine.Snapshot()

	if len(snap.Snake) != 2 {
		t.Fatalf("expected snake to grow after eating, got length %d", len(snap.Snake))
	}
	if snap.Score != 1 {
		t.Fatalf("expected score to increment after eating, got %d", snap.Score)
	}
	for _, segment := range snap.Snake {
		if reflect.DeepEqual(segment, snap.Food) {
			t.Fatalf("food spawned on top of snake at %+v", snap.Food)
		}
	}
}

func TestWallCollisionEndsGame(t *testing.T) {
	engine, err := NewEngine(3, 3, 9)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	engine.QueueDirection(DirectionUp)
	engine.AdvanceTick() // move to boundary
	engine.AdvanceTick() // collide with wall

	if !engine.Snapshot().GameOver {
		t.Fatalf("expected game to end after wall collision")
	}
}

func TestSelfCollisionEndsGame(t *testing.T) {
	engine, err := NewEngine(5, 5, 5)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	start := engine.snake[0]

	engine.food = Point{X: start.X + 1, Y: start.Y}
	engine.AdvanceTick()

	engine.QueueDirection(DirectionDown)
	engine.food = Point{X: start.X + 1, Y: start.Y + 1}
	engine.AdvanceTick()

	engine.QueueDirection(DirectionLeft)
	engine.food = Point{X: start.X, Y: start.Y + 1}
	engine.AdvanceTick()

	engine.QueueDirection(DirectionRight)
	engine.AdvanceTick()

	if !engine.Snapshot().GameOver {
		t.Fatalf("expected game to end after self-collision")
	}
}

func TestDeterministicTicksWithSameSeed(t *testing.T) {
	first, err := NewEngine(6, 6, 123)
	if err != nil {
		t.Fatalf("unexpected error creating first engine: %v", err)
	}
	second, err := NewEngine(6, 6, 123)
	if err != nil {
		t.Fatalf("unexpected error creating second engine: %v", err)
	}

	head := first.snake[0]
	food := Point{X: head.X + 1, Y: head.Y}
	first.food = food
	second.food = food

	first.AdvanceTick()
	second.AdvanceTick()

	sequence := []Direction{DirectionDown, DirectionDown, DirectionLeft, DirectionUp, DirectionRight}
	for _, dir := range sequence {
		first.QueueDirection(dir)
		first.AdvanceTick()

		second.QueueDirection(dir)
		second.AdvanceTick()
	}

	if !reflect.DeepEqual(first.Snapshot(), second.Snapshot()) {
		t.Fatalf("expected deterministic state with identical inputs")
	}
}
