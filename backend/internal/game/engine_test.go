package game

import (
	"errors"
	"math/rand"
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

	engine.food = Point{X: 0, Y: 0}

	startSnap := engine.Snapshot()
	start := startSnap.Snake[0]

	engine.AdvanceTick()
	snapAfterRight := engine.Snapshot()
	afterRight := snapAfterRight.Snake[0]

	if afterRight.X != start.X+1 || afterRight.Y != start.Y {
		t.Fatalf("expected head to move right from %+v, got %+v", start, afterRight)
	}
	if snapAfterRight.Tick != startSnap.Tick+1 {
		t.Fatalf("tick should increment after movement, expected %d got %d", startSnap.Tick+1, snapAfterRight.Tick)
	}
	if len(snapAfterRight.Snake) != len(startSnap.Snake) {
		t.Fatalf("snake length should remain %d when not eating, got %d", len(startSnap.Snake), len(snapAfterRight.Snake))
	}
	if snapAfterRight.Score != 0 {
		t.Fatalf("score should not change without eating, got %d", snapAfterRight.Score)
	}
	if snapAfterRight.Direction != DirectionRight {
		t.Fatalf("direction should remain right until new input, got %v", snapAfterRight.Direction)
	}

	engine.QueueDirection(DirectionDown)
	engine.AdvanceTick()
	snapAfterDown := engine.Snapshot()
	afterDown := snapAfterDown.Snake[0]

	if afterDown.X != afterRight.X || afterDown.Y != afterRight.Y+1 {
		t.Fatalf("expected head to move down from %+v, got %+v", afterRight, afterDown)
	}
	if snapAfterDown.Tick != snapAfterRight.Tick+1 {
		t.Fatalf("tick should increment after second movement, expected %d got %d", snapAfterRight.Tick+1, snapAfterDown.Tick)
	}
	if len(snapAfterDown.Snake) != len(snapAfterRight.Snake) {
		t.Fatalf("snake length should remain %d when no food eaten, got %d", len(snapAfterRight.Snake), len(snapAfterDown.Snake))
	}
	if snapAfterDown.Score != snapAfterRight.Score {
		t.Fatalf("score should remain unchanged, expected %d got %d", snapAfterRight.Score, snapAfterDown.Score)
	}
	if snapAfterDown.Direction != DirectionDown {
		t.Fatalf("expected direction to update to %v, got %v", DirectionDown, snapAfterDown.Direction)
	}
}

func TestAdvanceTickAppliesLatestQueuedDirection(t *testing.T) {
	engine, err := NewEngine(6, 6, 17)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	engine.food = Point{X: 0, Y: 0}
	start := engine.Snapshot()

	if queued := engine.QueueDirection(DirectionUp); !queued {
		t.Fatalf("expected to queue first direction")
	}
	if queued := engine.QueueDirection(DirectionLeft); !queued {
		t.Fatalf("expected to queue overriding direction")
	}

	engine.AdvanceTick()
	afterFirst := engine.Snapshot()

	expectedFirstHead := Point{X: start.Snake[0].X - 1, Y: start.Snake[0].Y}
	if afterFirst.Snake[0] != expectedFirstHead {
		t.Fatalf("expected head %+v after applying latest queued direction, got %+v", expectedFirstHead, afterFirst.Snake[0])
	}
	if afterFirst.Direction != DirectionLeft {
		t.Fatalf("direction mismatch: expected %v, got %v", DirectionLeft, afterFirst.Direction)
	}
	if afterFirst.Tick != start.Tick+1 {
		t.Fatalf("tick should advance by one, expected %d got %d", start.Tick+1, afterFirst.Tick)
	}
	if engine.hasInput {
		t.Fatalf("expected input flag to clear after advancing tick")
	}

	if queued := engine.QueueDirection(DirectionUp); !queued {
		t.Fatalf("expected to queue a new direction after movement")
	}

	engine.AdvanceTick()
	afterSecond := engine.Snapshot()
	expectedSecondHead := Point{X: expectedFirstHead.X, Y: expectedFirstHead.Y - 1}

	if afterSecond.Snake[0] != expectedSecondHead {
		t.Fatalf("expected head %+v after second movement, got %+v", expectedSecondHead, afterSecond.Snake[0])
	}
	if afterSecond.Direction != DirectionUp {
		t.Fatalf("direction mismatch after second tick: expected %v, got %v", DirectionUp, afterSecond.Direction)
	}
	if afterSecond.Tick != afterFirst.Tick+1 {
		t.Fatalf("tick should advance again, expected %d got %d", afterFirst.Tick+1, afterSecond.Tick)
	}
	if afterSecond.Score != 0 {
		t.Fatalf("score should remain unchanged without food, got %d", afterSecond.Score)
	}
	if len(afterSecond.Snake) != 1 {
		t.Fatalf("snake length should not change without eating, got %d", len(afterSecond.Snake))
	}
	if engine.hasInput {
		t.Fatalf("expected queued input to reset after processing second tick")
	}
}

func TestMoveSnakeUpdatesOccupancy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		snake            []Point
		occupied         map[Point]struct{}
		next             Point
		grew             bool
		expectedSnake    []Point
		expectedOccupied map[Point]struct{}
	}{
		{
			name:  "moves without growth and removes tail",
			snake: []Point{{X: 1, Y: 1}, {X: 1, Y: 2}, {X: 1, Y: 3}},
			occupied: map[Point]struct{}{
				{X: 1, Y: 1}: {},
				{X: 1, Y: 2}: {},
				{X: 1, Y: 3}: {},
				{X: 0, Y: 0}: {},
			},
			next: Point{X: 2, Y: 1},
			grew: false,
			expectedSnake: []Point{
				{X: 2, Y: 1},
				{X: 1, Y: 1},
				{X: 1, Y: 2},
			},
			expectedOccupied: map[Point]struct{}{
				{X: 2, Y: 1}: {},
				{X: 1, Y: 1}: {},
				{X: 1, Y: 2}: {},
				{X: 0, Y: 0}: {},
			},
		},
		{
			name:  "growing preserves tail and occupancy",
			snake: []Point{{X: 2, Y: 2}, {X: 2, Y: 3}},
			occupied: map[Point]struct{}{
				{X: 2, Y: 2}: {},
				{X: 2, Y: 3}: {},
			},
			next: Point{X: 3, Y: 2},
			grew: true,
			expectedSnake: []Point{
				{X: 3, Y: 2},
				{X: 2, Y: 2},
				{X: 2, Y: 3},
			},
			expectedOccupied: map[Point]struct{}{
				{X: 3, Y: 2}: {},
				{X: 2, Y: 2}: {},
				{X: 2, Y: 3}: {},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			engine := &Engine{
				snake:    append([]Point(nil), tt.snake...),
				occupied: map[Point]struct{}{},
			}
			for point := range tt.occupied {
				engine.occupied[point] = struct{}{}
			}

			engine.moveSnake(tt.next, tt.grew)

			if !reflect.DeepEqual(engine.snake, tt.expectedSnake) {
				t.Fatalf("expected snake %+v, got %+v", tt.expectedSnake, engine.snake)
			}

			if len(engine.occupied) != len(tt.expectedOccupied) {
				t.Fatalf("expected %d occupied cells, got %d", len(tt.expectedOccupied), len(engine.occupied))
			}
			for point := range tt.expectedOccupied {
				if _, ok := engine.occupied[point]; !ok {
					t.Fatalf("expected occupancy at %+v to remain set", point)
				}
			}
		})
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

	if snap.Tick != 1 {
		t.Fatalf("expected tick to advance after eating, got %d", snap.Tick)
	}
	if snap.GameOver {
		t.Fatalf("game should continue after eating food")
	}
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

func TestAdvanceTickSpawnsFoodDeterministically(t *testing.T) {
	engine, err := NewEngine(4, 3, 3)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	seed := int64(77)
	engine.rng = rand.New(rand.NewSource(seed))
	engine.snake = []Point{{X: 1, Y: 1}}
	engine.occupied = map[Point]struct{}{
		{X: 1, Y: 1}: {},
	}
	engine.direction = DirectionRight
	engine.food = Point{X: 2, Y: 1}

	occupiedAfterMove := map[Point]struct{}{
		{X: 1, Y: 1}: {},
		{X: 2, Y: 1}: {},
	}
	expectedRng := rand.New(rand.NewSource(seed))
	freeCells := make([]Point, 0, engine.width*engine.height-len(occupiedAfterMove))
	for y := 0; y < engine.height; y++ {
		for x := 0; x < engine.width; x++ {
			point := Point{X: x, Y: y}
			if _, taken := occupiedAfterMove[point]; !taken {
				freeCells = append(freeCells, point)
			}
		}
	}
	expectedFood := freeCells[expectedRng.Intn(len(freeCells))]

	engine.AdvanceTick()
	snap := engine.Snapshot()

	if snap.GameOver {
		t.Fatalf("did not expect game over while respawning deterministic food")
	}
	if snap.Tick != 1 {
		t.Fatalf("expected tick to advance, got %d", snap.Tick)
	}
	if snap.Score != 1 {
		t.Fatalf("expected score to increment after eating, got %d", snap.Score)
	}
	if len(snap.Snake) != 2 {
		t.Fatalf("expected snake to grow after eating, got length %d", len(snap.Snake))
	}
	if snap.Snake[0] != (Point{X: 2, Y: 1}) {
		t.Fatalf("expected head to move right onto food, got %+v", snap.Snake[0])
	}
	if snap.Food != expectedFood {
		t.Fatalf("expected deterministic food respawn at %+v, got %+v", expectedFood, snap.Food)
	}
}

func TestRandomEmptyCellRespectsExternalOccupancy(t *testing.T) {
	engine, err := NewEngine(3, 3, 5)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	engine.snake = []Point{{X: 1, Y: 1}}
	engine.occupied = map[Point]struct{}{
		{X: 1, Y: 1}: {},
		{X: 0, Y: 0}: {},
		{X: 2, Y: 2}: {},
	}

	seed := int64(7)
	engine.rng = rand.New(rand.NewSource(seed))

	free := make([]Point, 0, engine.width*engine.height-len(engine.occupied))
	for y := 0; y < engine.height; y++ {
		for x := 0; x < engine.width; x++ {
			point := Point{X: x, Y: y}
			if _, taken := engine.occupied[point]; !taken {
				free = append(free, point)
			}
		}
	}
	expected := free[rand.New(rand.NewSource(seed)).Intn(len(free))]

	point, err := engine.randomEmptyCell()
	if err != nil {
		t.Fatalf("unexpected error finding empty cell: %v", err)
	}
	if point != expected {
		t.Fatalf("expected deterministic free cell %+v, got %+v", expected, point)
	}
	if _, taken := engine.occupied[point]; taken {
		t.Fatalf("returned cell %+v should not be occupied", point)
	}
}

func TestWallCollisionEndsGame(t *testing.T) {
	engine, err := NewEngine(3, 3, 9)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	engine.food = Point{X: 0, Y: 0}

	engine.QueueDirection(DirectionUp)
	engine.AdvanceTick() // move to boundary
	engine.AdvanceTick() // collide with wall

	snap := engine.Snapshot()

	if !snap.GameOver {
		t.Fatalf("expected game to end after wall collision")
	}
	if snap.Tick != 1 {
		t.Fatalf("tick should not advance after wall collision, got %d", snap.Tick)
	}
	if snap.Score != 0 {
		t.Fatalf("score should remain zero on wall collision, got %d", snap.Score)
	}
	if len(snap.Snake) != 1 {
		t.Fatalf("snake length should remain unchanged on wall collision, got %d", len(snap.Snake))
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

	snap := engine.Snapshot()

	if !snap.GameOver {
		t.Fatalf("expected game to end after self-collision")
	}
	if snap.Tick != 3 {
		t.Fatalf("tick should stop after collision, got %d", snap.Tick)
	}
	if snap.Score != 3 {
		t.Fatalf("score should reflect consumed food before collision, got %d", snap.Score)
	}
	if len(snap.Snake) != 4 {
		t.Fatalf("snake length should remain after collision, got %d", len(snap.Snake))
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

func TestSnapshotReturnsIndependentCopy(t *testing.T) {
	engine, err := NewEngine(4, 4, 91)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	snap := engine.Snapshot()
	modified := snap
	modified.Snake[0] = Point{X: -1, Y: -1}
	modified.Snake = append(modified.Snake, Point{X: -2, Y: -2})
	modified.Food = Point{X: -3, Y: -3}
	modified.Score = 99
	modified.Direction = DirectionLeft

	latest := engine.Snapshot()

	if latest.Snake[0] == modified.Snake[0] || len(latest.Snake) != len(snap.Snake) {
		t.Fatalf("expected snapshot mutations to leave engine snake unchanged, got %+v", latest.Snake)
	}
	if latest.Food == modified.Food {
		t.Fatalf("engine food should remain unchanged after snapshot mutation")
	}
	if latest.Score == modified.Score {
		t.Fatalf("engine score should remain unchanged after snapshot mutation")
	}
	if latest.Direction == modified.Direction && latest.Direction == DirectionLeft {
		t.Fatalf("engine direction should remain unchanged after snapshot mutation")
	}
}

func TestNewEngineValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		width  int
		height int
		err    error
	}{
		{name: "zero width", width: 0, height: 5, err: ErrInvalidDimensions},
		{name: "zero height", width: 5, height: 0, err: ErrInvalidDimensions},
		{name: "empty grid", width: 0, height: 0, err: ErrInvalidDimensions},
		{name: "no free cells", width: 1, height: 1, err: ErrNoFreeCells},
		{name: "negative dimensions", width: -2, height: 3, err: ErrInvalidDimensions},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			engine, err := NewEngine(tt.width, tt.height, 99)
			if !errors.Is(err, tt.err) {
				t.Fatalf("expected error %v, got %v", tt.err, err)
			}
			if engine != nil {
				t.Fatalf("expected engine to be nil when initialization fails")
			}
		})
	}
}

func TestQueueDirectionValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		direction Direction
		gameOver  bool
		queued    bool
	}{
		{name: "rejects invalid direction", direction: Direction(-1), queued: false},
		{name: "rejects when game over", direction: DirectionLeft, gameOver: true, queued: false},
		{name: "queues valid direction", direction: DirectionUp, queued: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			engine, err := NewEngine(4, 4, 7)
			if err != nil {
				t.Fatalf("unexpected error creating engine: %v", err)
			}

			engine.gameOver = tt.gameOver

			queued := engine.QueueDirection(tt.direction)
			if queued != tt.queued {
				t.Fatalf("queue result mismatch: expected %v, got %v", tt.queued, queued)
			}

			if tt.queued {
				if !engine.hasInput {
					t.Fatalf("expected queued input to be recorded")
				}
				if engine.pending != tt.direction {
					t.Fatalf("expected pending direction %v, got %v", tt.direction, engine.pending)
				}
				if engine.direction != DirectionRight {
					t.Fatalf("direction should not change before advancing tick")
				}
			} else if engine.hasInput {
				t.Fatalf("did not expect direction to queue when input should be rejected")
			}
		})
	}
}

func TestAdvanceTickRejectsInvalidInputButContinuesMovement(t *testing.T) {
	engine, err := NewEngine(4, 4, 73)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	start := engine.Snapshot()
	engine.food = Point{X: 0, Y: 0}

	if queued := engine.QueueDirection(Direction(-99)); queued {
		t.Fatalf("expected invalid direction to be rejected")
	}

	engine.AdvanceTick()
	snap := engine.Snapshot()

	expectedHead := Point{X: start.Snake[0].X + 1, Y: start.Snake[0].Y}
	if snap.Snake[0] != expectedHead {
		t.Fatalf("expected head to advance right to %+v, got %+v", expectedHead, snap.Snake[0])
	}
	if snap.Direction != DirectionRight {
		t.Fatalf("direction should remain unchanged after rejecting input, got %v", snap.Direction)
	}
	if snap.Tick != start.Tick+1 {
		t.Fatalf("expected tick to increment from %d to %d, got %d", start.Tick, start.Tick+1, snap.Tick)
	}
	if snap.GameOver {
		t.Fatalf("game should continue after ignoring invalid input")
	}
	if snap.Score != start.Score {
		t.Fatalf("score should remain unchanged without eating, got %d", snap.Score)
	}
	if engine.hasInput {
		t.Fatalf("engine should not retain rejected input")
	}
}

func TestAdvanceTickCollisionScenarios(t *testing.T) {
	tests := []struct {
		name             string
		setup            func(t *testing.T) *Engine
		steps            []Direction
		expectedHead     func(e *Engine) Point
		staticHead       Point
		expectedLength   int
		expectedScore    int
		expectedTick     int
		expectedGameOver bool
		verify           func(t *testing.T, snap Snapshot)
	}{
		{
			name: "snake hits wall",
			setup: func(t *testing.T) *Engine {
				engine, err := NewEngine(3, 3, 1)
				if err != nil {
					t.Fatalf("unexpected error creating engine: %v", err)
				}
				engine.food = Point{X: 0, Y: 0}
				return engine
			},
			steps:            []Direction{DirectionUp, DirectionUp},
			staticHead:       Point{X: 1, Y: 0},
			expectedLength:   1,
			expectedScore:    0,
			expectedTick:     1,
			expectedGameOver: true,
		},
		{
			name: "self collision",
			setup: func(t *testing.T) *Engine {
				engine, err := NewEngine(4, 4, 5)
				if err != nil {
					t.Fatalf("unexpected error creating engine: %v", err)
				}

				engine.snake = []Point{{1, 1}, {1, 2}, {1, 3}}
				engine.occupied = map[Point]struct{}{
					{X: 1, Y: 1}: {},
					{X: 1, Y: 2}: {},
					{X: 1, Y: 3}: {},
				}
				engine.direction = DirectionDown
				engine.food = Point{X: 0, Y: 0}

				return engine
			},
			steps:            []Direction{DirectionDown},
			staticHead:       Point{X: 1, Y: 1},
			expectedLength:   3,
			expectedScore:    0,
			expectedTick:     0,
			expectedGameOver: true,
		},
		{
			name: "collision with other snake",
			setup: func(t *testing.T) *Engine {
				engine, err := NewEngine(4, 4, 11)
				if err != nil {
					t.Fatalf("unexpected error creating engine: %v", err)
				}

				engine.occupied[Point{X: 3, Y: 2}] = struct{}{}
				engine.food = Point{X: 0, Y: 0}

				return engine
			},
			steps:            []Direction{DirectionRight},
			staticHead:       Point{X: 2, Y: 2},
			expectedLength:   1,
			expectedScore:    0,
			expectedTick:     0,
			expectedGameOver: true,
		},
		{
			name: "eats food and grows",
			setup: func(t *testing.T) *Engine {
				engine, err := NewEngine(5, 5, 13)
				if err != nil {
					t.Fatalf("unexpected error creating engine: %v", err)
				}

				head := engine.snake[0]
				engine.food = Point{X: head.X + 1, Y: head.Y}

				return engine
			},
			steps: []Direction{DirectionRight},
			expectedHead: func(e *Engine) Point {
				return Point{X: e.snake[0].X + 1, Y: e.snake[0].Y}
			},
			expectedLength:   2,
			expectedScore:    1,
			expectedTick:     1,
			expectedGameOver: false,
			verify: func(t *testing.T, snap Snapshot) {
				for _, segment := range snap.Snake {
					if reflect.DeepEqual(segment, snap.Food) {
						t.Fatalf("food should not respawn on the snake")
					}
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			engine := tt.setup(t)

			expectedHead := tt.staticHead
			if tt.expectedHead != nil {
				expectedHead = tt.expectedHead(engine)
			}

			for _, direction := range tt.steps {
				if ok := engine.QueueDirection(direction); !ok && !engine.gameOver {
					t.Fatalf("failed to queue direction %v", direction)
				}
				engine.AdvanceTick()
			}

			snap := engine.Snapshot()

			if engine.gameOver != tt.expectedGameOver {
				t.Fatalf("engine gameOver flag mismatch: expected %v, got %v", tt.expectedGameOver, engine.gameOver)
			}
			if snap.GameOver != tt.expectedGameOver {
				t.Fatalf("game over mismatch: expected %v, got %v", tt.expectedGameOver, snap.GameOver)
			}
			if snap.Tick != tt.expectedTick {
				t.Fatalf("tick mismatch: expected %d, got %d", tt.expectedTick, snap.Tick)
			}
			if len(snap.Snake) != tt.expectedLength {
				t.Fatalf("snake length mismatch: expected %d, got %d", tt.expectedLength, len(snap.Snake))
			}
			if snap.Snake[0] != expectedHead {
				t.Fatalf("head position mismatch: expected %+v, got %+v", expectedHead, snap.Snake[0])
			}
			if snap.Score != tt.expectedScore {
				t.Fatalf("score mismatch: expected %d, got %d", tt.expectedScore, snap.Score)
			}

			if tt.verify != nil {
				tt.verify(t, snap)
			}
		})
	}
}

func TestAdvanceTickCollisionDetectionTable(t *testing.T) {
	directionPtr := func(dir Direction) *Direction {
		return &dir
	}

	tests := []struct {
		name             string
		setup            func(t *testing.T) *Engine
		queuedDirection  *Direction
		expectedHead     Point
		expectedLength   int
		expectedScore    int
		expectedTick     int
		expectedGameOver bool
		verify           func(t *testing.T, engine *Engine, snap Snapshot)
	}{
		{
			name: "wall collision from boundary start",
			setup: func(t *testing.T) *Engine {
				engine, err := NewEngine(3, 3, 101)
				if err != nil {
					t.Fatalf("unexpected error creating engine: %v", err)
				}

				engine.snake = []Point{{X: 0, Y: 0}}
				engine.occupied = map[Point]struct{}{
					{X: 0, Y: 0}: {},
				}
				engine.direction = DirectionRight
				engine.food = Point{X: 2, Y: 2}

				return engine
			},
			queuedDirection:  directionPtr(DirectionLeft),
			expectedHead:     Point{X: 0, Y: 0},
			expectedLength:   1,
			expectedScore:    0,
			expectedTick:     0,
			expectedGameOver: true,
		},
		{
			name: "self collision when reversing into body",
			setup: func(t *testing.T) *Engine {
				engine, err := NewEngine(4, 4, 7)
				if err != nil {
					t.Fatalf("unexpected error creating engine: %v", err)
				}

				engine.snake = []Point{{X: 1, Y: 1}, {X: 0, Y: 1}}
				engine.occupied = map[Point]struct{}{
					{X: 1, Y: 1}: {},
					{X: 0, Y: 1}: {},
				}
				engine.direction = DirectionRight
				engine.food = Point{X: 3, Y: 3}

				return engine
			},
			queuedDirection:  directionPtr(DirectionLeft),
			expectedHead:     Point{X: 1, Y: 1},
			expectedLength:   2,
			expectedScore:    0,
			expectedTick:     0,
			expectedGameOver: true,
			verify: func(t *testing.T, engine *Engine, snap Snapshot) {
				if len(engine.occupied) != 2 {
					t.Fatalf("expected body occupancy to remain after collision, got %d cells", len(engine.occupied))
				}
				if snap.Snake[0] != (Point{X: 1, Y: 1}) {
					t.Fatalf("expected head to remain at starting cell after collision, got %+v", snap.Snake[0])
				}
			},
		},
		{
			name: "collision with other snake occupancy",
			setup: func(t *testing.T) *Engine {
				engine, err := NewEngine(4, 4, 11)
				if err != nil {
					t.Fatalf("unexpected error creating engine: %v", err)
				}

				engine.snake = []Point{{X: 1, Y: 1}}
				engine.occupied = map[Point]struct{}{
					{X: 1, Y: 1}: {},
					{X: 2, Y: 1}: {}, // other player
				}
				engine.direction = DirectionRight
				engine.food = Point{X: 0, Y: 0}

				return engine
			},
			expectedHead:     Point{X: 1, Y: 1},
			expectedLength:   1,
			expectedScore:    0,
			expectedTick:     0,
			expectedGameOver: true,
			verify: func(t *testing.T, engine *Engine, _ Snapshot) {
				if _, ok := engine.occupied[Point{X: 2, Y: 1}]; !ok {
					t.Fatalf("expected other player occupancy to remain recorded")
				}
			},
		},
		{
			name: "eats food and avoids external occupancy on respawn",
			setup: func(t *testing.T) *Engine {
				engine, err := NewEngine(4, 4, 33)
				if err != nil {
					t.Fatalf("unexpected error creating engine: %v", err)
				}

				engine.snake = []Point{{X: 2, Y: 2}}
				engine.occupied = map[Point]struct{}{
					{X: 2, Y: 2}: {},
					{X: 0, Y: 0}: {}, // other snake segment
				}
				engine.direction = DirectionRight
				engine.food = Point{X: 3, Y: 2}
				engine.rng = rand.New(rand.NewSource(77))

				return engine
			},
			expectedHead:     Point{X: 3, Y: 2},
			expectedLength:   2,
			expectedScore:    1,
			expectedTick:     1,
			expectedGameOver: false,
			verify: func(t *testing.T, engine *Engine, snap Snapshot) {
				occupiedAfterMove := map[Point]struct{}{
					{X: 3, Y: 2}: {},
					{X: 2, Y: 2}: {},
					{X: 0, Y: 0}: {},
				}

				expectedRng := rand.New(rand.NewSource(77))
				free := make([]Point, 0, engine.width*engine.height-len(occupiedAfterMove))
				for y := 0; y < engine.height; y++ {
					for x := 0; x < engine.width; x++ {
						point := Point{X: x, Y: y}
						if _, taken := occupiedAfterMove[point]; !taken {
							free = append(free, point)
						}
					}
				}
				expectedFood := free[expectedRng.Intn(len(free))]

				if snap.Food != expectedFood {
					t.Fatalf("expected deterministic food away from external occupancy at %+v, got %+v", expectedFood, snap.Food)
				}
				if _, ok := engine.occupied[Point{X: 0, Y: 0}]; !ok {
					t.Fatalf("expected external occupancy to remain after tick")
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			engine := tt.setup(t)

			if tt.queuedDirection != nil {
				if ok := engine.QueueDirection(*tt.queuedDirection); !ok {
					t.Fatalf("failed to queue direction %v", *tt.queuedDirection)
				}
			}

			engine.AdvanceTick()
			snap := engine.Snapshot()

			if snap.GameOver != tt.expectedGameOver {
				t.Fatalf("game over mismatch: expected %v, got %v", tt.expectedGameOver, snap.GameOver)
			}
			if snap.Tick != tt.expectedTick {
				t.Fatalf("tick mismatch: expected %d, got %d", tt.expectedTick, snap.Tick)
			}
			if len(snap.Snake) != tt.expectedLength {
				t.Fatalf("snake length mismatch: expected %d, got %d", tt.expectedLength, len(snap.Snake))
			}
			if snap.Snake[0] != tt.expectedHead {
				t.Fatalf("head position mismatch: expected %+v, got %+v", tt.expectedHead, snap.Snake[0])
			}
			if snap.Score != tt.expectedScore {
				t.Fatalf("score mismatch: expected %d, got %d", tt.expectedScore, snap.Score)
			}

			if tt.verify != nil {
				tt.verify(t, engine, snap)
			}
		})
	}
}

func TestAdvanceTickEndsGameWhenNoSpaceForFood(t *testing.T) {
	engine, err := NewEngine(2, 2, 21)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	engine.snake = []Point{{0, 1}, {0, 0}, {1, 0}}
	engine.occupied = map[Point]struct{}{
		{X: 0, Y: 1}: {},
		{X: 0, Y: 0}: {},
		{X: 1, Y: 0}: {},
	}
	engine.direction = DirectionRight
	engine.food = Point{X: 1, Y: 1}

	engine.AdvanceTick()

	snap := engine.Snapshot()

	if !snap.GameOver {
		t.Fatalf("expected game to end when no space remains for food")
	}
	if snap.Score != 0 {
		t.Fatalf("score should not increment when food cannot respawn, got %d", snap.Score)
	}
	if snap.Tick != 1 {
		t.Fatalf("tick should advance before detecting no free cells, got %d", snap.Tick)
	}
	if len(snap.Snake) != 4 {
		t.Fatalf("expected snake to fill the board, got length %d", len(snap.Snake))
	}
	if snap.Snake[0] != (Point{X: 1, Y: 1}) {
		t.Fatalf("expected head to move onto final food cell, got %+v", snap.Snake[0])
	}
}

func TestAdvanceTickNoOpAfterGameOver(t *testing.T) {
	engine, err := NewEngine(4, 4, 55)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	engine.gameOver = true
	engine.tick = 3
	engine.score = 2
	engine.direction = DirectionDown
	engine.pending = DirectionLeft
	engine.hasInput = true
	engine.food = Point{X: 1, Y: 1}

	before := engine.Snapshot()

	engine.AdvanceTick()
	after := engine.Snapshot()

	if !reflect.DeepEqual(before, after) {
		t.Fatalf("expected snapshot to remain unchanged after advancing while game over")
	}
	if engine.hasInput != true {
		t.Fatalf("queued input should remain untouched when game is over")
	}
	if engine.pending != DirectionLeft {
		t.Fatalf("pending direction should not be processed after game over, got %v", engine.pending)
	}
}

func TestAdvanceTickEndsGameWhenNoSpaceForFoodWithOtherSnakes(t *testing.T) {
	engine, err := NewEngine(2, 2, 23)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	engine.snake = []Point{{X: 0, Y: 0}}
	engine.occupied = map[Point]struct{}{
		{X: 0, Y: 0}: {},
		{X: 1, Y: 0}: {},
		{X: 1, Y: 1}: {},
	}
	engine.direction = DirectionDown
	engine.food = Point{X: 0, Y: 1}

	if queued := engine.QueueDirection(DirectionDown); !queued {
		t.Fatalf("expected to queue downward movement")
	}

	engine.AdvanceTick()
	snap := engine.Snapshot()

	if !snap.GameOver {
		t.Fatalf("expected game to end when no free cells remain for respawn")
	}
	if snap.Tick != 1 {
		t.Fatalf("tick should advance once before detecting the lack of space, got %d", snap.Tick)
	}
	if snap.Score != 0 {
		t.Fatalf("score should remain unchanged when food cannot respawn, got %d", snap.Score)
	}

	expectedSnake := []Point{{X: 0, Y: 1}, {X: 0, Y: 0}}
	if !reflect.DeepEqual(snap.Snake, expectedSnake) {
		t.Fatalf("expected snake to grow into final free cell, got %+v", snap.Snake)
	}

	for _, point := range []Point{{X: 1, Y: 0}, {X: 1, Y: 1}} {
		if _, ok := engine.occupied[point]; !ok {
			t.Fatalf("expected external occupancy at %+v to persist", point)
		}
	}
	if engine.QueueDirection(DirectionLeft) {
		t.Fatalf("should not accept further input after game over")
	}
}

func TestAdvanceTickStopsAfterGameOver(t *testing.T) {
	engine, err := NewEngine(2, 2, 31)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	start := engine.Snapshot()

	if ok := engine.QueueDirection(DirectionRight); !ok {
		t.Fatalf("expected direction to queue before collision")
	}
	engine.AdvanceTick()

	afterCollision := engine.Snapshot()
	if !afterCollision.GameOver {
		t.Fatalf("expected wall collision to end the game")
	}
	if afterCollision.Tick != start.Tick {
		t.Fatalf("tick should not advance after immediate collision, expected %d got %d", start.Tick, afterCollision.Tick)
	}
	if !reflect.DeepEqual(afterCollision.Snake, start.Snake) {
		t.Fatalf("snake position should not change on collision, expected %+v got %+v", start.Snake, afterCollision.Snake)
	}
	if engine.QueueDirection(DirectionDown) {
		t.Fatalf("should not accept inputs after game is over")
	}

	engine.AdvanceTick()
	final := engine.Snapshot()

	if !reflect.DeepEqual(final, afterCollision) {
		t.Fatalf("state should remain frozen after game over, got %+v", final)
	}
}

func TestAdvanceTickDetectsBoundaryCollisions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		width        int
		height       int
		steps        []Direction
		food         Point
		expectedHead Point
		expectedTick int
	}{
		{
			name:         "single column top boundary",
			width:        1,
			height:       3,
			steps:        []Direction{DirectionUp, DirectionUp},
			food:         Point{X: 0, Y: 2},
			expectedHead: Point{X: 0, Y: 0},
			expectedTick: 1,
		},
		{
			name:         "single row left boundary",
			width:        3,
			height:       1,
			steps:        []Direction{DirectionLeft, DirectionLeft},
			food:         Point{X: 2, Y: 0},
			expectedHead: Point{X: 0, Y: 0},
			expectedTick: 1,
		},
		{
			name:         "single column bottom boundary",
			width:        1,
			height:       3,
			steps:        []Direction{DirectionDown, DirectionDown},
			food:         Point{X: 0, Y: 0},
			expectedHead: Point{X: 0, Y: 2},
			expectedTick: 1,
		},
		{
			name:         "single row right boundary",
			width:        3,
			height:       1,
			steps:        []Direction{DirectionRight, DirectionRight},
			food:         Point{X: 0, Y: 0},
			expectedHead: Point{X: 2, Y: 0},
			expectedTick: 1,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			engine, err := NewEngine(tt.width, tt.height, 41)
			if err != nil {
				t.Fatalf("unexpected error creating engine: %v", err)
			}

			engine.food = tt.food

			for _, direction := range tt.steps {
				if ok := engine.QueueDirection(direction); !ok {
					t.Fatalf("failed to queue direction %v", direction)
				}
				engine.AdvanceTick()
			}

			snap := engine.Snapshot()

			if !snap.GameOver {
				t.Fatalf("expected boundary collision to end game")
			}
			if snap.Tick != tt.expectedTick {
				t.Fatalf("tick mismatch: expected %d, got %d", tt.expectedTick, snap.Tick)
			}
			if snap.Snake[0] != tt.expectedHead {
				t.Fatalf("head position mismatch: expected %+v, got %+v", tt.expectedHead, snap.Snake[0])
			}
			if snap.Score != 0 {
				t.Fatalf("score should remain zero when no food eaten, got %d", snap.Score)
			}
		})
	}
}

func TestAdvanceTickPreservesExternalOccupancy(t *testing.T) {
	engine, err := NewEngine(5, 5, 61)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	external := []Point{{X: 0, Y: 0}, {X: 4, Y: 4}}

	engine.snake = []Point{{X: 2, Y: 2}, {X: 2, Y: 3}}
	engine.occupied = map[Point]struct{}{
		{X: 2, Y: 2}: {},
		{X: 2, Y: 3}: {},
	}
	for _, point := range external {
		engine.occupied[point] = struct{}{}
	}
	engine.direction = DirectionRight
	engine.food = Point{X: 1, Y: 1}

	engine.AdvanceTick()

	snap := engine.Snapshot()

	for _, point := range external {
		if _, ok := engine.occupied[point]; !ok {
			t.Fatalf("expected external occupancy at %+v to persist after movement", point)
		}
	}
	if snap.GameOver {
		t.Fatalf("expected game to continue when not colliding with other snakes")
	}
	if snap.Tick != 1 {
		t.Fatalf("expected tick to advance, got %d", snap.Tick)
	}
	expectedSnake := []Point{{X: 3, Y: 2}, {X: 2, Y: 2}}
	if !reflect.DeepEqual(snap.Snake, expectedSnake) {
		t.Fatalf("expected snake to move right without growth, got %+v", snap.Snake)
	}
	if snap.Score != 0 {
		t.Fatalf("expected score to remain unchanged, got %d", snap.Score)
	}
}

func TestAdvanceTickSpawnsFoodWithOtherSnakesPresent(t *testing.T) {
	engine, err := NewEngine(3, 3, 51)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	otherSnakes := []Point{{X: 1, Y: 0}, {X: 2, Y: 0}, {X: 1, Y: 1}, {X: 2, Y: 1}, {X: 0, Y: 2}, {X: 1, Y: 2}}

	engine.snake = []Point{{X: 0, Y: 0}}
	engine.direction = DirectionDown
	engine.occupied = map[Point]struct{}{
		{X: 0, Y: 0}: {},
	}
	for _, point := range otherSnakes {
		engine.occupied[point] = struct{}{}
	}
	engine.food = Point{X: 0, Y: 1}

	if queued := engine.QueueDirection(DirectionDown); !queued {
		t.Fatalf("expected to queue direction before advancing")
	}

	engine.AdvanceTick()

	snap := engine.Snapshot()

	if snap.GameOver {
		t.Fatalf("unexpected game over while resolving food respawn near other snakes")
	}
	if len(snap.Snake) != 2 {
		t.Fatalf("expected snake to grow after eating food, got length %d", len(snap.Snake))
	}
	if snap.Score != 1 {
		t.Fatalf("expected score to increase after eating food, got %d", snap.Score)
	}
	if snap.Tick != 1 {
		t.Fatalf("expected tick to advance while growing, got %d", snap.Tick)
	}
	for _, point := range otherSnakes {
		if _, ok := engine.occupied[point]; !ok {
			t.Fatalf("expected other snake occupancy at %+v to persist after growth", point)
		}
	}

	expectedFood := Point{X: 2, Y: 2}
	if snap.Food != expectedFood {
		t.Fatalf("expected food to respawn at %+v, got %+v", expectedFood, snap.Food)
	}
	if _, blocked := engine.occupied[expectedFood]; blocked {
		t.Fatalf("food should spawn on free cell, but %+v is marked occupied", expectedFood)
	}
}

func TestAdvanceTickStateProgressionDeterministic(t *testing.T) {
	type expected struct {
		head      Point
		length    int
		score     int
		tick      int
		gameOver  bool
		food      Point
		direction Direction
	}

	tests := []struct {
		name          string
		setup         func(t *testing.T) *Engine
		steps         []Direction
		expected      expected
		expectedSnake []Point
		verify        func(t *testing.T, engine *Engine, snap Snapshot)
	}{
		{
			name: "two consecutive food pickups with external occupancy",
			setup: func(t *testing.T) *Engine {
				const seed = int64(123)

				engine, err := NewEngine(4, 4, seed)
				if err != nil {
					t.Fatalf("unexpected error creating engine: %v", err)
				}

				engine.snake = []Point{{X: 1, Y: 1}}
				engine.occupied = map[Point]struct{}{
					{X: 1, Y: 1}: {},
					{X: 0, Y: 0}: {}, // other player segment
				}
				engine.direction = DirectionRight
				engine.food = Point{X: 2, Y: 1}
				engine.rng = rand.New(rand.NewSource(seed))

				return engine
			},
			steps: []Direction{DirectionRight, DirectionRight},
			expected: expected{
				head:      Point{X: 3, Y: 1},
				length:    3,
				score:     2,
				tick:      2,
				gameOver:  false,
				food:      Point{X: 1, Y: 3},
				direction: DirectionRight,
			},
			expectedSnake: []Point{
				{X: 3, Y: 1},
				{X: 2, Y: 1},
				{X: 1, Y: 1},
			},
			verify: func(t *testing.T, engine *Engine, snap Snapshot) {
				for _, point := range []Point{{X: 0, Y: 0}, {X: 3, Y: 1}, {X: 2, Y: 1}, {X: 1, Y: 1}} {
					if _, ok := engine.occupied[point]; !ok {
						t.Fatalf("expected occupancy to retain %+v after progression", point)
					}
				}
				if snap.Food != (Point{X: 1, Y: 3}) {
					t.Fatalf("expected deterministic second respawn at %+v, got %+v", Point{X: 1, Y: 3}, snap.Food)
				}
			},
		},
		{
			name: "collision with other snake halts tick advancement",
			setup: func(t *testing.T) *Engine {
				engine, err := NewEngine(3, 3, 7)
				if err != nil {
					t.Fatalf("unexpected error creating engine: %v", err)
				}

				engine.snake = []Point{{X: 1, Y: 1}}
				engine.occupied = map[Point]struct{}{
					{X: 1, Y: 1}: {},
					{X: 2, Y: 1}: {},
				}
				engine.direction = DirectionRight
				engine.food = Point{X: 0, Y: 0}
				engine.rng = rand.New(rand.NewSource(7))

				return engine
			},
			steps: []Direction{DirectionRight},
			expected: expected{
				head:      Point{X: 1, Y: 1},
				length:    1,
				score:     0,
				tick:      0,
				gameOver:  true,
				food:      Point{X: 0, Y: 0},
				direction: DirectionRight,
			},
			expectedSnake: []Point{{X: 1, Y: 1}},
			verify: func(t *testing.T, engine *Engine, _ Snapshot) {
				if _, ok := engine.occupied[Point{X: 2, Y: 1}]; !ok {
					t.Fatalf("expected other snake occupancy to remain after collision")
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			engine := tt.setup(t)

			for _, dir := range tt.steps {
				if queued := engine.QueueDirection(dir); !queued {
					t.Fatalf("expected direction %v to queue", dir)
				}
				engine.AdvanceTick()
			}

			snap := engine.Snapshot()

			if snap.GameOver != tt.expected.gameOver {
				t.Fatalf("game over mismatch: expected %v, got %v", tt.expected.gameOver, snap.GameOver)
			}
			if snap.Tick != tt.expected.tick {
				t.Fatalf("tick mismatch: expected %d, got %d", tt.expected.tick, snap.Tick)
			}
			if snap.Score != tt.expected.score {
				t.Fatalf("score mismatch: expected %d, got %d", tt.expected.score, snap.Score)
			}
			if len(snap.Snake) != tt.expected.length {
				t.Fatalf("snake length mismatch: expected %d, got %d", tt.expected.length, len(snap.Snake))
			}
			if snap.Snake[0] != tt.expected.head {
				t.Fatalf("head position mismatch: expected %+v, got %+v", tt.expected.head, snap.Snake[0])
			}
			if snap.Food != tt.expected.food {
				t.Fatalf("food position mismatch: expected %+v, got %+v", tt.expected.food, snap.Food)
			}
			if snap.Direction != tt.expected.direction {
				t.Fatalf("direction mismatch: expected %v, got %v", tt.expected.direction, snap.Direction)
			}
			if tt.expectedSnake != nil && !reflect.DeepEqual(snap.Snake, tt.expectedSnake) {
				t.Fatalf("snake body mismatch: expected %+v, got %+v", tt.expectedSnake, snap.Snake)
			}

			if tt.verify != nil {
				tt.verify(t, engine, snap)
			}
		})
	}
}

func TestIsOpposite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		current   Direction
		candidate Direction
		opposite  bool
	}{
		{name: "up vs down", current: DirectionUp, candidate: DirectionDown, opposite: true},
		{name: "down vs up", current: DirectionDown, candidate: DirectionUp, opposite: true},
		{name: "left vs right", current: DirectionLeft, candidate: DirectionRight, opposite: true},
		{name: "right vs left", current: DirectionRight, candidate: DirectionLeft, opposite: true},
		{name: "same direction", current: DirectionUp, candidate: DirectionUp, opposite: false},
		{name: "non opposite turn", current: DirectionUp, candidate: DirectionRight, opposite: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isOpposite(tt.current, tt.candidate); got != tt.opposite {
				t.Fatalf("isOpposite(%v, %v) = %v, want %v", tt.current, tt.candidate, got, tt.opposite)
			}
		})
	}
}
