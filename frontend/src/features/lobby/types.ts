export interface RoomSummary {
  id: string
  name?: string
  players?: number
}

export interface CreateRoomRequest {
  name: string
  playerId: string
}

export interface JoinRoomRequest {
  roomId: string
  playerId: string
}

export interface LobbyApi {
  listRooms: () => Promise<RoomSummary[]>
  createRoom: (request: CreateRoomRequest) => Promise<{ roomId: string }>
  joinRoom: (request: JoinRoomRequest) => Promise<{ roomId: string }>
  connectToRoom: (roomId: string, playerId: string) => WebSocket | null
}
