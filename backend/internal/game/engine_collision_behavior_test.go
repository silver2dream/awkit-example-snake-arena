package game

import (
	"errors"
	"math/rand"
	"reflect"
	"testing"
)

func TestQueueDirectionPreservesPendingOnRejectedInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		initialPending  Direction
		initialHasInput bool
		gameOver        bool
		attempted       Direction
	}{
		{
			name:            "invalid direction leaves queued input intact",
			initialPending:  DirectionUp,
			initialHasInput: true,
			attempted:       Direction(-99),
		},
		{
			name:            "game over prevents overriding pending input",
			initialPending:  DirectionLeft,
			initialHasInput: true,
			gameOver:        true,
			attempted:       DirectionDown,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			engine := mustEngine(t, 4, 4, 101)
			engine.pending = tt.initialPending
			engine.hasInput = tt.initialHasInput
			engine.direction = DirectionRight
			engine.gameOver = tt.gameOver

			if queued := engine.QueueDirection(tt.attempted); queued {
				t.Fatalf("expected queue to be rejected for %v", tt.attempted)
			}
			if engine.pending != tt.initialPending {
				t.Fatalf("pending direction mutated: expected %v, got %v", tt.initialPending, engine.pending)
			}
			if engine.hasInput != tt.initialHasInput {
				t.Fatalf("hasInput mutated: expected %v, got %v", tt.initialHasInput, engine.hasInput)
			}
			if engine.direction != DirectionRight {
				t.Fatalf("direction should remain unchanged after rejected input, got %v", engine.direction)
			}
		})
	}
}

func TestAdvanceTickClearsQueuedInputAfterCollision(t *testing.T) {
	t.Parallel()

	type expected struct {
		head      Point
		length    int
		direction Direction
	}

	tests := []struct {
		name   string
		setup  func(t *testing.T) *Engine
		queued Direction
		expect expected
		verify func(t *testing.T, engine *Engine)
	}{
		{
			name: "wall collision keeps state but clears input",
			setup: func(t *testing.T) *Engine {
				engine := mustEngine(t, 2, 2, 5)
				engine.snake = []Point{{X: 0, Y: 0}}
				engine.occupied = map[Point]struct{}{
					{X: 0, Y: 0}: {},
				}
				engine.direction = DirectionRight
				engine.food = Point{X: 1, Y: 1}
				return engine
			},
			queued: DirectionLeft,
			expect: expected{
				head:      Point{X: 0, Y: 0},
				length:    1,
				direction: DirectionLeft,
			},
		},
		{
			name: "self collision leaves body unchanged",
			setup: func(t *testing.T) *Engine {
				engine := mustEngine(t, 4, 4, 7)
				engine.snake = []Point{{X: 1, Y: 1}, {X: 1, Y: 2}}
				engine.occupied = map[Point]struct{}{
					{X: 1, Y: 1}: {},
					{X: 1, Y: 2}: {},
				}
				engine.direction = DirectionUp
				engine.food = Point{X: 3, Y: 3}
				return engine
			},
			queued: DirectionDown,
			expect: expected{
				head:      Point{X: 1, Y: 1},
				length:    2,
				direction: DirectionDown,
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
				engine.direction = DirectionUp
				engine.food = Point{X: 0, Y: 0}
				return engine
			},
			queued: DirectionRight,
			expect: expected{
				head:      Point{X: 1, Y: 1},
				length:    1,
				direction: DirectionRight,
			},
			verify: func(t *testing.T, engine *Engine) {
				if _, ok := engine.occupied[Point{X: 2, Y: 1}]; !ok {
					t.Fatalf("expected other snake occupancy to persist")
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

			if ok := engine.QueueDirection(tt.queued); !ok {
				t.Fatalf("expected to queue direction %v", tt.queued)
			}

			engine.AdvanceTick()
			snap := engine.Snapshot()

			if !snap.GameOver {
				t.Fatalf("expected collision to end the game")
			}
			if snap.Tick != before.Tick {
				t.Fatalf("tick should not advance on immediate collision, expected %d got %d", before.Tick, snap.Tick)
			}
			if snap.Score != before.Score {
				t.Fatalf("score should remain unchanged, expected %d got %d", before.Score, snap.Score)
			}
			if len(snap.Snake) != tt.expect.length {
				t.Fatalf("length mismatch: expected %d, got %d", tt.expect.length, len(snap.Snake))
			}
			if snap.Snake[0] != tt.expect.head {
				t.Fatalf("head mismatch: expected %+v, got %+v", tt.expect.head, snap.Snake[0])
			}
			if !reflect.DeepEqual(snap.Snake, before.Snake) {
				t.Fatalf("snake body should remain unchanged after collision, before %+v after %+v", before.Snake, snap.Snake)
			}
			if snap.Direction != tt.expect.direction {
				t.Fatalf("direction mismatch: expected %v, got %v", tt.expect.direction, snap.Direction)
			}
			if engine.hasInput {
				t.Fatalf("queued input should clear after processing collision")
			}

			if tt.verify != nil {
				tt.verify(t, engine)
			}
		})
	}
}

func TestAdvanceTickCollisionOnFoodCell(t *testing.T) {
	t.Parallel()

	engine := mustEngine(t, 3, 3, 606)
	engine.snake = []Point{{X: 1, Y: 1}}
	engine.occupied = map[Point]struct{}{
		{X: 1, Y: 1}: {},
		{X: 2, Y: 1}: {}, // other snake segment occupying the food cell
	}
	engine.direction = DirectionRight
	engine.food = Point{X: 2, Y: 1}

	before := engine.Snapshot()

	engine.AdvanceTick()
	snap := engine.Snapshot()

	if !snap.GameOver {
		t.Fatalf("expected collision with other snake to end the game")
	}
	if snap.Tick != before.Tick {
		t.Fatalf("tick should remain unchanged after immediate collision, expected %d got %d", before.Tick, snap.Tick)
	}
	if snap.Score != before.Score {
		t.Fatalf("score should not change on collision, expected %d got %d", before.Score, snap.Score)
	}
	if !reflect.DeepEqual(snap.Snake, before.Snake) {
		t.Fatalf("snake body should remain unchanged after collision on food cell, before %+v after %+v", before.Snake, snap.Snake)
	}
	if snap.Food != before.Food {
		t.Fatalf("food should not respawn when collision happens before eating, expected %+v got %+v", before.Food, snap.Food)
	}
	if _, ok := engine.occupied[Point{X: 2, Y: 1}]; !ok {
		t.Fatalf("expected external occupancy at %+v to persist after collision", Point{X: 2, Y: 1})
	}
}

func TestRandomEmptyCellReturnsErrWhenFull(t *testing.T) {
	t.Parallel()

	occupied := map[Point]struct{}{
		{X: 0, Y: 0}: {},
		{X: 1, Y: 0}: {},
		{X: 0, Y: 1}: {},
		{X: 1, Y: 1}: {},
	}
	engine := &Engine{
		width:    2,
		height:   2,
		snake:    []Point{{0, 0}, {1, 0}, {0, 1}, {1, 1}},
		occupied: occupied,
		rng:      rand.New(rand.NewSource(1)),
	}

	before := make(map[Point]struct{}, len(engine.occupied))
	for point := range engine.occupied {
		before[point] = struct{}{}
	}

	if _, err := engine.randomEmptyCell(); !errors.Is(err, ErrNoFreeCells) {
		t.Fatalf("expected ErrNoFreeCells, got %v", err)
	}
	if !reflect.DeepEqual(engine.occupied, before) {
		t.Fatalf("occupied cells mutated while resolving error, before %+v after %+v", before, engine.occupied)
	}
}

func TestRandomEmptyCellWithExternalOccupancy(t *testing.T) {
	t.Parallel()

	clone := func(src map[Point]struct{}) map[Point]struct{} {
		dst := make(map[Point]struct{}, len(src))
		for point := range src {
			dst[point] = struct{}{}
		}
		return dst
	}

	tests := []struct {
		name      string
		width     int
		height    int
		seed      int64
		snake     []Point
		occupied  map[Point]struct{}
		expect    Point
		expectErr error
	}{
		{
			name:   "board full due to other players",
			width:  3,
			height: 3,
			seed:   13,
			snake:  []Point{{X: 1, Y: 1}},
			occupied: map[Point]struct{}{
				{X: 0, Y: 0}: {},
				{X: 1, Y: 0}: {},
				{X: 2, Y: 0}: {},
				{X: 0, Y: 1}: {},
				{X: 1, Y: 1}: {},
				{X: 2, Y: 1}: {},
				{X: 0, Y: 2}: {},
				{X: 1, Y: 2}: {},
				{X: 2, Y: 2}: {},
			},
			expectErr: ErrNoFreeCells,
		},
		{
			name:   "single free cell beside other snakes",
			width:  3,
			height: 3,
			seed:   7,
			snake:  []Point{{X: 1, Y: 1}},
			occupied: map[Point]struct{}{
				{X: 0, Y: 0}: {},
				{X: 1, Y: 0}: {},
				{X: 2, Y: 0}: {},
				{X: 0, Y: 1}: {},
				{X: 1, Y: 1}: {},
				{X: 2, Y: 1}: {},
				{X: 0, Y: 2}: {},
				{X: 1, Y: 2}: {},
			},
			expect: Point{X: 2, Y: 2},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			engine := &Engine{
				width:    tt.width,
				height:   tt.height,
				snake:    append([]Point(nil), tt.snake...),
				occupied: map[Point]struct{}{},
				rng:      rand.New(rand.NewSource(tt.seed)),
			}
			for point := range tt.occupied {
				engine.occupied[point] = struct{}{}
			}

			before := clone(engine.occupied)

			cell, err := engine.randomEmptyCell()
			if tt.expectErr != nil {
				if !errors.Is(err, tt.expectErr) {
					t.Fatalf("expected error %v, got %v", tt.expectErr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error finding empty cell: %v", err)
				}
				if cell != tt.expect {
					t.Fatalf("expected free cell %+v, got %+v", tt.expect, cell)
				}
				if _, taken := engine.occupied[cell]; taken {
					t.Fatalf("returned cell %+v should not be occupied", cell)
				}
			}

			if !reflect.DeepEqual(engine.occupied, before) {
				t.Fatalf("expected occupancy map to remain unchanged, before %+v after %+v", before, engine.occupied)
			}
		})
	}
}

func TestRandomEmptyCellSequenceDeterministicAcrossRespawns(t *testing.T) {
	t.Parallel()

	const (
		width  = 3
		height = 3
		seed   = int64(9091)
	)

	clone := func(src map[Point]struct{}) map[Point]struct{} {
		dst := make(map[Point]struct{}, len(src))
		for point := range src {
			dst[point] = struct{}{}
		}
		return dst
	}

	initialOccupied := map[Point]struct{}{
		{X: 1, Y: 1}: {}, // snake head
		{X: 0, Y: 2}: {}, // other player segment
	}

	expectedOrder := func() []Point {
		order := make([]Point, 0, width*height-len(initialOccupied))
		occupied := clone(initialOccupied)
		rng := rand.New(rand.NewSource(seed))

		for len(occupied) < width*height {
			free := make([]Point, 0, width*height-len(occupied))
			for y := 0; y < height; y++ {
				for x := 0; x < width; x++ {
					point := Point{X: x, Y: y}
					if _, taken := occupied[point]; !taken {
						free = append(free, point)
					}
				}
			}

			chosen := free[rng.Intn(len(free))]
			order = append(order, chosen)
			occupied[chosen] = struct{}{}
		}

		return order
	}()

	engine := &Engine{
		width:    width,
		height:   height,
		snake:    []Point{{X: 1, Y: 1}},
		occupied: clone(initialOccupied),
		rng:      rand.New(rand.NewSource(seed)),
	}

	var sequence []Point
	for len(engine.occupied) < width*height {
		cell, err := engine.randomEmptyCell()
		if err != nil {
			t.Fatalf("unexpected error before board filled: %v", err)
		}
		sequence = append(sequence, cell)
		engine.occupied[cell] = struct{}{}
	}

	if _, err := engine.randomEmptyCell(); !errors.Is(err, ErrNoFreeCells) {
		t.Fatalf("expected ErrNoFreeCells after occupying board, got %v", err)
	}

	if len(engine.occupied) != width*height {
		t.Fatalf("expected board to be fully occupied, got %d cells", len(engine.occupied))
	}
	if !reflect.DeepEqual(sequence, expectedOrder) {
		t.Fatalf("respawn sequence mismatch:\nexpected %v\ngot      %v", expectedOrder, sequence)
	}
}

func TestAdvanceTickProcessesGrowthAndClearsInput(t *testing.T) {
	t.Parallel()

	engine := mustEngine(t, 3, 3, 57)
	engine.snake = []Point{{X: 1, Y: 1}}
	engine.occupied = map[Point]struct{}{
		{X: 1, Y: 1}: {},
		{X: 0, Y: 0}: {}, // other player segment
	}
	engine.direction = DirectionUp
	engine.food = Point{X: 2, Y: 1}
	engine.rng = rand.New(rand.NewSource(57))

	if ok := engine.QueueDirection(DirectionRight); !ok {
		t.Fatalf("expected to queue turn toward food")
	}

	engine.AdvanceTick()
	snap := engine.Snapshot()

	expectedOccupied := map[Point]struct{}{
		{X: 2, Y: 1}: {},
		{X: 1, Y: 1}: {},
		{X: 0, Y: 0}: {},
	}
	expectedFood := deterministicEmptyCell(engine.width, engine.height, expectedOccupied, 57)

	if snap.GameOver {
		t.Fatalf("did not expect game over after eating")
	}
	if snap.Tick != 1 {
		t.Fatalf("tick mismatch: expected 1, got %d", snap.Tick)
	}
	if snap.Score != 1 {
		t.Fatalf("score mismatch: expected 1, got %d", snap.Score)
	}
	if len(snap.Snake) != 2 {
		t.Fatalf("snake length mismatch: expected 2, got %d", len(snap.Snake))
	}
	if snap.Snake[0] != (Point{X: 2, Y: 1}) {
		t.Fatalf("head mismatch: expected %+v, got %+v", Point{X: 2, Y: 1}, snap.Snake[0])
	}
	if snap.Direction != DirectionRight {
		t.Fatalf("direction mismatch: expected %v, got %v", DirectionRight, snap.Direction)
	}
	if snap.Food != expectedFood {
		t.Fatalf("food respawn mismatch: expected %+v, got %+v", expectedFood, snap.Food)
	}
	if engine.hasInput {
		t.Fatalf("queued input flag should clear after processing movement")
	}
	for point := range expectedOccupied {
		if _, ok := engine.occupied[point]; !ok {
			t.Fatalf("expected occupancy at %+v to persist after growth", point)
		}
	}
}
