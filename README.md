# awkit-example-snake-arena

多人即時貪吃蛇對戰遊戲。後端為 Go HTTP/WebSocket 伺服器，前端為 React + TypeScript。

## 需求

- Go 1.25+
- Node.js (LTS) + npm

## 快速啟動

### 1. 啟動後端

```bash
cd backend
PORT=9091 go run main.go
# 伺服器啟動於 :9091
```

### 2. 啟動前端

開啟新的終端機視窗：

```bash
cd frontend
npm install   # 首次執行需安裝依賴
npm run dev
# 開發伺服器啟動於 http://localhost:5173
```

### 3. 開始遊玩

1. 開啟瀏覽器前往 `http://localhost:5173`
2. 輸入玩家名稱
3. 建立新房間或加入現有房間
4. 使用方向鍵控制蛇的移動

## 遊戲規則

- 使用 **方向鍵** (↑ ↓ ← →) 控制蛇移動
- 吃到食物可增加分數並讓蛇變長
- 撞牆、撞到自己或其他玩家的蛇身即死亡
- 支援多人同時對戰

## 專案結構

```
backend/    # Go 遊戲伺服器（HTTP + WebSocket）
frontend/   # React 前端（Vite）
```

## 開發指令

### 後端

```bash
cd backend
go test ./...          # 執行測試
go build -o snake-backend main.go  # 建置
```

### 前端

```bash
cd frontend
npm run build    # 建置
npm run test     # 執行測試
```

## API 端點

| 方法 | 路徑 | 說明 |
|------|------|------|
| GET | `/health` | 健康檢查 |
| GET | `/api/rooms` | 列出所有房間 |
| POST | `/api/rooms` | 建立新房間 |
| POST | `/api/rooms/{roomId}/join` | 加入房間 |
| GET | `/ws/room/{roomId}` | WebSocket 連線（即時遊戲） |
