package game

import (
	"math/rand"
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

func TestDeterministicProgressionThroughGrowthAndCollision(t *testing.T) {
	t.Parallel()

	const (
		width  = 4
		height = 3
		seed   = int64(121)
	)

	buildEngine := func(t *testing.T) *Engine {
		t.Helper()

		engine := mustEngine(t, width, height, seed)
		engine.snake = []Point{{X: 1, Y: 1}, {X: 1, Y: 2}}
		engine.occupied = map[Point]struct{}{
			{X: 1, Y: 1}: {},
			{X: 1, Y: 2}: {},
			{X: 0, Y: 0}: {},
		}
		engine.direction = DirectionRight
		engine.food = Point{X: 2, Y: 1}
		engine.rng = rand.New(rand.NewSource(seed))

		return engine
	}

	occupiedAfterGrowth := map[Point]struct{}{
		{X: 2, Y: 1}: {},
		{X: 1, Y: 1}: {},
		{X: 1, Y: 2}: {},
		{X: 0, Y: 0}: {},
	}
	expectedFood := deterministicEmptyCell(width, height, occupiedAfterGrowth, seed)

	steps := []Direction{DirectionRight, DirectionUp, DirectionUp}

	first := buildEngine(t)
	second := buildEngine(t)

	for _, dir := range steps {
		for _, engine := range []*Engine{first, second} {
			if ok := engine.QueueDirection(dir); !ok && !engine.gameOver {
				t.Fatalf("expected to queue direction %v before collision", dir)
			}
			engine.AdvanceTick()
		}
	}

	expectedSnake := []Point{
		{X: 2, Y: 0},
		{X: 2, Y: 1},
		{X: 1, Y: 1},
	}
	expectedOccupied := map[Point]struct{}{
		{X: 2, Y: 0}: {},
		{X: 2, Y: 1}: {},
		{X: 1, Y: 1}: {},
		{X: 0, Y: 0}: {},
	}
	expectedSnapshot := Snapshot{
		Width:     width,
		Height:    height,
		Snake:     expectedSnake,
		Direction: DirectionUp,
		Food:      expectedFood,
		Tick:      2,
		GameOver:  true,
		Score:     1,
	}

	for _, engine := range []*Engine{first, second} {
		snap := engine.Snapshot()

		if !reflect.DeepEqual(snap, expectedSnapshot) {
			t.Fatalf("expected snapshot %+v, got %+v", expectedSnapshot, snap)
		}
		if !reflect.DeepEqual(engine.occupied, expectedOccupied) {
			t.Fatalf("expected occupancy %+v, got %+v", expectedOccupied, engine.occupied)
		}
		if engine.hasInput {
			t.Fatalf("queued input should clear after processing collision")
		}
	}

	if !reflect.DeepEqual(first.Snapshot(), second.Snapshot()) {
		t.Fatalf("expected deterministic snapshots across identical runs")
	}
}
