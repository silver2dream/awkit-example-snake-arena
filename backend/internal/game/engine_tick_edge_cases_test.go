package game

import (
	"math/rand"
	"reflect"
	"testing"
)

func TestAdvanceTickBoundaryCollisions(t *testing.T) {
	t.Parallel()

	clone := func(src map[Point]struct{}) map[Point]struct{} {
		dst := make(map[Point]struct{}, len(src))
		for point := range src {
			dst[point] = struct{}{}
		}
		return dst
	}

	type expectation struct {
		head      Point
		direction Direction
	}

	tests := []struct {
		name   string
		width  int
		height int
		seed   int64
		queue  *Direction
		setup  func(engine *Engine)
		expect func(engine *Engine) expectation
	}{
		{
			name:   "single column immediate wall collision",
			width:  1,
			height: 3,
			seed:   303,
			setup: func(engine *Engine) {
				engine.food = Point{X: 0, Y: 0}
			},
			expect: func(engine *Engine) expectation {
				return expectation{
					head:      engine.snake[0],
					direction: DirectionRight,
				}
			},
		},
		{
			name:   "single row queued turn collides vertically",
			width:  4,
			height: 1,
			seed:   404,
			queue:  ptr(DirectionUp),
			setup: func(engine *Engine) {
				engine.food = Point{X: 0, Y: 0}
			},
			expect: func(engine *Engine) expectation {
				return expectation{
					head:      engine.snake[0],
					direction: DirectionUp,
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			engine := mustEngine(t, tt.width, tt.height, tt.seed)
			if tt.setup != nil {
				tt.setup(engine)
			}

			beforeOccupied := clone(engine.occupied)
			startTick := engine.tick

			if tt.queue != nil {
				if ok := engine.QueueDirection(*tt.queue); !ok {
					t.Fatalf("failed to queue direction %v", *tt.queue)
				}
			}

			expected := tt.expect(engine)

			engine.AdvanceTick()
			snap := engine.Snapshot()

			if !snap.GameOver {
				t.Fatalf("expected collision to end the game")
			}
			if snap.Tick != startTick {
				t.Fatalf("tick should not advance after immediate collision, expected %d got %d", startTick, snap.Tick)
			}
			if snap.Score != 0 {
				t.Fatalf("score should remain zero on collision, got %d", snap.Score)
			}
			if len(snap.Snake) != 1 {
				t.Fatalf("expected snake length to remain 1 after collision, got %d", len(snap.Snake))
			}
			if snap.Snake[0] != expected.head {
				t.Fatalf("head mismatch: expected %+v, got %+v", expected.head, snap.Snake[0])
			}
			if snap.Direction != expected.direction {
				t.Fatalf("direction mismatch: expected %v, got %v", expected.direction, snap.Direction)
			}
			if engine.hasInput {
				t.Fatalf("queued input flag should clear after processing collision")
			}
			if !reflect.DeepEqual(engine.occupied, beforeOccupied) {
				t.Fatalf("expected occupied cells to remain unchanged, before %+v after %+v", beforeOccupied, engine.occupied)
			}
		})
	}
}

func TestAdvanceTickDeterministicRespawnWithExternalOccupancy(t *testing.T) {
	t.Parallel()

	clone := func(src map[Point]struct{}) map[Point]struct{} {
		dst := make(map[Point]struct{}, len(src))
		for point := range src {
			dst[point] = struct{}{}
		}
		return dst
	}

	type scenario struct {
		name      string
		width     int
		height    int
		seed      int64
		snake     []Point
		external  []Point
		direction Direction
		food      Point
	}

	tests := []scenario{
		{
			name:      "growth respawns food away from other snakes",
			width:     4,
			height:    3,
			seed:      808,
			snake:     []Point{{X: 1, Y: 1}},
			external:  []Point{{X: 0, Y: 0}, {X: 3, Y: 2}},
			direction: DirectionRight,
			food:      Point{X: 2, Y: 1},
		},
		{
			name:      "crowded grid still spawns deterministically",
			width:     3,
			height:    3,
			seed:      909,
			snake:     []Point{{X: 1, Y: 1}},
			external:  []Point{{X: 0, Y: 0}, {X: 0, Y: 1}, {X: 2, Y: 1}, {X: 2, Y: 2}},
			direction: DirectionDown,
			food:      Point{X: 1, Y: 2},
		},
	}

	makeEngine := func(t *testing.T, sc scenario) *Engine {
		t.Helper()

		engine := mustEngine(t, sc.width, sc.height, sc.seed)
		engine.snake = append([]Point(nil), sc.snake...)
		engine.occupied = map[Point]struct{}{}
		for _, point := range sc.snake {
			engine.occupied[point] = struct{}{}
		}
		for _, point := range sc.external {
			engine.occupied[point] = struct{}{}
		}
		engine.direction = sc.direction
		engine.food = sc.food
		engine.rng = rand.New(rand.NewSource(sc.seed))

		return engine
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var snapshots []Snapshot

			for i := 0; i < 2; i++ {
				engine := makeEngine(t, tt)
				startSnake := append([]Point(nil), engine.snake...)
				beforeOccupied := clone(engine.occupied)

				expectedOccupied := clone(beforeOccupied)
				expectedOccupied[tt.food] = struct{}{}
				expectedFood := deterministicEmptyCell(tt.width, tt.height, expectedOccupied, tt.seed)
				expectedSnake := append([]Point{tt.food}, startSnake...)

				engine.AdvanceTick()
				snap := engine.Snapshot()

				if snap.GameOver {
					t.Fatalf("did not expect game over after eating food")
				}
				if snap.Tick != 1 {
					t.Fatalf("tick should advance to 1 after movement, got %d", snap.Tick)
				}
				if snap.Score != 1 {
					t.Fatalf("score should increment after eating, got %d", snap.Score)
				}
				if len(snap.Snake) != len(expectedSnake) {
					t.Fatalf("snake length mismatch: expected %d, got %d", len(expectedSnake), len(snap.Snake))
				}
				if !reflect.DeepEqual(snap.Snake, expectedSnake) {
					t.Fatalf("snake body mismatch: expected %+v, got %+v", expectedSnake, snap.Snake)
				}
				if snap.Snake[0] != tt.food {
					t.Fatalf("head mismatch: expected %+v, got %+v", tt.food, snap.Snake[0])
				}
				if snap.Direction != tt.direction {
					t.Fatalf("direction mismatch: expected %v, got %v", tt.direction, snap.Direction)
				}
				if snap.Food != expectedFood {
					t.Fatalf("food respawn mismatch: expected %+v, got %+v", expectedFood, snap.Food)
				}
				if engine.hasInput {
					t.Fatalf("queued input should clear after processing movement")
				}
				if len(engine.occupied) != len(expectedOccupied) {
					t.Fatalf("occupied cells mismatch: expected %d, got %d", len(expectedOccupied), len(engine.occupied))
				}
				for point := range expectedOccupied {
					if _, ok := engine.occupied[point]; !ok {
						t.Fatalf("expected occupancy at %+v to persist after growth", point)
					}
				}
				if _, blocked := engine.occupied[snap.Food]; blocked {
					t.Fatalf("food should not respawn on occupied cell %+v", snap.Food)
				}

				snapshots = append(snapshots, snap)
			}

			if !reflect.DeepEqual(snapshots[0], snapshots[1]) {
				t.Fatalf("expected deterministic snapshots across identical runs, got %+v and %+v", snapshots[0], snapshots[1])
			}
		})
	}
}

func TestAdvanceTickProcessesQueuedSameDirection(t *testing.T) {
	t.Parallel()

	engine := mustEngine(t, 5, 4, 4242)
	engine.food = Point{X: 0, Y: 0}

	start := engine.Snapshot()
	startHead := start.Snake[0]

	if ok := engine.QueueDirection(DirectionRight); !ok {
		t.Fatalf("expected to queue same direction input")
	}
	if !engine.hasInput {
		t.Fatalf("queued input flag should be set before advancing tick")
	}

	engine.AdvanceTick()
	snap := engine.Snapshot()

	expectedHead := Point{X: startHead.X + 1, Y: startHead.Y}

	if snap.GameOver {
		t.Fatalf("game should continue when moving straight without collisions")
	}
	if snap.Tick != start.Tick+1 {
		t.Fatalf("tick should advance by one, expected %d got %d", start.Tick+1, snap.Tick)
	}
	if snap.Score != start.Score {
		t.Fatalf("score should remain unchanged without eating, expected %d got %d", start.Score, snap.Score)
	}
	if snap.Direction != DirectionRight {
		t.Fatalf("direction should remain right after processing queued input, got %v", snap.Direction)
	}
	if engine.hasInput {
		t.Fatalf("queued input flag should clear after applying direction")
	}
	if len(snap.Snake) != len(start.Snake) {
		t.Fatalf("snake length should remain unchanged, expected %d got %d", len(start.Snake), len(snap.Snake))
	}
	if snap.Snake[0] != expectedHead {
		t.Fatalf("head position mismatch: expected %+v, got %+v", expectedHead, snap.Snake[0])
	}
	if _, ok := engine.occupied[expectedHead]; !ok {
		t.Fatalf("expected occupancy to include new head %+v", expectedHead)
	}
	if _, ok := engine.occupied[startHead]; ok {
		t.Fatalf("expected previous head %+v to be cleared from occupancy", startHead)
	}
}
