<?php
// game.php
header('Content-Type: application/json');

$action = isset($_POST['action']) ? $_POST['action'] : 'attack';

// Goから最新状態を取得
$goStateJson = @file_get_contents("http://localhost:8080/get-state");
if (!$goStateJson) {
    echo json_encode(["status" => "error", "message" => "ゲームサーバーに接続できません。"]);
    exit;
}
$currentState = json_decode($goStateJson, true);

$dmgToEnemy = 0;
$dmgToPlayer = 0; // デフォルトは0（回復時は反撃なし）
$healAmount = 0;
$mpCost = 0;
$useItem = "false";
$logMessage = "";

switch ($action) {
    Case 'heal_small':
        $mpCost = 15;
        if ($currentState['player_mp'] < $mpCost) {
            echo json_encode(["status" => "error", "message" => "MP不足！小回復が唱えられません。"]);
            exit;
        }
        $healAmount = rand(150, 250);
        $logMessage = "🪄 小回復（ヒール）を唱えた！\nあなたのHPが {$healAmount} 回復。";
        break;

    Case 'heal_large':
        $mpCost = 40;
        if ($currentState['player_mp'] < $mpCost) {
            echo json_encode(["status" => "error", "message" => "MP不足！大回復が唱えられません。"]);
            exit;
        }
        $healAmount = rand(400, 600);
        $logMessage = "✨ 大回復（マナヒール）を唱えた！\nあなたのHPが {$healAmount} 回復。";
        break;

    Case 'use_herb':
        if ($currentState['items_left'] <= 0) {
            echo json_encode(["status" => "error", "message" => "薬草がありません！"]);
            exit;
        }
        $useItem = "true";
        $healAmount = 300;
        $logMessage = "🌿 薬草を使った！\nあなたのHPが {$healAmount} 回復。";
        break;

    Case 'attack':
    default:
        $dmgToEnemy = rand(40, 80);
        $isCritical = rand(1, 5) === 1;
        if ($isCritical) { $dmgToEnemy *= 2; }
        
        $logMessage = "⚔️ あなたの攻撃！\n敵に {$dmgToEnemy} のダメージを与えた！" . ($isCritical ? "（会心の一撃！）" : "");

        // 敵が生き残る場合のみ、反撃ダメージを計算
        if (($currentState['current_hp'] - $dmgToEnemy) > 0) {
            $dmgToPlayer = ($currentState['enemy_type'] === 'BOSS') ? rand(80, 150) : rand(30, 60);
            $logMessage .= "\n💥 敵の反撃！\nあなたは {$dmgToPlayer} のダメージを受けた！";
        } else {
            $logMessage .= "\n🎉 敵を撃破した！";
        }
        break;
}

// Go言語への同期
$goUrl = "http://localhost:8080/battle?" . http_build_query([
    "to_enemy" => $dmgToEnemy,
    "to_player" => $dmgToPlayer,
    "heal_amount" => $healAmount,
    "mp_cost" => $mpCost,
    "use_item" => $useItem
]);

$ch = curl_init($goUrl);
curl_setopt($ch, CURLOPT_POST, true);
curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
curl_exec($ch);
curl_close($ch);

echo json_encode([
    "status" => "success",
    "log" => $logMessage
]);