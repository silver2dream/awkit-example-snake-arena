package game

// CloneState creates a deep copy of the provided game state.
func CloneState(state *GameState) *GameState {
	if state == nil {
		return nil
	}

	snakes := make(map[string]*Snake, len(state.Snakes))
	for id, snake := range state.Snakes {
		if snake == nil {
			continue
		}
		bodyCopy := append([]Position(nil), snake.Body...)
		snakes[id] = &Snake{
			ID:        snake.ID,
			Body:      bodyCopy,
			Direction: snake.Direction,
			Alive:     snake.Alive,
		}
	}

	return &GameState{
		Grid:   state.Grid,
		Snakes: snakes,
		Food:   append([]Position(nil), state.Food...),
	}
}
