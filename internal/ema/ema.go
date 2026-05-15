package ema

import (
	"fmt"
)

// EMACalculator はEMAの状態を保持する構造体です
type EMACalculator struct {
	CurrentAverage float64 // 現在の推定平均値
	Alpha          float64 // 平滑化係数（0.0 ~ 1.0）
	IsInitialized  bool    // 最初のデータかどうか
}

// NewEMACalculator は新しい計算機を初期化します
// alpha: 1に近いほど最新の値を重視（変化に敏感）、0に近いほど過去を重視（滑らか）
func NewEMACalculator(alpha float64) *EMACalculator {
	return &EMACalculator{
		Alpha: alpha,
	}
}

// Update は新しい数値を受け取り、更新された平均値を返します
func (e *EMACalculator) Update(input float64) float64 {
	if !e.IsInitialized {
		// 最初のデータはそのまま平均値としてセットする
		e.CurrentAverage = input
		e.IsInitialized = true
		return e.CurrentAverage
	}

	// EMAの基本式: S_t = α * X_t + (1 - α) * S_{t-1}
	e.CurrentAverage = (e.Alpha * input) + (1-e.Alpha)*e.CurrentAverage
	return e.CurrentAverage
}

func main() {
	// アルファ値を0.3に設定（適宜調整してください）
	ema := NewEMACalculator(0.3)

	// 流れてくる数値のシミュレーション
	inputs := []float64{5, 8, 3, 6, 5, 20, 22, 21} // 途中から平均が跳ね上がる例

	fmt.Printf("Alpha: %.2f\n", ema.Alpha)
	fmt.Println("-------------------------------")
	for _, val := range inputs {
		avg := ema.Update(val)
		fmt.Printf("入力: %4.1f -> 推定平均: %4.2f\n", val, avg)
	}
}
