package game

import "testing"

func TestNextHeadPositionByDirection(t *testing.T) {
	t.Parallel()

	start := Point{X: 2, Y: 2}

	tests := []struct {
		name      string
		direction Direction
		expected  Point
	}{
		{name: "up", direction: DirectionUp, expected: Point{X: start.X, Y: start.Y - 1}},
		{name: "down", direction: DirectionDown, expected: Point{X: start.X, Y: start.Y + 1}},
		{name: "left", direction: DirectionLeft, expected: Point{X: start.X - 1, Y: start.Y}},
		{name: "right", direction: DirectionRight, expected: Point{X: start.X + 1, Y: start.Y}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			engine, err := NewEngine(5, 5, 42)
			if err != nil {
				t.Fatalf("unexpected error creating engine: %v", err)
			}

			engine.snake = []Point{start}
			engine.direction = tt.direction

			if next := engine.nextHeadPosition(); next != tt.expected {
				t.Fatalf("nextHeadPosition with %v = %+v, want %+v", tt.direction, next, tt.expected)
			}
		})
	}
}

func TestParseDirection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    string
		expected Direction
		ok       bool
	}{
		{name: "lowercase up", value: "up", expected: DirectionUp, ok: true},
		{name: "mixed case left with spaces", value: "  LeFt ", expected: DirectionLeft, ok: true},
		{name: "uppercase down", value: "DOWN", expected: DirectionDown, ok: true},
		{name: "right trimmed", value: "right", expected: DirectionRight, ok: true},
		{name: "invalid word", value: "forward", ok: false},
		{name: "empty string", value: "", ok: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			actual, ok := ParseDirection(tt.value)
			if ok != tt.ok {
				t.Fatalf("ParseDirection(%q) success = %v, want %v", tt.value, ok, tt.ok)
			}
			if tt.ok && actual != tt.expected {
				t.Fatalf("ParseDirection(%q) = %v, want %v", tt.value, actual, tt.expected)
			}
		})
	}
}
