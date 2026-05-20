package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
)

type GameState struct {
	EnemyType   string `json:"enemy_type"` // "ZAKO", "BOSS", "CLEAR", "GAMEOVER"
	EnemyName   string `json:"enemy_name"`
	CurrentHP   int    `json:"current_hp"`
	MaxHP       int    `json:"max_hp"`
	Defeated    int    `json:"defeated"`
	PlayerHP    int    `json:"player_hp"`
	PlayerMaxHP int    `json:"player_max_hp"`
	PlayerMP    int    `json:"player_mp"`
	PlayerMaxMP int    `json:"player_max_mp"`
	ItemsLeft   int    `json:"items_left"`
}

var (
	state = GameState{
		EnemyType:   "ZAKO",
		EnemyName:   "スライム A",
		CurrentHP:   300,
		MaxHP:       300,
		Defeated:    0,
		PlayerHP:    1000,
		PlayerMaxHP: 1000,
		PlayerMP:    150,
		PlayerMaxMP: 150,
		ItemsLeft:   3,
	}
	mu      sync.Mutex
	clients = make(map[chan string]bool)
)

func main() {
	// 安全なノンブロッキング・ブロードキャスト
	broadcast := func() {
		bytes, _ := json.Marshal(state)
		stateJSON := string(bytes)
		for ch := range clients {
			select {
			case ch <- stateJSON:
			default:
				// チャネルが詰まっているクライアントはスキップして全体のデッドロックを防ぐ
			}
		}
	}

	http.HandleFunc("/game-stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		messageChan := make(chan string, 10) // バッファを持たせて安定化
		mu.Lock()
		clients[messageChan] = true
		mu.Unlock()

		defer func() {
			mu.Lock()
			delete(clients, messageChan)
			mu.Unlock()
			close(messageChan)
		}()

		bytes, _ := json.Marshal(state)
		fmt.Fprintf(w, "data: %s\n\n", string(bytes))
		w.(http.Flusher).Flush()

		for msg := range messageChan {
			fmt.Fprintf(w, "data: %s\n\n", msg)
			w.(http.Flusher).Flush()
		}
	})
	http.HandleFunc("/reset", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method != http.MethodPost {
			return
		}
	
		mu.Lock()
		// 全すべてのステータスを初期状態にリセット
		state.EnemyType = "ZAKO"
		state.EnemyName = "スライム A"
		state.CurrentHP = 300
		state.MaxHP = 300
		state.Defeated = 0
		state.PlayerHP = 1000
		state.PlayerMaxHP = 1000
		state.PlayerMP = 150
		state.PlayerMaxMP = 150
		state.ItemsLeft = 3
		
		broadcast() // リセットされた状態をブラウザに即時通知
		mu.Unlock()
	
		w.WriteHeader(http.StatusOK)
	})
	http.HandleFunc("/get-state", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")
		mu.Lock()
		defer mu.Unlock()
		json.NewEncoder(w).Encode(state)
	})

	http.HandleFunc("/battle", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method != http.MethodPost {
			return
		}

		dmgToEnemy, _ := strconv.Atoi(r.URL.Query().Get("to_enemy"))
		dmgToPlayer, _ := strconv.Atoi(r.URL.Query().Get("to_player"))
		healAmount, _ := strconv.Atoi(r.URL.Query().Get("heal_amount"))
		mpCost, _ := strconv.Atoi(r.URL.Query().Get("mp_cost"))
		useItem, _ := strconv.ParseBool(r.URL.Query().Get("use_item"))

		mu.Lock()
		defer mu.Unlock()

		if state.EnemyType == "CLEAR" || state.EnemyType == "GAMEOVER" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// リソース消費
		state.PlayerMP -= mpCost
		if useItem && state.ItemsLeft > 0 {
			state.ItemsLeft--
		}

		// 1. 回復処理
		if healAmount > 0 {
			state.PlayerHP += healAmount
			if state.PlayerHP > state.PlayerMaxHP {
				state.PlayerHP = state.PlayerMaxHP
			}
		}

		// 2. 敵へのダメージ処理
		if dmgToEnemy > 0 {
			state.CurrentHP -= dmgToEnemy
		}

		// 3. 敵からの反撃処理（敵がまだ生きていれば実行）
		if state.CurrentHP > 0 {
			if dmgToPlayer > 0 {
				state.PlayerHP -= dmgToPlayer
				if state.PlayerHP <= 0 {
					state.PlayerHP = 0
					state.EnemyType = "GAMEOVER"
				}
			}
		}

		// 4. 討伐・次ウェーブ遷移ロジック
		if state.CurrentHP <= 0 && state.EnemyType != "GAMEOVER" {
			if state.EnemyType == "ZAKO" {
				state.Defeated++
				if state.Defeated >= 3 {
					state.EnemyType = "BOSS"
					state.EnemyName = "ドラゴン（BOSS）"
					state.MaxHP = 2000
					state.CurrentHP = 2000
				} else {
					state.EnemyName = fmt.Sprintf("スライム %c", 'A'+state.Defeated)
					state.MaxHP = 300
					state.CurrentHP = 300
				}
			} else if state.EnemyType == "BOSS" {
				state.EnemyType = "CLEAR"
				state.EnemyName = "なし"
				state.CurrentHP = 0
			}
		}

		broadcast()
		w.WriteHeader(http.StatusOK)
	})

	fmt.Println("Server started on :8080")
	http.ListenAndServe(":8080", nil)
}