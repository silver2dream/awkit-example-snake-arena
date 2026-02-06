export interface GridPoint {
  x: number
  y: number
}

export interface GameSnapshot {
  tick: number
  width: number
  height: number
  snakes: Record<string, GridPoint[]>
  food: GridPoint | null
  scores: Record<string, number>
  gameOver: boolean
  players?: string[]
}
