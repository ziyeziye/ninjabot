package model

import (
	"fmt"
	"math"
	"strconv"
	"time"
)

type TelegramSettings struct {
	Enabled bool
	Token   string
	Users   []int
}

type Settings struct {
	Pairs    []string
	Telegram TelegramSettings
}

type Balance struct {
	Asset    string
	Free     float64
	Lock     float64
	Leverage float64
}

type AssetInfo struct {
	BaseAsset  string
	QuoteAsset string

	MinPrice    float64
	MaxPrice    float64
	MinQuantity float64
	MaxQuantity float64
	StepSize    float64
	TickSize    float64

	QuotePrecision     int
	BaseAssetPrecision int
}

type Dataframe struct {
	Pair string

	OHLC
	LastUpdate time.Time

	// Custom user metadata
	Metadata map[string]Series[float64]
}

func (df Dataframe) Sample(positions int) Dataframe {
	sample := df
	size := len(sample.Time)
	start := size - positions
	if start <= 0 {
		return df
	}

	sample.Close = sample.Close[start:]
	sample.Open = sample.Open[start:]
	sample.Low = sample.Low[start:]
	sample.High = sample.High[start:]
	sample.Volume = sample.Volume[start:]
	sample.Time = sample.Time[start:]

	return sample
}

// OHLC is a connector for technical analysis usage
type OHLC struct {
	Close         Series[float64]
	Open          Series[float64]
	High          Series[float64]
	Low           Series[float64]
	Volume        Series[float64]
	ChangePercent Series[float64]
	IsBullMarket  []bool
	Time          []time.Time
	IsHeikinAshi  bool
}

// HL2 (最高价+最低价)/2
func (df *OHLC) HL2() []float64 {
	var result []float64

	for i, _ := range df.Close {
		result = append(result, (df.High[i]+df.Low[i])/2)
	}
	return result
}

// HLC3 (最高价+最低价+收盘价)/3
func (df *OHLC) HLC3() []float64 {
	var result []float64

	for i, _ := range df.Close {
		result = append(result, (df.High[i]+df.Low[i]+df.Close[i])/3)
	}
	return result
}

// OHLC4 (开盘价 + 最高价 + 最低价 + 收盘价)/4
func (df *OHLC) OHLC4() []float64 {
	var result []float64

	for i, _ := range df.Close {
		result = append(result, (df.Open[i]+df.High[i]+df.Low[i]+df.Close[i])/4)
	}
	return result
}

func (df *OHLC) Candle(i int) Candle {
	return Candle{
		Time:   df.Time[i],
		Open:   df.Open[i],
		Close:  df.Close[i],
		Low:    df.Low[i],
		High:   df.High[i],
		Volume: df.Volume[i],
	}
}

func (df *OHLC) Last(index ...int) Candle {
	length := len(df.Close)
	if length == 0 {
		return Candle{}
	}

	position := 0
	if len(index) > 0 {
		position = index[0]
	}
	i := length - 1 - position
	return df.Candle(i)
}

// ToHeikinAshi 转换成平均K线
func (df *OHLC) ToHeikinAshi() *OHLC {
	ha := NewHeikinAshi()

	df.ChangePercent = make([]float64, len(df.Close))
	df.IsBullMarket = make([]bool, len(df.Close))
	for i, _ := range df.Time {
		candle := df.Candle(i)
		candle = candle.ToHeikinAshi(ha)
		df.Close[i] = candle.Close
		df.Open[i] = candle.Open
		df.Low[i] = candle.Low
		df.High[i] = candle.High
		df.Volume[i] = candle.Volume
		df.ChangePercent[i] = (df.Close[i] - df.Open[i]) / df.Open[i]
		if df.Close[i] > df.Open[i] {
			df.IsBullMarket[i] = true
		}
	}
	df.IsHeikinAshi = true
	return df
}

type Candle struct {
	Pair      string
	Time      time.Time
	UpdatedAt time.Time
	Open      float64
	Close     float64
	Low       float64
	High      float64
	Volume    float64
	Complete  bool

	// Aditional collums from CSV inputs
	Metadata map[string]float64
}

func (c Candle) Empty() bool {
	return c.Pair == "" && c.Close == 0 && c.Open == 0 && c.Volume == 0
}

type HeikinAshi struct {
	PreviousHACandle Candle
}

func NewHeikinAshi() *HeikinAshi {
	return &HeikinAshi{}
}

func (c Candle) ToSlice(precision int) []string {
	return []string{
		fmt.Sprintf("%d", c.Time.Unix()),
		strconv.FormatFloat(c.Open, 'f', precision, 64),
		strconv.FormatFloat(c.Close, 'f', precision, 64),
		strconv.FormatFloat(c.Low, 'f', precision, 64),
		strconv.FormatFloat(c.High, 'f', precision, 64),
		strconv.FormatFloat(c.Volume, 'f', precision, 64),
	}
}

func (c Candle) ToHeikinAshi(ha *HeikinAshi) Candle {
	haCandle := ha.CalculateHeikinAshi(c)

	return Candle{
		Pair:      c.Pair,
		Open:      haCandle.Open,
		High:      haCandle.High,
		Low:       haCandle.Low,
		Close:     haCandle.Close,
		Volume:    c.Volume,
		Complete:  c.Complete,
		Time:      c.Time,
		UpdatedAt: c.UpdatedAt,
	}
}

func (c Candle) Less(j Item) bool {
	diff := j.(Candle).Time.Sub(c.Time)
	if diff < 0 {
		return false
	}
	if diff > 0 {
		return true
	}

	diff = j.(Candle).UpdatedAt.Sub(c.UpdatedAt)
	if diff < 0 {
		return false
	}
	if diff > 0 {
		return true
	}

	return c.Pair < j.(Candle).Pair
}

type Account struct {
	Balances []Balance
}

func (a Account) Balance(assetTick, quoteTick string) (Balance, Balance) {
	var assetBalance, quoteBalance Balance
	var isSetAsset, isSetQuote bool

	for _, balance := range a.Balances {
		switch balance.Asset {
		case assetTick:
			assetBalance = balance
			isSetAsset = true
		case quoteTick:
			quoteBalance = balance
			isSetQuote = true
		}

		if isSetAsset && isSetQuote {
			break
		}
	}

	return assetBalance, quoteBalance
}

func (a Account) Equity() float64 {
	var total float64

	for _, balance := range a.Balances {
		total += balance.Free
		total += balance.Lock
	}

	return total
}

func (ha *HeikinAshi) CalculateHeikinAshi(c Candle) Candle {
	var hkCandle Candle

	openValue := ha.PreviousHACandle.Open
	closeValue := ha.PreviousHACandle.Close

	// First HA candle is calculated using current candle
	if ha.PreviousHACandle.Empty() {
		openValue = c.Open
		closeValue = c.Close
	}

	hkCandle.Open = (openValue + closeValue) / 2
	hkCandle.Close = (c.Open + c.High + c.Low + c.Close) / 4
	hkCandle.High = math.Max(c.High, math.Max(hkCandle.Open, hkCandle.Close))
	hkCandle.Low = math.Min(c.Low, math.Min(hkCandle.Open, hkCandle.Close))
	ha.PreviousHACandle = hkCandle

	return hkCandle
}
