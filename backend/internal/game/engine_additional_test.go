package game

import (
	"math/rand"
	"reflect"
	"testing"
)

func TestAdvanceTickCollisionAndStateProgression(t *testing.T) {
	t.Parallel()

	type expectation struct {
		snake     []Point
		head      Point
		score     int
		tick      int
		gameOver  bool
		direction Direction
		hasInput  bool
		food      Point
		occupied  map[Point]struct{}
	}

	growthOccupied := map[Point]struct{}{
		{X: 2, Y: 1}: {},
		{X: 1, Y: 1}: {},
		{X: 0, Y: 0}: {},
	}
	expectedGrowthFood := deterministicEmptyCell(4, 3, growthOccupied, 33)

	tests := []struct {
		name      string
		width     int
		height    int
		seed      int64
		snake     []Point
		external  []Point
		direction Direction
		food      Point
		queued    *Direction
		expect    expectation
	}{
		{
			name:      "wall collision after queued turn",
			width:     3,
			height:    2,
			seed:      5,
			snake:     []Point{{X: 0, Y: 0}},
			external:  []Point{{X: 2, Y: 1}},
			direction: DirectionRight,
			food:      Point{X: 1, Y: 1},
			queued:    ptr(DirectionLeft),
			expect: expectation{
				snake:     []Point{{X: 0, Y: 0}},
				head:      Point{X: 0, Y: 0},
				score:     0,
				tick:      0,
				gameOver:  true,
				direction: DirectionLeft,
				hasInput:  false,
				food:      Point{X: 1, Y: 1},
				occupied: map[Point]struct{}{
					{X: 0, Y: 0}: {},
					{X: 2, Y: 1}: {},
				},
			},
		},
		{
			name:      "self collision preserves board occupancy",
			width:     4,
			height:    4,
			seed:      7,
			snake:     []Point{{X: 1, Y: 1}, {X: 1, Y: 2}, {X: 1, Y: 3}},
			external:  []Point{{X: 0, Y: 0}},
			direction: DirectionDown,
			food:      Point{X: 3, Y: 3},
			expect: expectation{
				snake:     []Point{{X: 1, Y: 1}, {X: 1, Y: 2}, {X: 1, Y: 3}},
				head:      Point{X: 1, Y: 1},
				score:     0,
				tick:      0,
				gameOver:  true,
				direction: DirectionDown,
				hasInput:  false,
				food:      Point{X: 3, Y: 3},
				occupied: map[Point]struct{}{
					{X: 1, Y: 1}: {},
					{X: 1, Y: 2}: {},
					{X: 1, Y: 3}: {},
					{X: 0, Y: 0}: {},
				},
			},
		},
		{
			name:      "collision with other snake halts progression",
			width:     4,
			height:    3,
			seed:      11,
			snake:     []Point{{X: 1, Y: 1}},
			external:  []Point{{X: 2, Y: 1}, {X: 0, Y: 2}},
			direction: DirectionRight,
			food:      Point{X: 0, Y: 0},
			expect: expectation{
				snake:     []Point{{X: 1, Y: 1}},
				head:      Point{X: 1, Y: 1},
				score:     0,
				tick:      0,
				gameOver:  true,
				direction: DirectionRight,
				hasInput:  false,
				food:      Point{X: 0, Y: 0},
				occupied: map[Point]struct{}{
					{X: 1, Y: 1}: {},
					{X: 2, Y: 1}: {},
					{X: 0, Y: 2}: {},
				},
			},
		},
		{
			name:      "eats food, grows, and respawns deterministically",
			width:     4,
			height:    3,
			seed:      33,
			snake:     []Point{{X: 1, Y: 1}},
			external:  []Point{{X: 0, Y: 0}},
			direction: DirectionRight,
			food:      Point{X: 2, Y: 1},
			expect: expectation{
				snake:     []Point{{X: 2, Y: 1}, {X: 1, Y: 1}},
				head:      Point{X: 2, Y: 1},
				score:     1,
				tick:      1,
				gameOver:  false,
				direction: DirectionRight,
				hasInput:  false,
				food:      expectedGrowthFood,
				occupied: map[Point]struct{}{
					{X: 2, Y: 1}: {},
					{X: 1, Y: 1}: {},
					{X: 0, Y: 0}: {},
				},
			},
		},
		{
			name:      "moves forward without food and trims tail",
			width:     5,
			height:    4,
			seed:      17,
			snake:     []Point{{X: 2, Y: 1}, {X: 2, Y: 2}},
			external:  []Point{{X: 0, Y: 0}},
			direction: DirectionUp,
			food:      Point{X: 4, Y: 3},
			expect: expectation{
				snake:     []Point{{X: 2, Y: 0}, {X: 2, Y: 1}},
				head:      Point{X: 2, Y: 0},
				score:     0,
				tick:      1,
				gameOver:  false,
				direction: DirectionUp,
				hasInput:  false,
				food:      Point{X: 4, Y: 3},
				occupied: map[Point]struct{}{
					{X: 2, Y: 0}: {},
					{X: 2, Y: 1}: {},
					{X: 0, Y: 0}: {},
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			engine := mustEngine(t, tt.width, tt.height, tt.seed)
			engine.snake = append([]Point(nil), tt.snake...)
			engine.occupied = map[Point]struct{}{}
			for _, point := range tt.snake {
				engine.occupied[point] = struct{}{}
			}
			for _, point := range tt.external {
				engine.occupied[point] = struct{}{}
			}
			engine.direction = tt.direction
			engine.food = tt.food
			engine.rng = rand.New(rand.NewSource(tt.seed))

			if tt.queued != nil {
				engine.pending = *tt.queued
				engine.hasInput = true
			}

			engine.AdvanceTick()
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
			if snap.Snake[0] != tt.expect.head {
				t.Fatalf("head mismatch: expected %+v, got %+v", tt.expect.head, snap.Snake[0])
			}
			if !reflect.DeepEqual(snap.Snake, tt.expect.snake) {
				t.Fatalf("snake body mismatch: expected %+v, got %+v", tt.expect.snake, snap.Snake)
			}
			if snap.Food != tt.expect.food {
				t.Fatalf("food mismatch: expected %+v, got %+v", tt.expect.food, snap.Food)
			}
			if engine.hasInput != tt.expect.hasInput {
				t.Fatalf("hasInput mismatch: expected %v, got %v", tt.expect.hasInput, engine.hasInput)
			}

			if len(engine.occupied) != len(tt.expect.occupied) {
				t.Fatalf("occupied count mismatch: expected %d, got %d", len(tt.expect.occupied), len(engine.occupied))
			}
			for point := range tt.expect.occupied {
				if _, ok := engine.occupied[point]; !ok {
					t.Fatalf("expected occupancy at %+v to remain set", point)
				}
			}
		})
	}
}

func TestCollisionDetectionCoverage(t *testing.T) {
	t.Parallel()

	type expectation struct {
		head     Point
		length   int
		score    int
		tick     int
		gameOver bool
	}

	tests := []struct {
		name   string
		setup  func(t *testing.T) *Engine
		expect expectation
		verify func(t *testing.T, engine *Engine, snap Snapshot)
	}{
		{
			name: "wall collision stops progression",
			setup: func(t *testing.T) *Engine {
				engine := mustEngine(t, 2, 2, 1)
				engine.snake = []Point{{X: 0, Y: 0}}
				engine.occupied = map[Point]struct{}{
					{X: 0, Y: 0}: {},
				}
				engine.direction = DirectionLeft
				engine.food = Point{X: 1, Y: 1}
				return engine
			},
			expect: expectation{
				head:     Point{X: 0, Y: 0},
				length:   1,
				score:    0,
				tick:     0,
				gameOver: true,
			},
		},
		{
			name: "self collision with existing body segment",
			setup: func(t *testing.T) *Engine {
				engine := mustEngine(t, 4, 4, 2)
				engine.snake = []Point{{X: 1, Y: 1}, {X: 1, Y: 2}, {X: 1, Y: 3}}
				engine.occupied = map[Point]struct{}{
					{X: 1, Y: 1}: {},
					{X: 1, Y: 2}: {},
					{X: 1, Y: 3}: {},
				}
				engine.direction = DirectionDown
				engine.food = Point{X: 3, Y: 3}
				return engine
			},
			expect: expectation{
				head:     Point{X: 1, Y: 1},
				length:   3,
				score:    0,
				tick:     0,
				gameOver: true,
			},
		},
		{
			name: "food consumption grows snake and respawns deterministically",
			setup: func(t *testing.T) *Engine {
				const seed = int64(33)
				engine := mustEngine(t, 4, 3, seed)
				engine.snake = []Point{{X: 1, Y: 1}}
				engine.occupied = map[Point]struct{}{
					{X: 1, Y: 1}: {},
				}
				engine.direction = DirectionRight
				engine.food = Point{X: 2, Y: 1}
				engine.rng = rand.New(rand.NewSource(seed))
				return engine
			},
			expect: expectation{
				head:     Point{X: 2, Y: 1},
				length:   2,
				score:    1,
				tick:     1,
				gameOver: false,
			},
			verify: func(t *testing.T, engine *Engine, snap Snapshot) {
				occupiedAfterGrowth := map[Point]struct{}{
					{X: 2, Y: 1}: {},
					{X: 1, Y: 1}: {},
				}
				expectedFood := deterministicEmptyCell(engine.width, engine.height, occupiedAfterGrowth, 33)
				if snap.Food != expectedFood {
					t.Fatalf("expected deterministic food at %+v, got %+v", expectedFood, snap.Food)
				}
				if _, blocked := engine.occupied[snap.Food]; blocked {
					t.Fatalf("food should respawn on a free cell, got occupied %+v", snap.Food)
				}
			},
		},
		{
			name: "collision with other snake occupancy",
			setup: func(t *testing.T) *Engine {
				engine := mustEngine(t, 4, 4, 5)
				engine.snake = []Point{{X: 1, Y: 1}}
				engine.occupied = map[Point]struct{}{
					{X: 1, Y: 1}: {},
					{X: 2, Y: 1}: {}, // other snake segment
				}
				engine.direction = DirectionRight
				engine.food = Point{X: 0, Y: 0}
				return engine
			},
			expect: expectation{
				head:     Point{X: 1, Y: 1},
				length:   1,
				score:    0,
				tick:     0,
				gameOver: true,
			},
			verify: func(t *testing.T, engine *Engine, _ Snapshot) {
				if _, ok := engine.occupied[Point{X: 2, Y: 1}]; !ok {
					t.Fatalf("expected external occupancy to remain after collision")
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			engine := tt.setup(t)
			engine.AdvanceTick()
			snap := engine.Snapshot()

			if snap.GameOver != tt.expect.gameOver {
				t.Fatalf("game over mismatch: expected %v, got %v", tt.expect.gameOver, snap.GameOver)
			}
			if snap.Tick != tt.expect.tick {
				t.Fatalf("tick mismatch: expected %d, got %d", tt.expect.tick, snap.Tick)
			}
			if len(snap.Snake) != tt.expect.length {
				t.Fatalf("length mismatch: expected %d, got %d", tt.expect.length, len(snap.Snake))
			}
			if snap.Snake[0] != tt.expect.head {
				t.Fatalf("head mismatch: expected %+v, got %+v", tt.expect.head, snap.Snake[0])
			}
			if snap.Score != tt.expect.score {
				t.Fatalf("score mismatch: expected %d, got %d", tt.expect.score, snap.Score)
			}

			if tt.verify != nil {
				tt.verify(t, engine, snap)
			}
		})
	}
}

func TestAdvanceTickStateTransitions(t *testing.T) {
	t.Parallel()

	type expectation struct {
		head      Point
		length    int
		score     int
		tick      int
		gameOver  bool
		direction Direction
		hasInput  bool
		pending   Direction
	}

	tests := []struct {
		name    string
		setup   func(t *testing.T) *Engine
		queue   *Direction
		advance int
		expect  expectation
		verify  func(t *testing.T, before Snapshot, after Snapshot)
	}{
		{
			name: "moves forward without queued input",
			setup: func(t *testing.T) *Engine {
				engine := mustEngine(t, 4, 4, 9)
				engine.snake = []Point{{X: 1, Y: 1}}
				engine.occupied = map[Point]struct{}{
					{X: 1, Y: 1}: {},
				}
				engine.direction = DirectionRight
				engine.food = Point{X: 3, Y: 3}
				return engine
			},
			advance: 1,
			expect: expectation{
				head:      Point{X: 2, Y: 1},
				length:    1,
				score:     0,
				tick:      1,
				gameOver:  false,
				direction: DirectionRight,
			},
		},
		{
			name: "applies queued turn before movement",
			setup: func(t *testing.T) *Engine {
				engine := mustEngine(t, 5, 5, 11)
				engine.snake = []Point{{X: 2, Y: 2}}
				engine.occupied = map[Point]struct{}{
					{X: 2, Y: 2}: {},
				}
				engine.direction = DirectionRight
				engine.food = Point{X: 4, Y: 4}
				return engine
			},
			queue:   ptr(DirectionUp),
			advance: 1,
			expect: expectation{
				head:      Point{X: 2, Y: 1},
				length:    1,
				score:     0,
				tick:      1,
				gameOver:  false,
				direction: DirectionUp,
			},
		},
		{
			name: "halts progression when already game over",
			setup: func(t *testing.T) *Engine {
				engine := mustEngine(t, 3, 3, 7)
				engine.snake = []Point{{X: 1, Y: 1}}
				engine.occupied = map[Point]struct{}{
					{X: 1, Y: 1}: {},
				}
				engine.direction = DirectionDown
				engine.pending = DirectionLeft
				engine.hasInput = true
				engine.gameOver = true
				engine.tick = 4
				engine.score = 2
				engine.food = Point{X: 0, Y: 0}
				return engine
			},
			advance: 1,
			expect: expectation{
				head:      Point{X: 1, Y: 1},
				length:    1,
				score:     2,
				tick:      4,
				gameOver:  true,
				direction: DirectionDown,
				hasInput:  true,
				pending:   DirectionLeft,
			},
			verify: func(t *testing.T, before Snapshot, after Snapshot) {
				if !reflect.DeepEqual(before, after) {
					t.Fatalf("expected snapshot to remain unchanged after game over, before %+v after %+v", before, after)
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
					t.Fatalf("failed to queue direction %v", *tt.queue)
				}
			}

			for i := 0; i < tt.advance; i++ {
				engine.AdvanceTick()
			}

			after := engine.Snapshot()

			if after.GameOver != tt.expect.gameOver {
				t.Fatalf("game over mismatch: expected %v, got %v", tt.expect.gameOver, after.GameOver)
			}
			if after.Tick != tt.expect.tick {
				t.Fatalf("tick mismatch: expected %d, got %d", tt.expect.tick, after.Tick)
			}
			if len(after.Snake) != tt.expect.length {
				t.Fatalf("length mismatch: expected %d, got %d", tt.expect.length, len(after.Snake))
			}
			if after.Snake[0] != tt.expect.head {
				t.Fatalf("head mismatch: expected %+v, got %+v", tt.expect.head, after.Snake[0])
			}
			if after.Score != tt.expect.score {
				t.Fatalf("score mismatch: expected %d, got %d", tt.expect.score, after.Score)
			}
			if after.Direction != tt.expect.direction {
				t.Fatalf("direction mismatch: expected %v, got %v", tt.expect.direction, after.Direction)
			}
			if tt.expect.hasInput != engine.hasInput {
				t.Fatalf("hasInput mismatch: expected %v, got %v", tt.expect.hasInput, engine.hasInput)
			}
			if tt.expect.pending != 0 {
				if engine.pending != tt.expect.pending {
					t.Fatalf("pending direction mismatch: expected %v, got %v", tt.expect.pending, engine.pending)
				}
			}

			if tt.verify != nil {
				tt.verify(t, before, after)
			}
		})
	}
}

func TestEdgeCaseConstraints(t *testing.T) {
	t.Parallel()

	t.Run("empty grid rejected", func(t *testing.T) {
		engine, err := NewEngine(0, 0, 1)
		if err == nil {
			t.Fatalf("expected error creating engine with empty grid")
		}
		if engine != nil {
			t.Fatalf("engine should be nil on invalid dimensions")
		}
	})

	t.Run("full board with other players leaves no space for food", func(t *testing.T) {
		engine := mustEngine(t, 2, 2, 21)
		engine.snake = []Point{{X: 0, Y: 0}}
		engine.occupied = map[Point]struct{}{
			{X: 0, Y: 0}: {},
			{X: 1, Y: 0}: {}, // other snake
			{X: 1, Y: 1}: {}, // other snake
		}
		engine.direction = DirectionDown
		engine.food = Point{X: 0, Y: 1}

		engine.AdvanceTick()
		snap := engine.Snapshot()

		if !snap.GameOver {
			t.Fatalf("expected game to end when food cannot respawn")
		}
		if snap.Tick != 1 {
			t.Fatalf("tick should advance once before detecting no space, got %d", snap.Tick)
		}
		if snap.Score != 0 {
			t.Fatalf("score should remain zero when food respawn fails, got %d", snap.Score)
		}
		if len(snap.Snake) != 2 {
			t.Fatalf("expected snake to grow into last free cell, got length %d", len(snap.Snake))
		}
		if snap.Snake[0] != (Point{X: 0, Y: 1}) {
			t.Fatalf("expected head to move onto final food cell, got %+v", snap.Snake[0])
		}

		for _, point := range []Point{{X: 1, Y: 0}, {X: 1, Y: 1}} {
			if _, ok := engine.occupied[point]; !ok {
				t.Fatalf("expected external occupancy at %+v to persist", point)
			}
		}
	})
}

func mustEngine(t *testing.T, width, height int, seed int64) *Engine {
	t.Helper()
	engine, err := NewEngine(width, height, seed)
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}
	return engine
}

func deterministicEmptyCell(width, height int, occupied map[Point]struct{}, seed int64) Point {
	rng := rand.New(rand.NewSource(seed))
	free := make([]Point, 0, width*height-len(occupied))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			point := Point{X: x, Y: y}
			if _, taken := occupied[point]; !taken {
				free = append(free, point)
			}
		}
	}

	return free[rng.Intn(len(free))]
}

func ptr(direction Direction) *Direction {
	return &direction
}
